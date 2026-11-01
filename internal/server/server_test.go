package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func newTestServer(t *testing.T, mod func(*Config)) *httptest.Server {
	cfg := Config{AllowedOrigins: []string{"*"}, MaxTextChars: 5000, MaxUploadBytes: 1 << 20, RateLimitRPM: 1000, CrossrefOn: true}
	if mod != nil {
		mod(&cfg)
	}
	static := fstest.MapFS{"index.html": {Data: []byte("<h1>HUMANIX</h1>")}}
	ts := httptest.NewServer(New(cfg, static, slog.New(slog.NewTextHandler(io.Discard, nil))))
	t.Cleanup(ts.Close)
	return ts
}

func post(t *testing.T, url string, body any) (int, map[string]any) {
	b, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var m map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&m)
	return resp.StatusCode, m
}

func TestHealthAndStatic(t *testing.T) {
	ts := newTestServer(t, nil)
	for _, p := range []string{"/health", "/api/v1/health", "/api/v1/meta"} {
		resp, err := http.Get(ts.URL + p)
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("%s: %v %v", p, err, resp.StatusCode)
		}
		if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
			t.Fatal("missing security header")
		}
	}
	resp, _ := http.Get(ts.URL + "/")
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "HUMANIX") {
		t.Fatal("static not served")
	}
}

func TestAnalyze(t *testing.T) {
	ts := newTestServer(t, nil)
	code, m := post(t, ts.URL+"/api/v1/analyze", map[string]any{"text": "The cat sat. The dog ran.", "audience": "health"})
	if code != 200 {
		t.Fatalf("%d %v", code, m)
	}
	a := m["analysis"].(map[string]any)
	if a["counts"].(map[string]any)["words"].(float64) != 6 {
		t.Fatalf("%v", a["counts"])
	}
}

func TestValidation(t *testing.T) {
	ts := newTestServer(t, nil)
	if code, _ := post(t, ts.URL+"/api/v1/analyze", map[string]any{"text": "  "}); code != 422 {
		t.Fatalf("empty: %d", code)
	}
	if code, _ := post(t, ts.URL+"/api/v1/analyze", map[string]any{"text": strings.Repeat("a ", 3000)}); code != 413 {
		t.Fatalf("too long: %d", code)
	}
	if code, _ := post(t, ts.URL+"/api/v1/analyze", map[string]any{"text": "x", "unknown": 1}); code != 400 {
		t.Fatalf("unknown field: %d", code)
	}
	resp, _ := http.Get(ts.URL + "/api/v1/analyze")
	if resp.StatusCode != 405 {
		t.Fatalf("method: %d", resp.StatusCode)
	}
	resp, _ = http.Get(ts.URL + "/api/v1/nope")
	if resp.StatusCode != 404 {
		t.Fatalf("404: %d", resp.StatusCode)
	}
	if code, _ := post(t, ts.URL+"/api/v1/simplify", map[string]any{"text": "x", "level": "aggressive"}); code != 422 {
		t.Fatalf("level: %d", code)
	}
}

func TestSimplify(t *testing.T) {
	ts := newTestServer(t, nil)
	code, m := post(t, ts.URL+"/api/v1/simplify", map[string]any{
		"text": "In order to apply, individuals must submit the form prior to 30 June.", "audience": "general"})
	if code != 200 {
		t.Fatalf("%d %v", code, m)
	}
	if m["revised_text"] != "To apply, people must submit the form before 30 June." {
		t.Fatalf("%v", m["revised_text"])
	}
	if m["preservation"].(map[string]any)["risk"] == "high" {
		t.Fatalf("risk: %v", m["preservation"])
	}
}

