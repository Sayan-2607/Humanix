// Package preserve checks whether a revised text keeps the factual "anchors"
// of the original: numbers, negations, modal verbs (obligation/possibility),
// named entities, citations and URLs, plus lexical overlap.
//
// It is a lexical verifier, not a semantic model. High overlap does not prove
// equivalent meaning, and low overlap does not prove a change of meaning;
// the anchor checks catch the specific edits that most often flip meaning
// (dropped "not", 15 -> 50, "must" -> "may"). A human should review flags.
package preserve

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/humanix-ai/humanix/internal/textutil"
)

type Diff struct {
	Missing []string `json:"missing"`
	Added   []string `json:"added"`
}

type Report struct {
	LexicalCosine  float64        `json:"lexical_cosine"` // content-word TF cosine
	CharTrigramCos float64        `json:"char_trigram_cosine"`
	Numbers        Diff           `json:"numbers"`
	Negations      map[string]int `json:"negation_counts"` // original, revised
	Modals         Diff           `json:"modal_verbs"`
	Entities       Diff           `json:"named_entities"`
	Citations      Diff           `json:"citations"`
	URLs           Diff           `json:"urls"`
	Risk           string         `json:"risk"` // low | medium | high
	Warnings       []string       `json:"warnings"`
	Method         string         `json:"method"`
}

var (
	reNumber   = regexp.MustCompile(`\d+(?:[.,]\d+)*\s?%?`)
	reURL      = regexp.MustCompile(`https?://[^\s<>"')\]]+`)
	reCitation = regexp.MustCompile(`\([A-Z][^()\n]{0,120}?\d{4}[a-z]?\)|\[\d+(?:\s*[,–-]\s*\d+)*\]|10\.\d{4,9}/[^\s"<>]+`)
	negations  = map[string]bool{"not": true, "no": true, "never": true, "none": true, "nobody": true, "nothing": true,
		"neither": true, "nor": true, "without": true, "cannot": true, "nowhere": true}
	// Modal/deontic words whose substitution changes obligation or certainty.
	// "can/could/will/would" are excluded because plain-language rewrites
	// legitimately introduce them ("is able to" -> "can").
	modals = map[string]bool{"must": true, "may": true, "might": true, "should": true, "shall": true,
		"required": true, "optional": true, "mandatory": true, "prohibited": true, "forbidden": true}
)

// Compare original and revised text.
func Compare(original, revised string) Report {
	a, b := textutil.Normalize(original), textutil.Normalize(revised)
	r := Report{
		Method:   "Lexical cosine on stemmed content words + character-trigram cosine + anchor diffs (numbers, negation, modality, entities, citations, URLs).",
		Warnings: []string{},
	}
	r.LexicalCosine = round(cosine(tf(stems(a)), tf(stems(b))))
	r.CharTrigramCos = round(cosine(trigrams(a), trigrams(b)))
	r.Numbers = diff(normNums(reNumber.FindAllString(a, -1)), normNums(reNumber.FindAllString(b, -1)))
	r.URLs = diff(trimAll(reURL.FindAllString(a, -1)), trimAll(reURL.FindAllString(b, -1)))
	r.Citations = diff(reCitation.FindAllString(a, -1), reCitation.FindAllString(b, -1))
	r.Entities = diff(entities(a), entities(b))
	r.Modals = diff(filter(textutil.Words(a), modals), filter(textutil.Words(b), modals))
	na, nb := countNeg(a), countNeg(b)
	r.Negations = map[string]int{"original": na, "revised": nb}

	score := 0
	if len(r.Numbers.Missing)+len(r.Numbers.Added) > 0 {
		score += 3
		r.Warnings = append(r.Warnings, "Numbers changed: missing "+list(r.Numbers.Missing)+"; added "+list(r.Numbers.Added)+".")
	}
	if na != nb {
		score += 3
		r.Warnings = append(r.Warnings, "Number of negations changed — a statement may have been reversed.")
	}
	if len(r.Citations.Missing) > 0 {
		score += 2
		r.Warnings = append(r.Warnings, "Citations removed: "+list(r.Citations.Missing)+".")
	}
	if len(r.URLs.Missing) > 0 {
		score += 1
		r.Warnings = append(r.Warnings, "Links removed: "+list(r.URLs.Missing)+".")
	}
	if len(r.Modals.Missing)+len(r.Modals.Added) > 0 {
		score += 2
		r.Warnings = append(r.Warnings, "Obligation/possibility words changed (e.g. must/may): missing "+list(r.Modals.Missing)+"; added "+list(r.Modals.Added)+".")
	}
	if len(r.Entities.Missing) > 0 {
		score += 1
		r.Warnings = append(r.Warnings, "Names or proper nouns no longer present: "+list(r.Entities.Missing)+".")
	}
	if r.LexicalCosine < 0.5 {
		score += 2
		r.Warnings = append(r.Warnings, "Low content-word overlap; the revision may have dropped or added ideas.")
	}
	switch {
	case score >= 3:
		r.Risk = "high"
	case score >= 1:
		r.Risk = "medium"
	default:
		r.Risk = "low"
	}
	return r
}

