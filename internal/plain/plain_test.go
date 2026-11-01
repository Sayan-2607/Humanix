package plain

import (
	"strings"
	"testing"
)

func TestSimplifyBasic(t *testing.T) {
	r := Simplify("In order to utilize the service, you must commence the process prior to 5 June.", Options{})
	want := "To use the service, you must start the process before 5 June."
	if r.Revised != want {
		t.Fatalf("got %q\nwant %q", r.Revised, want)
	}
	if len(r.Applied) != 4 {
		t.Fatalf("applied %d: %+v", len(r.Applied), r.Applied)
	}
}

func TestSimplifyProtectsQuotesAndTerms(t *testing.T) {
	in := `He said "we utilize data" and we utilize Approximately Inc. tools (Smith, prior to 2020).`
	r := Simplify(in, Options{PreserveTerms: []string{"Approximately Inc"}})
	if !strings.Contains(r.Revised, `"we utilize data"`) {
		t.Errorf("quote modified: %s", r.Revised)
	}
	if !strings.Contains(r.Revised, "Approximately Inc.") {
		t.Errorf("protected term modified: %s", r.Revised)
	}
	if !strings.Contains(r.Revised, "(Smith, prior to 2020)") {
		t.Errorf("citation modified: %s", r.Revised)
	}
	if !strings.Contains(r.Revised, "we use Approximately") {
		t.Errorf("expected replacement outside protected spans: %s", r.Revised)
	}
}

func TestConservativeLevel(t *testing.T) {
	r := Simplify("We utilize tools in order to help.", Options{Level: "conservative"})
	if r.Revised != "We utilize tools to help." {
		t.Fatalf("got %q", r.Revised)
	}
}

func TestUnsafeAreSuggestionsAndArticles(t *testing.T) {
	r := Simplify("This is an endeavor to facilitate access. It needs an approximately equal share.", Options{})
	if strings.Contains(r.Revised, "try") || len(r.Suggestions) < 2 {
		t.Fatalf("unsafe phrase applied or not suggested: %q %+v", r.Revised, r.Suggestions)
	}
	if !strings.Contains(r.Revised, "an about equal") {
		t.Fatalf("article handling: %q", r.Revised)
	}
	r2 := Simplify("We need an additionally large team.", Options{})
	if !strings.Contains(r2.Revised, "an also") {
		t.Fatalf("got %q", r2.Revised)
	}
	r3 := Simplify("It was an assistance program.", Options{})
	if !strings.Contains(r3.Revised, "a help program") {
		t.Fatalf("article a/an not fixed: %q", r3.Revised)
	}
}

func TestCasePreserved(t *testing.T) {
	r := Simplify("Utilize it. UTILIZE it.", Options{})
	if r.Revised != "Use it. USE it." {
		t.Fatalf("got %q", r.Revised)
	}
}
