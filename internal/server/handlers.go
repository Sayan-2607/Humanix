package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/humanix-ai/humanix/internal/citation"
	"github.com/humanix-ai/humanix/internal/docx"
	"github.com/humanix-ai/humanix/internal/originality"
	"github.com/humanix-ai/humanix/internal/plain"
	"github.com/humanix-ai/humanix/internal/preserve"
	"github.com/humanix-ai/humanix/internal/quality"
	"github.com/humanix-ai/humanix/internal/textutil"
)

type apiError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	_ = enc.Encode(v)
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	var e apiError
	e.Error.Code, e.Error.Message = code, msg
	writeJSON(w, status, e)
}

// decode reads a JSON body with a size cap and strict field checking.
func (s *Server) decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	if ct := r.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(ct, "application/json") {
		writeErr(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Send JSON with Content-Type: application/json.")
		return false
	}
	// ~4 bytes per char worst case for UTF-8, plus JSON overhead and references
	limit := int64(s.cfg.MaxTextChars)*4*6 + 64<<10
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			writeErr(w, http.StatusRequestEntityTooLarge, "too_large", "Request body is too large.")
		} else {
			writeErr(w, http.StatusBadRequest, "bad_json", "Invalid JSON: "+err.Error())
		}
		return false
	}
	return true
}

func (s *Server) checkText(w http.ResponseWriter, field, t string) bool {
	if strings.TrimSpace(t) == "" {
		writeErr(w, http.StatusUnprocessableEntity, "empty_text", "Field \""+field+"\" must not be empty.")
		return false
	}
	if !utf8.ValidString(t) {
		writeErr(w, http.StatusUnprocessableEntity, "invalid_utf8", "Field \""+field+"\" must be valid UTF-8.")
		return false
	}
	if n := utf8.RuneCountInString(t); n > s.cfg.MaxTextChars {
		writeErr(w, http.StatusRequestEntityTooLarge, "text_too_long",
			"Field \""+field+"\" exceeds the limit of "+itoa(s.cfg.MaxTextChars)+" characters.")
		return false
	}
	return true
}

// ---------- system ----------

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok", "version": Version, "uptime_seconds": int(time.Since(s.started).Seconds()),
	})
}

func (s *Server) meta(w http.ResponseWriter, r *http.Request) {
	aud := map[string]any{}
	for k, a := range quality.Audiences {
		aud[k] = map[string]any{"label": a.Label, "target_grade": a.TargetGrade, "max_avg_sentence_words": a.MaxAvgWords}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name": "HUMANIX", "version": Version,
		"audiences": aud, "citation_styles": citation.Styles,
		"limits": map[string]any{"max_text_chars": s.cfg.MaxTextChars, "max_upload_bytes": s.cfg.MaxUploadBytes,
			"max_references": maxRefs, "rate_limit_rpm": s.cfg.RateLimitRPM},
		"crossref_enabled": s.cfg.CrossrefOn,
		"privacy":          "Submitted text is processed in memory and never stored or logged by the server.",
		"endpoints": []string{"GET /api/v1/health", "GET /api/v1/meta", "POST /api/v1/analyze", "POST /api/v1/simplify",
			"POST /api/v1/compare", "POST /api/v1/originality/check", "POST /api/v1/citations/extract",
			"POST /api/v1/citations/format", "GET /api/v1/citations/lookup?doi=", "POST /api/v1/documents/extract"},
	})
}

// ---------- analysis ----------

type analyzeReq struct {
	Text     string `json:"text"`
	Audience string `json:"audience"`
}

func (s *Server) analyze(w http.ResponseWriter, r *http.Request) {
	var req analyzeReq
	if !s.decode(w, r, &req) || !s.checkText(w, "text", req.Text) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"analysis": quality.Analyze(req.Text, req.Audience), "normalized_text": textutil.Normalize(req.Text)})
}

type simplifyReq struct {
	Text          string   `json:"text"`
	Level         string   `json:"level"`
	Audience      string   `json:"audience"`
	PreserveTerms []string `json:"preserve_terms"`
}

type snapshot struct {
	ConsensusGrade    float64 `json:"consensus_grade"`
	FleschReadingEase float64 `json:"flesch_reading_ease"`
	AvgWordsPerSent   float64 `json:"avg_words_per_sentence"`
	Words             int     `json:"words"`
	ComplexWords      int     `json:"complex_words"`
	MeetsTarget       bool    `json:"meets_target"`
}