func stems(s string) []string {
	ws := textutil.ContentWords(s)
	for i, w := range ws {
		ws[i] = textutil.Stem(w)
	}
	return ws
}

func tf(ws []string) map[string]float64 {
	m := map[string]float64{}
	for _, w := range ws {
		m[w]++
	}
	return m
}

func trigrams(s string) map[string]float64 {
	rs := []rune(strings.ToLower(s))
	m := map[string]float64{}
	for i := 0; i+3 <= len(rs); i++ {
		m[string(rs[i:i+3])]++
	}
	return m
}

func cosine(a, b map[string]float64) float64 {
	if len(a) == 0 || len(b) == 0 {
		if len(a) == 0 && len(b) == 0 {
			return 1
		}
		return 0
	}
	var dot, na, nb float64
	for k, v := range a {
		na += v * v
		dot += v * b[k]
	}
	for _, v := range b {
		nb += v * v
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// diff computes multiset differences.
func diff(a, b []string) Diff {
	ca, cb := map[string]int{}, map[string]int{}
	for _, x := range a {
		ca[x]++
	}
	for _, x := range b {
		cb[x]++
	}
	d := Diff{Missing: []string{}, Added: []string{}}
	for k, n := range ca {
		for i := cb[k]; i < n; i++ {
			d.Missing = append(d.Missing, k)
		}
	}
	for k, n := range cb {
		for i := ca[k]; i < n; i++ {
			d.Added = append(d.Added, k)
		}
	}
	sort.Strings(d.Missing)
	sort.Strings(d.Added)
	return d
}

func normNums(xs []string) []string {
	for i, x := range xs {
		xs[i] = strings.ReplaceAll(strings.TrimSpace(x), " ", "")
	}
	return xs
}

func trimAll(xs []string) []string {
	for i, x := range xs {
		xs[i] = strings.TrimRight(x, ".,;:")
	}
	return xs
}

func filter(ws []string, set map[string]bool) []string {
	var out []string
	for _, w := range ws {
		if set[w] {
			out = append(out, w)
		}
	}
	return out
}

func countNeg(s string) int {
	n := 0
	for _, w := range textutil.Words(s) {
		if negations[w] || strings.HasSuffix(w, "n't") || strings.HasSuffix(w, "n’t") {
			n++
		}
	}
	return n
}

// entities: capitalised tokens that are not sentence-initial, plus acronyms.
func entities(s string) []string {
	var out []string
	for _, sent := range textutil.Sentences(s) {
		toks := textutil.Tokens(sent.Text)
		for i, t := range toks {
			r := []rune(t.Text)
			if !unicode.IsUpper(r[0]) {
				continue
			}
			isAcr := len(r) >= 2 && strings.ToUpper(t.Text) == t.Text
			if i == 0 && !isAcr {
				continue
			}
			if t.Text == "I" {
				continue
			}
			out = append(out, t.Text)
		}
	}
	return out
}

func list(xs []string) string {
	if len(xs) == 0 {
		return "none"
	}
	if len(xs) > 8 {
		return strings.Join(xs[:8], ", ") + ", …"
	}
	return strings.Join(xs, ", ")
}

func round(x float64) float64 {
	if math.IsNaN(x) {
		return 0
	}
	return math.Round(x*1000) / 1000
}
