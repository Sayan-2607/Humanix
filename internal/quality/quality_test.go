package quality

import (
	"math"
	"strings"
	"testing"
)

func TestReadabilityKnownValues(t *testing.T) {
	// 2 sentences, 10 words; syllables counted by our heuristic.
	r := Analyze("The cat sat on the mat. The dog ran fast.", "general")
	if r.Counts.Words != 10 || r.Counts.Sentences != 2 {
		t.Fatalf("counts %+v", r.Counts)
	}
	W, S, Sy := 10.0, 2.0, float64(r.Counts.Syllables)
	want := 206.835 - 1.015*(W/S) - 84.6*(Sy/W)
	if math.Abs(r.Readability.FleschReadingEase-math.Round(want*10)/10) > 1e-9 {
		t.Fatalf("FRE %v want %v", r.Readability.FleschReadingEase, want)
	}
	if !r.Audience.MeetsTarget {
		t.Fatal("simple text should meet general target")
	}
}

func TestHardTextFlagged(t *testing.T) {
	txt := strings.Repeat("The implementation of comprehensive organizational methodologies necessitates considerable interdepartmental coordination and the utilization of sophisticated technological infrastructure in order to facilitate operational effectiveness across geographically distributed administrative units. ", 3)
	r := Analyze(txt, "health")
	if r.Audience.MeetsTarget || len(r.Recommendations) == 0 || len(r.Issues.LongSentences) != 3 {
		t.Fatalf("expected issues: %+v", r)
	}
	if len(r.Issues.WordyPhrases) == 0 {
		t.Fatal("wordy phrases not detected")
	}
}

func TestPassive(t *testing.T) {
	r := Analyze("The report was written by the team. Mistakes were made. The seed is planted.", "general")
	if len(r.Issues.PassiveCandidates) != 3 {
		t.Fatalf("passive: %+v", r.Issues.PassiveCandidates)
	}
}

func TestLexicalDiversity(t *testing.T) {
	if v := mattr([]string{"a", "b", "a", "b"}, 2); v != 1 {
		t.Fatalf("mattr %v", v)
	}
	same := strings.Fields(strings.Repeat("x ", 200))
	if m := mtldPass(same); m > 2 {
		t.Fatalf("mtld of constant text should be ~1, got %v", m)
	}
}

func TestEmpty(t *testing.T) {
	r := Analyze("   ", "")
	if r.Counts.Words != 0 || r.Recommendations == nil {
		t.Fatal("empty handling")
	}
}
