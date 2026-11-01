// Package textutil provides deterministic, dependency-free text primitives:
// normalisation, sentence segmentation, tokenisation with offsets and an
// English syllable estimator. All offsets are rune (Unicode code point)
// offsets so that browsers can map them with Array.from(text).
package textutil

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	reSpaces    = regexp.MustCompile(`[ \t\f\v\x{00A0}]+`)
	reManyBlank = regexp.MustCompile(`\n{3,}`)
	reWord      = regexp.MustCompile(`[\p{L}\p{N}]+(?:['’\-][\p{L}\p{N}]+)*`)
)

// Normalize cleans whitespace without changing wording: unifies line endings,
// removes NUL and other C0 control characters, collapses horizontal
// whitespace, trims lines and limits blank runs to one empty line.
func Normalize(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r == utf8.RuneError || (r < 0x20) || r == 0x7f {
			return -1
		}
		return r
	}, s)
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(reSpaces.ReplaceAllString(l, " "))
	}
	s = strings.Join(lines, "\n")
	s = reManyBlank.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// Token is a word with its rune offsets in the source text.
type Token struct {
	Text  string `json:"text"`
	Lower string `json:"-"`
	Start int    `json:"start"` // rune offset, inclusive
	End   int    `json:"end"`   // rune offset, exclusive
}

// Tokens returns word tokens with rune offsets.
func Tokens(s string) []Token {
	idx := reWord.FindAllStringIndex(s, -1)
	out := make([]Token, 0, len(idx))
	// Convert byte offsets to rune offsets in one forward pass.
	runePos, bytePos := 0, 0
	advance := func(to int) {
		for bytePos < to {
			_, sz := utf8.DecodeRuneInString(s[bytePos:])
			bytePos += sz
			runePos++
		}
	}
	for _, p := range idx {
		advance(p[0])
		start := runePos
		advance(p[1])
		w := s[p[0]:p[1]]
		out = append(out, Token{Text: w, Lower: strings.ToLower(strings.ReplaceAll(w, "’", "'")), Start: start, End: runePos})
	}
	return out
}

// Words returns lower-cased word strings.
func Words(s string) []string {
	ts := Tokens(s)
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Lower
	}
	return out
}