func snap(q quality.Report) snapshot {
	return snapshot{q.Readability.ConsensusGrade, q.Readability.FleschReadingEase, round2(q.Counts.AvgWordsPerSent),
		q.Counts.Words, q.Counts.ComplexWords, q.Audience.MeetsTarget}
}

func (s *Server) simplify(w http.ResponseWriter, r *http.Request) {
	var req simplifyReq
	if !s.decode(w, r, &req) || !s.checkText(w, "text", req.Text) {
		return
	}
	if req.Level != "" && req.Level != "conservative" && req.Level != "standard" {
		writeErr(w, http.StatusUnprocessableEntity, "bad_level", "level must be \"conservative\" or \"standard\".")
		return
	}
	if len(req.PreserveTerms) > 200 {
		req.PreserveTerms = req.PreserveTerms[:200]
	}
	text := textutil.Normalize(req.Text)
	res := plain.Simplify(text, plain.Options{Level: req.Level, PreserveTerms: req.PreserveTerms})
	before := quality.Analyze(text, req.Audience)
	after := quality.Analyze(res.Revised, req.Audience)
	writeJSON(w, http.StatusOK, map[string]any{
		"original_text": text,
		"revised_text":  res.Revised,
		"applied":       res.Applied,
		"suggestions":   res.Suggestions,
		"before":        snap(before),
		"after":         snap(after),
		"remaining_issues": map[string]any{
			"long_sentences":           after.Issues.LongSentences,
			"passive_voice_candidates": after.Issues.PassiveCandidates,
		},
		"preservation": preserve.Compare(text, res.Revised),
		"method":       "Rule-based substitutions from a curated plain-language phrase list. Sentence restructuring is left to the writer; long sentences and passive voice are listed for manual revision.",
	})
}

type compareReq struct {
	Original string `json:"original"`
	Revised  string `json:"revised"`
}

func (s *Server) compare(w http.ResponseWriter, r *http.Request) {
	var req compareReq
	if !s.decode(w, r, &req) || !s.checkText(w, "original", req.Original) || !s.checkText(w, "revised", req.Revised) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"preservation": preserve.Compare(req.Original, req.Revised)})
}

// ---------- originality ----------

const maxRefs = 25

type origReq struct {
	Text        string                  `json:"text"`
	References  []originality.Reference `json:"references"`
	ShingleSize int                     `json:"shingle_size"`
}

func (s *Server) originality(w http.ResponseWriter, r *http.Request) {
	var req origReq
	if !s.decode(w, r, &req) || !s.checkText(w, "text", req.Text) {
		return
	}
	if len(req.References) == 0 {
		writeErr(w, http.StatusUnprocessableEntity, "no_references", "Provide at least one reference text to compare against.")
		return
	}
	if len(req.References) > maxRefs {
		writeErr(w, http.StatusUnprocessableEntity, "too_many_references", "At most "+itoa(maxRefs)+" references per request.")
		return
	}
	total := 0
	for i, ref := range req.References {
		if !s.checkText(w, "references["+itoa(i)+"].text", ref.Text) {
			return
		}
		total += utf8.RuneCountInString(ref.Text)
	}
	if total > s.cfg.MaxTextChars*5 {
		writeErr(w, http.StatusRequestEntityTooLarge, "references_too_long", "Combined references exceed "+itoa(s.cfg.MaxTextChars*5)+" characters.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"originality": originality.Check(req.Text, req.References, req.ShingleSize)})
}

// ---------- citations ----------

func (s *Server) citationsExtract(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text string `json:"text"`
	}
	if !s.decode(w, r, &req) || !s.checkText(w, "text", req.Text) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"citations": citation.Extract(req.Text)})
}

type formatReq struct {
	Source  citation.Source   `json:"source"`
	Sources []citation.Source `json:"sources"`
	Style   string            `json:"style"`
}

