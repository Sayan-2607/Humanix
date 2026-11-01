package citation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExtract(t *testing.T) {
	txt := "Vaccines reduce mortality (Smith et al., 2020). Rates fell by 40% in two years. " +
		"Jones (2019) disagreed [3]. See https://example.org/report. DOI 10.1016/S0140-6736(20)30183-5 is cited. The sky is blue."
	ex := Extract(txt)
	types := map[string]int{}
	for _, c := range ex.Citations {
		types[c.Type]++
	}
	if types["author_year"] != 2 || types["numeric"] != 1 || types["url"] != 1 || types["doi"] != 1 {
		t.Fatalf("types %v: %+v", types, ex.Citations)
	}
	for _, c := range ex.Citations {
		if c.Type == "doi" && c.Value != "10.1016/S0140-6736(20)30183-5" {
			t.Fatalf("doi %q", c.Value)
		}
	}
	if len(ex.UncitedClaims) != 1 || !strings.Contains(ex.UncitedClaims[0].Sentence, "40%") {
		t.Fatalf("claims %+v", ex.UncitedClaims)
	}
}

var src = Source{
	Type: "article", Title: "A study of plain language", Container: "Journal of Readability",
	Year: "2021", Volume: "12", Issue: "3", Pages: "45–67", DOI: "10.1234/jr.2021.003",
	Authors: []Author{{"Ana María", "García"}, {"John", "Smith"}},
}

func TestFormats(t *testing.T) {
	want := map[string]string{
		"apa":     "García, A. M., & Smith, J. (2021). A study of plain language. *Journal of Readability*, *12*(3), 45–67. https://doi.org/10.1234/jr.2021.003",
		"mla":     "García, Ana María, and John Smith. “A study of plain language.” *Journal of Readability*, vol. 12, no. 3, 2021, pp. 45–67. https://doi.org/10.1234/jr.2021.003.",
		"ieee":    "A. M. García and J. Smith, “A study of plain language,” *Journal of Readability*, vol. 12, no. 3, pp. 45–67, 2021, doi: 10.1234/jr.2021.003.",
		"chicago": "García, Ana María, and John Smith. “A study of plain language.” *Journal of Readability* 12, no. 3 (2021): 45–67. https://doi.org/10.1234/jr.2021.003.",
	}
	for style, w := range want {
		f, err := Format(src, style)
		if err != nil {
			t.Fatal(err)
		}
		if f.Text != w {
			t.Errorf("%s:\n got  %s\n want %s", style, f.Text, w)
		}
	}
	if _, err := Format(src, "harvard"); err == nil {
		t.Fatal("expected error")
	}
	b, _ := Format(Source{Type: "book", Title: "Plain <Words>", Publisher: "Penguin", Year: "1954", Authors: []Author{{"Ernest", "Gowers"}}}, "apa")
	if b.Text != "Gowers, E. (1954). *Plain <Words>*. Penguin." || !strings.Contains(b.HTML, "&lt;Words&gt;") {
		t.Fatalf("book: %q %q", b.Text, b.HTML)
	}
}

func TestCrossrefLookup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/works/10.1234/abc") {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"message":{"type":"journal-article","title":["Test  Title"],"container-title":["J Test"],
		"volume":"5","issue":"2","page":"1-9","DOI":"10.1234/abc","author":[{"given":"Ada","family":"Lovelace"}],
		"issued":{"date-parts":[[1843]]}}}`))
	}))
	defer srv.Close()
	c := Crossref{BaseURL: srv.URL}
	s, err := c.Lookup(context.Background(), "https://doi.org/10.1234/abc")
	if err != nil {
		t.Fatal(err)
	}
	if s.Title != "Test Title" || s.Year != "1843" || s.Pages != "1–9" || !s.Verified || s.Authors[0].Family != "Lovelace" {
		t.Fatalf("%+v", s)
	}
	if _, err := c.Lookup(context.Background(), "10.9999/missing"); err != ErrNotFound {
		t.Fatalf("want not found, got %v", err)
	}
	if _, err := c.Lookup(context.Background(), "not-a-doi"); err == nil {
		t.Fatal("expected validation error")
	}
}