// Sentence is a sentence span in rune offsets.
type Sentence struct {
	Text  string `json:"text"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

var abbreviations = map[string]bool{
	"mr": true, "mrs": true, "ms": true, "dr": true, "prof": true, "sr": true, "jr": true,
	"st": true, "vs": true, "etc": true, "e.g": true, "i.e": true, "al": true, "fig": true,
	"no": true, "vol": true, "pp": true, "p": true, "ed": true, "eds": true, "approx": true,
	"inc": true, "ltd": true, "co": true, "dept": true, "est": true, "u.s": true, "u.k": true,
	"jan": true, "feb": true, "mar": true, "apr": true, "jun": true, "jul": true, "aug": true,
	"sep": true, "sept": true, "oct": true, "nov": true, "dec": true, "cf": true, "ch": true,
}

// Sentences splits text into sentences using punctuation, abbreviation and
// initial heuristics. Paragraph breaks always end a sentence.
func Sentences(s string) []Sentence {
	rs := []rune(s)
	n := len(rs)
	var out []Sentence
	start := 0
	emit := func(end int) {
		// trim
		a, b := start, end
		for a < b && unicode.IsSpace(rs[a]) {
			a++
		}
		for b > a && unicode.IsSpace(rs[b-1]) {
			b--
		}
		if b > a {
			out = append(out, Sentence{Text: string(rs[a:b]), Start: a, End: b})
		}
		start = end
	}
	for i := 0; i < n; i++ {
		r := rs[i]
		if r == '\n' && i+1 < n && rs[i+1] == '\n' {
			emit(i)
			continue
		}
		if r == '\n' {
			// single newline after a line that looks like a heading/list item
			// (no terminal punctuation) is treated as a boundary too.
			emit(i)
			continue
		}
		if r != '.' && r != '!' && r != '?' && r != '…' {
			continue
		}
		// absorb repeated terminators and closing quotes/brackets
		j := i + 1
		for j < n && strings.ContainsRune(".!?…\"'”’)]", rs[j]) {
			j++
		}
		if j < n && !unicode.IsSpace(rs[j]) {
			continue // e.g. 3.14, example.com, U.S.
		}
		if r == '.' && isAbbrevBefore(rs, i) {
			continue
		}
		// next non-space must look like a sentence start (or end of text)
		k := j
		for k < n && rs[k] == ' ' {
			k++
		}
		if k < n && unicode.IsLower(rs[k]) {
			continue
		}
		emit(j)
		i = j - 1
	}
	emit(n)
	return out
}

func isAbbrevBefore(rs []rune, dot int) bool {
	k := dot - 1
	for k >= 0 && (unicode.IsLetter(rs[k]) || rs[k] == '.') {
		k--
	}
	w := strings.ToLower(string(rs[k+1 : dot]))
	if w == "" {
		return false
	}
	if abbreviations[w] {
		return true
	}
	// single capital initial, e.g. "J. Smith"
	if utf8.RuneCountInString(w) == 1 && unicode.IsUpper(rs[dot-1]) {
		return true
	}
	return false
}

var reVowelGroups = regexp.MustCompile(`[aeiouy]+`)

// Syllables estimates English syllables with a standard vowel-group heuristic.
// Typical accuracy is roughly 85–90% on common vocabulary; it is used only for
// aggregate readability formulas, where per-word errors largely average out.
func Syllables(word string) int {
	w := strings.ToLower(word)
	w = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' {
			return r
		}
		return -1
	}, w)
	if w == "" {
		return 0
	}
	if len(w) <= 3 {
		return 1
	}
	if strings.HasSuffix(w, "es") || strings.HasSuffix(w, "ed") {
		if !strings.HasSuffix(w, "ted") && !strings.HasSuffix(w, "ded") && !strings.HasSuffix(w, "ses") && !strings.HasSuffix(w, "zes") && !strings.HasSuffix(w, "ces") && !strings.HasSuffix(w, "ges") {
			w = w[:len(w)-2]
		}
	} else if strings.HasSuffix(w, "e") && !strings.HasSuffix(w, "le") && !strings.HasSuffix(w, "ee") {
		w = w[:len(w)-1]
	}
	if strings.HasPrefix(w, "y") {
		w = w[1:]
	}
	c := len(reVowelGroups.FindAllString(w, -1))
	if c < 1 {
		c = 1
	}
	return c
}

// IsNumeric reports whether a token is purely numeric.
func IsNumeric(w string) bool {
	for _, r := range w {
		if !unicode.IsDigit(r) && r != '.' && r != ',' && r != '-' {
			return false
		}
	}
	return w != ""
}

// Stopwords is a compact English function-word list.
var Stopwords = func() map[string]bool {
	list := `a about above after again against all am an and any are aren't as at be because been before being below
between both but by can can't cannot could couldn't did didn't do does doesn't doing don't down during each few for
from further had hadn't has hasn't have haven't having he her here hers herself him himself his how i if in into is
isn't it it's its itself let's me more most my myself no nor not of off on once only or other ought our ours
ourselves out over own same she should shouldn't so some such than that that's the their theirs them themselves then
there there's these they this those through to too under until up very was wasn't we were weren't what when where
which while who whom why will with won't would wouldn't you your yours yourself yourselves also may might must shall
upon us`
	m := map[string]bool{}
	for _, w := range strings.Fields(list) {
		m[w] = true
	}
	return m
}()

// ContentWords returns lower-cased non-stopword, non-numeric words.
func ContentWords(s string) []string {
	var out []string
	for _, w := range Words(s) {
		if !Stopwords[w] && !IsNumeric(w) && utf8.RuneCountInString(w) > 1 {
			out = append(out, w)
		}
	}
	return out
}

// Stem applies a light, conservative English suffix stripper (not Porter);
// it only merges obvious inflections such as plurals and -ing/-ed forms.
func Stem(w string) string {
	if len(w) <= 4 {
		return w
	}
	for _, suf := range []string{"ies", "ing", "ed", "es", "s"} {
		if strings.HasSuffix(w, suf) && len(w)-len(suf) >= 3 {
			base := w[:len(w)-len(suf)]
			if suf == "ies" {
				return base + "y"
			}
			if suf == "s" && strings.HasSuffix(w, "ss") {
				return w
			}
			return base
		}
	}
	return w
}

// RuneLen returns the number of runes.
func RuneLen(s string) int { return utf8.RuneCountInString(s) }
