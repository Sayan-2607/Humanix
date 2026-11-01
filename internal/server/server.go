// Package server exposes the HUMANIX engines over a JSON HTTP API and serves
// the embedded web interface. It is stateless: no submitted text is stored
// or logged, which keeps it privacy-preserving and lets it run unchanged on
// serverless platforms (Vercel) and containers alike.
package server

import (
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/humanix-ai/humanix/internal/citation"
)

const Version = "1.0.0"

type Config struct {
	AllowedOrigins []string // "*" allows any origin (API holds no credentials)
	MaxTextChars   int
	MaxUploadBytes int64
	RateLimitRPM   int
	CrossrefMailto string
	CrossrefOn     bool
	CrossrefBase   string
}

// ConfigFromEnv reads configuration with safe defaults.
func ConfigFromEnv() Config {
	c := Config{
		AllowedOrigins: splitCSV(getenv("ALLOWED_ORIGINS", "*")),
		MaxTextChars:   atoi(getenv("MAX_TEXT_CHARS", "100000"), 100000),
		MaxUploadBytes: int64(atoi(getenv("MAX_UPLOAD_BYTES", "4194304"), 4<<20)),
		RateLimitRPM:   atoi(getenv("RATE_LIMIT_RPM", "120"), 120),
		CrossrefMailto: os.Getenv("CROSSREF_MAILTO"),
		CrossrefOn:     getenv("CROSSREF_ENABLED", "true") != "false",
		CrossrefBase:   os.Getenv("CROSSREF_BASE_URL"),
	}
	return c
}

type Server struct {
	cfg      Config
	log      *slog.Logger
	limiter  *rateLimiter
	crossref citation.Crossref
	static   fs.FS
	started  time.Time
}

// New builds the HTTP handler. static may be nil (API only).
func New(cfg Config, static fs.FS, log *slog.Logger) http.Handler {
	if log == nil {
		log = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}
	s := &Server{
		cfg: cfg, log: log, static: static, started: time.Now(),
		limiter:  newRateLimiter(cfg.RateLimitRPM),
		crossref: citation.Crossref{BaseURL: cfg.CrossrefBase, Mailto: cfg.CrossrefMailto, Client: &http.Client{Timeout: 8 * time.Second}},
	}
	mux := http.NewServeMux()
	api := map[string]map[string]http.HandlerFunc{
		"/api/v1/health":            {"GET": s.health},
		"/api/v1/meta":              {"GET": s.meta},
		"/api/v1/analyze":           {"POST": s.analyze},
		"/api/v1/simplify":          {"POST": s.simplify},
		"/api/v1/compare":           {"POST": s.compare},
		"/api/v1/originality/check": {"POST": s.originality},
		"/api/v1/citations/extract": {"POST": s.citationsExtract},
		"/api/v1/citations/format":  {"POST": s.citationsFormat},
		"/api/v1/citations/lookup":  {"GET": s.citationsLookup},
		"/api/v1/documents/extract": {"POST": s.documentsExtract},
	}
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimRight(r.URL.Path, "/")
		methods, ok := api[path]
		if !ok {
			writeErr(w, http.StatusNotFound, "not_found", "Unknown endpoint. See GET /api/v1/meta.")
			return
		}
		h, ok := methods[r.Method]
		if !ok {
			allow := make([]string, 0, len(methods))
			for m := range methods {
				allow = append(allow, m)
			}
			w.Header().Set("Allow", strings.Join(allow, ", ")+", OPTIONS")
			writeErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use "+strings.Join(allow, ", ")+".")
			return
		}
		if r.Method != http.MethodGet && !s.limiter.allow(clientIP(r)) {
			w.Header().Set("Retry-After", "30")
			writeErr(w, http.StatusTooManyRequests, "rate_limited", "Too many requests; please wait a moment.")
			return
		}
		h(w, r)
	})
	mux.HandleFunc("/health", s.health)
	if static != nil {
		mux.Handle("/", staticHandler(static))
	}
	return chain(mux, s.recoverer, s.logger, s.securityHeaders, s.cors)
}

func getenv(k, d string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return d
}

func atoi(s string, d int) int {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return d
	}
	return n
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
