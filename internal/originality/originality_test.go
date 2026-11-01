package originality

import (
	"strings"
	"testing"
)

const ref = "Climate change is expected to increase the frequency and intensity of extreme weather events across many regions of the world."

func TestUnattributedMatch(t *testing.T) {
	doc := "Our report notes that climate change is expected to increase the frequency and intensity of extreme weather events. We plan accordingly."
	r := Check(doc, []Reference{{Title: "IPCC summary", Text: ref}}, 5)
	if len(r.Matches) != 1 || r.Matches[0].Attribution != "unattributed" {
		t.Fatalf("%+v", r.Matches)
	}
	m := r.Matches[0]
	if got := string([]rune(r.NormalizedText)[m.Start:m.End]); !strings.HasPrefix(strings.ToLower(got), "climate change is expected") {
		t.Fatalf("offsets wrong: %q", got)
	}
	if r.UnattributedPercent <= 0 || r.References[0].Containment <= 0 {
		t.Fatalf("scores: %+v", r)
	}
}

func TestQuotedAndCited(t *testing.T) {
	doc := `As stated, "climate change is expected to increase the frequency and intensity of extreme weather events". ` +
		`Also, the frequency and intensity of extreme weather events across many regions will rise (IPCC, 2021).`
	r := Check(doc, []Reference{{Text: ref}}, 5)
	kinds := map[string]bool{}
	for _, m := range r.Matches {
		kinds[m.Attribution] = true
	}
	if !kinds["quoted"] || !kinds["cited"] || kinds["unattributed"] {
		t.Fatalf("attribution: %+v", r.Matches)
	}
	if r.UnattributedPercent != 0 {
		t.Fatal("expected 0 unattributed")
	}
}

func TestNoMatch(t *testing.T) {
	r := Check("Completely different sentence about cooking rice at home tonight.", []Reference{{Text: ref}}, 5)
	if len(r.Matches) != 0 || r.OverlapPercent != 0 {
		t.Fatalf("%+v", r)
	}
}