func (s *Server) citationsFormat(w http.ResponseWriter, r *http.Request) {
	var req formatReq
	if !s.decode(w, r, &req) {
		return
	}
	srcs := req.Sources
	if len(srcs) == 0 {
		srcs = []citation.Source{req.Source}
	}
	if len(srcs) > 200 {
		writeErr(w, http.StatusUnprocessableEntity, "too_many_sources", "At most 200 sources per request.")
		return
	}
	styles := citation.Styles
	if req.Style != "" {
		styles = []string{strings.ToLower(req.Style)}
	}
	out := make([]map[string]citation.Formatted, 0, len(srcs))
	for _, src := range srcs {
		// a client cannot mark metadata as registry-verified
		src.Verified = false
		m := map[string]citation.Formatted{}
		for _, st := range styles {
			f, err := citation.Format(src, st)
			if err != nil {
				writeErr(w, http.StatusUnprocessableEntity, "bad_style", err.Error())
				return
			}
			m[st] = f
		}
		out = append(out, m)
	}
	writeJSON(w, http.StatusOK, map[string]any{"formatted": out})
}

func (s *Server) citationsLookup(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.CrossrefOn {
		writeErr(w, http.StatusServiceUnavailable, "lookup_disabled", "DOI lookup is disabled on this server.")
		return
	}
	if !s.limiter.allow(clientIP(r)) {
		writeErr(w, http.StatusTooManyRequests, "rate_limited", "Too many requests; please wait a moment.")
		return
	}
	doi := r.URL.Query().Get("doi")
	if _, err := citation.NormalizeDOI(doi); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "bad_doi", err.Error())
		return
	}
	src, err := s.crossref.Lookup(r.Context(), doi)
	if errors.Is(err, citation.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "doi_not_found", "Crossref has no record for this DOI. It may be registered with another agency (e.g. DataCite) or mistyped.")
		return
	}
	if err != nil {
		s.log.Warn("crossref", "err", err.Error())
		writeErr(w, http.StatusBadGateway, "lookup_failed", "Could not reach Crossref. Try again later.")
		return
	}
	formatted := map[string]citation.Formatted{}
	for _, st := range citation.Styles {
		formatted[st], _ = citation.Format(src, st)
	}
	writeJSON(w, http.StatusOK, map[string]any{"source": src, "formatted": formatted})
}

// ---------- documents ----------

func (s *Server) documentsExtract(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxUploadBytes+(64<<10))
	if err := r.ParseMultipartForm(s.cfg.MaxUploadBytes); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			writeErr(w, http.StatusRequestEntityTooLarge, "too_large", "File exceeds "+itoa(int(s.cfg.MaxUploadBytes>>20))+" MB.")
		} else {
			writeErr(w, http.StatusBadRequest, "bad_upload", "Send multipart/form-data with a \"file\" field.")
		}
		return
	}
	defer r.MultipartForm.RemoveAll()
	f, hdr, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "no_file", "Missing \"file\" field.")
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, s.cfg.MaxUploadBytes+1))
	if err != nil || int64(len(data)) > s.cfg.MaxUploadBytes {
		writeErr(w, http.StatusRequestEntityTooLarge, "too_large", "File too large.")
		return
	}
	var text string
	switch ext := strings.ToLower(filepath.Ext(hdr.Filename)); ext {
	case ".docx":
		text, err = docx.ExtractText(data)
		if err != nil {
			writeErr(w, http.StatusUnprocessableEntity, "bad_docx", err.Error())
			return
		}
	case ".txt", ".md", ".markdown", ".csv":
		if !utf8.Valid(data) {
			writeErr(w, http.StatusUnprocessableEntity, "invalid_utf8", "Text files must be UTF-8 encoded.")
			return
		}
		text = string(data)
	case ".pdf":
		writeErr(w, http.StatusUnprocessableEntity, "pdf_client_side", "PDFs are read in your browser; use the upload button in the web app.")
		return
	default:
		writeErr(w, http.StatusUnsupportedMediaType, "unsupported_type", "Supported: .docx, .txt, .md (PDF in the browser).")
		return
	}
	text = textutil.Normalize(text)
	truncated := false
	if rs := []rune(text); len(rs) > s.cfg.MaxTextChars {
		text, truncated = string(rs[:s.cfg.MaxTextChars]), true
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"filename": filepath.Base(hdr.Filename), "text": text,
		"characters": utf8.RuneCountInString(text), "truncated": truncated,
	})
}

func itoa(i int) string { return strconv.Itoa(i) }

func round2(x float64) float64 { return float64(int(x*100+0.5)) / 100 }