func TestOriginalityAndCitations(t *testing.T) {
	ts := newTestServer(t, nil)
	code, m := post(t, ts.URL+"/api/v1/originality/check", map[string]any{
		"text":       "We know that access to clean water reduces child mortality in rural areas significantly.",
		"references": []map[string]string{{"title": "Report", "text": "Evidence shows access to clean water reduces child mortality in rural areas."}},
	})
	if code != 200 || len(m["originality"].(map[string]any)["matches"].([]any)) != 1 {
		t.Fatalf("%d %v", code, m)
	}
	if code, _ := post(t, ts.URL+"/api/v1/originality/check", map[string]any{"text": "abc"}); code != 422 {
		t.Fatal("refs required")
	}
	code, m = post(t, ts.URL+"/api/v1/citations/format", map[string]any{"style": "apa",
		"source": map[string]any{"type": "book", "title": "Plain Words", "year": "1954", "publisher": "HMSO",
			"authors": []map[string]string{{"given": "Ernest", "family": "Gowers"}}, "verified": true}})
	if code != 200 {
		t.Fatalf("%d %v", code, m)
	}
	f := m["formatted"].([]any)[0].(map[string]any)["apa"].(map[string]any)
	if f["text"] != "Gowers, E. (1954). *Plain Words*. HMSO." {
		t.Fatalf("%v", f["text"])
	}
	// client-supplied "verified" must be ignored
	if !strings.Contains(strings.Join(toStrings(f["warnings"]), " "), "not verified") {
		t.Fatal("verified flag should be server-controlled")
	}
}

func toStrings(v any) []string {
	var out []string
	for _, x := range v.([]any) {
		out = append(out, x.(string))
	}
	return out
}

func TestDOILookupViaMock(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"message":{"type":"journal-article","title":["T"],"container-title":["J"],"DOI":"10.1000/x","issued":{"date-parts":[[2020]]},"author":[{"given":"A","family":"B"}]}}`))
	}))
	defer mock.Close()
	ts := newTestServer(t, func(c *Config) { c.CrossrefBase = mock.URL })
	resp, err := http.Get(ts.URL + "/api/v1/citations/lookup?doi=10.1000/x")
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("%v %v", err, resp.StatusCode)
	}
	resp, _ = http.Get(ts.URL + "/api/v1/citations/lookup?doi=garbage")
	if resp.StatusCode != 422 {
		t.Fatalf("bad doi: %d", resp.StatusCode)
	}
}

func TestUploadDocx(t *testing.T) {
	ts := newTestServer(t, nil)
	var zbuf bytes.Buffer
	zw := zip.NewWriter(&zbuf)
	fw, _ := zw.Create("word/document.xml")
	fw.Write([]byte(`<w:document xmlns:w="w"><w:body><w:p><w:r><w:t>Hello docx</w:t></w:r></w:p></w:body></w:document>`))
	zw.Close()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, _ := mw.CreateFormFile("file", "a.docx")
	part.Write(zbuf.Bytes())
	mw.Close()
	resp, err := http.Post(ts.URL+"/api/v1/documents/extract", mw.FormDataContentType(), &body)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("%v %d", err, resp.StatusCode)
	}
	var m map[string]any
	json.NewDecoder(resp.Body).Decode(&m)
	if m["text"] != "Hello docx" {
		t.Fatalf("%v", m)
	}
}

func TestRateLimitAndCORS(t *testing.T) {
	ts := newTestServer(t, func(c *Config) { c.RateLimitRPM = 2; c.AllowedOrigins = []string{"https://ngo.example"} })
	var last int
	for i := 0; i < 4; i++ {
		last, _ = post(t, ts.URL+"/api/v1/analyze", map[string]any{"text": "Hi there."})
	}
	if last != 429 {
		t.Fatalf("expected 429, got %d", last)
	}
	req, _ := http.NewRequest("OPTIONS", ts.URL+"/api/v1/analyze", nil)
	req.Header.Set("Origin", "https://ngo.example")
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != 204 || resp.Header.Get("Access-Control-Allow-Origin") != "https://ngo.example" {
		t.Fatalf("cors: %d %v", resp.StatusCode, resp.Header)
	}
	req.Header.Set("Origin", "https://evil.example")
	resp, _ = http.DefaultClient.Do(req)
	if resp.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("disallowed origin got CORS header")
	}
}
