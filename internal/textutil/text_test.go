package textutil

import "testing"

func TestSentences(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"Dr. Smith arrived. He was late.", 2},
		{"The value is 3.14 today. Next sentence!", 2},
		{"See e.g. the report. Then act?", 2},
		{"J. K. Rowling wrote it. Yes.", 2},
		{"Heading\nBody text here.", 2},
		{"One. Two. Three.", 3},
		{"", 0},
	}
	for _, c := range cases {
		if got := len(Sentences(c.in)); got != c.want {
			t.Errorf("Sentences(%q) = %d, want %d: %+v", c.in, got, c.want, Sentences(c.in))
		}
	}
}

func TestSyllables(t *testing.T) {
	cases := map[string]int{"cat": 1, "table": 2, "readability": 5, "beautiful": 3, "make": 1, "created": 3, "the": 1, "hello": 2}
	bad := 0
	for w, want := range cases {
		if got := Syllables(w); got != want {
			bad++
			t.Logf("Syllables(%q)=%d want %d", w, got, want)
		}
	}
	if bad > 1 { // heuristic: allow one miss
		t.Errorf("too many syllable errors: %d", bad)
	}
}

func TestTokensRuneOffsets(t *testing.T) {
	s := "Café naïve test"
	ts := Tokens(s)
	if len(ts) != 3 || ts[1].Start != 5 || ts[1].End != 10 {
		t.Fatalf("bad tokens: %+v", ts)
	}
	if string([]rune(s)[ts[1].Start:ts[1].End]) != "naïve" {
		t.Fatal("offset mismatch")
	}
}

func TestNormalize(t *testing.T) {
	got := Normalize("a\r\n\r\n\r\n\r\nb   c\x00\t d ")
	if got != "a\n\nb c d" {
		t.Fatalf("got %q", got)
	}
}
