// Package originality compares a document against reference texts that the
// user supplies. It uses word k-shingling (Broder, 1997) to locate verbatim or
// near-verbatim overlap, reports each matched passage with offsets, and
// classifies it as quoted, cited, or unattributed.
//
// Scope: it only knows the references it is given. It does not search the
// internet or any commercial database, and its numbers are not comparable
// to Turnitin/iThenticate scores.
package originality

import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/humanix-ai/humanix/internal/textutil"
)

type Reference struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

type Match struct {
	ReferenceID    string `json:"reference_id"`
	ReferenceTitle string `json:"reference_title"`
	Start          int    `json:"start"` // rune offsets in normalised document
	End            int    `json:"end"`
	Words          int    `json:"words"`
	Text           string `json:"text"`
	Attribution    string `json:"attribution"` // quoted | cited | unattributed
}

type RefScore struct {
	ID           string  `json:"id"`
	Title        string  `json:"title"`
	Containment  float64 `json:"containment"` // share of document shingles found in reference
	Jaccard      float64 `json:"jaccard"`
	Cosine       float64 `json:"tfidf_cosine"`
	MatchedWords int     `json:"matched_words"`
}

type Report struct {
	NormalizedText      string     `json:"normalized_text"`
	ShingleSize         int        `json:"shingle_size"`
	TotalWords          int        `json:"total_words"`
	MatchedWords        int        `json:"matched_words"`
	OverlapPercent      float64    `json:"overlap_percent"`
	UnattributedPercent float64    `json:"unattributed_percent"`
	References          []RefScore `json:"references"`
	Matches             []Match    `json:"matches"`
	Interpretation      string     `json:"interpretation"`
	Scope               string     `json:"scope"`
}

var (
	reQuote    = regexp.MustCompile(`"[^"]+"|“[^”]+”|«[^»]+»`)
	reCiteNear = regexp.MustCompile(`\([A-Z][^()\n]{0,120}?\d{4}[a-z]?[^()\n]{0,30}\)|\[\d+(?:\s*[,–-]\s*\d+)*\]|\bdoi\.org/|\b10\.\d{4,9}/`)
)

// Check runs the comparison. k is the shingle size (3–12, default 5).
func Check(doc string, refs []Reference, k int) Report {
	if k < 3 || k > 12 {
		k = 5
	}
	doc = textutil.Normalize(doc)
	dt := textutil.Tokens(doc)
	dw := make([]string, len(dt))
	for i, t := range dt {
		dw[i] = t.Lower
	}
	rep := Report{
		NormalizedText: doc, ShingleSize: k, TotalWords: len(dw),
		References: []RefScore{}, Matches: []Match{},
		Scope: "Compared only against the reference texts you provided. No internet or database search was performed.",
	}

	docSh := shingles(dw, k)
	covered := make([]bool, len(dw))
	unattributed := make([]bool, len(dw))

	// rune spans for quotes and per-sentence citation presence
	runeIdx := runeIndex(doc)
	var quotes [][2]int
	for _, m := range reQuote.FindAllStringIndex(doc, -1) {
		quotes = append(quotes, [2]int{runeIdx(m[0]), runeIdx(m[1])})
	}
	sents := textutil.Sentences(doc)
	docRunes := []rune(doc)

	// IDF over {doc} ∪ refs for cosine
	allDocs := [][]string{stemmed(doc)}
	for _, r := range refs {
		allDocs = append(allDocs, stemmed(textutil.Normalize(r.Text)))
	}
	idf := computeIDF(allDocs)
	docVec := tfidf(allDocs[0], idf)

	for ri, r := range refs {
		rw := textutil.Words(textutil.Normalize(r.Text))
		refSh := shingles(rw, k)
		refSet := make(map[string]struct{}, len(refSh))
		for _, s := range refSh {
			refSet[s] = struct{}{}
		}
		hit := make([]bool, len(docSh))
		inter := 0
		docSet := map[string]struct{}{}
		for i, s := range docSh {
			docSet[s] = struct{}{}
			if _, ok := refSet[s]; ok {
				hit[i] = true
			}
		}
		for s := range docSet {
			if _, ok := refSet[s]; ok {
				inter++
			}
		}
		union := len(docSet) + len(refSet) - inter
		matchedWords := make([]bool, len(dw))
		// merge consecutive hit shingles into passages
		for i := 0; i < len(hit); {
			if !hit[i] {
				i++
				continue
			}
			j := i
			for j+1 < len(hit) && hit[j+1] {
				j++
			}
			a, b := i, j+k // word indices [a,b)
			for w := a; w < b; w++ {
				matchedWords[w] = true
				covered[w] = true
			}
			start, end := dt[a].Start, dt[b-1].End
			attr := "unattributed"
			if inside(start, end, quotes) {
				attr = "quoted"
			} else if citedNearby(start, end, sents) {
				attr = "cited"
			}
			if attr == "unattributed" {
				for w := a; w < b; w++ {
					unattributed[w] = true
				}
			}
			rep.Matches = append(rep.Matches, Match{
				ReferenceID: refID(r, ri), ReferenceTitle: r.Title,
				Start: start, End: end, Words: b - a, Text: string(docRunes[start:end]), Attribution: attr,
			})
			i = j + 1
		}
		mw := 0
		for _, m := range matchedWords {
			if m {
				mw++
			}
		}
		rs := RefScore{ID: refID(r, ri), Title: r.Title, MatchedWords: mw,
			Cosine: round(cosine(docVec, tfidf(allDocs[ri+1], idf)))}
		if len(docSet) > 0 {
			rs.Containment = round(float64(inter) / float64(len(docSet)))
		}
		if union > 0 {
			rs.Jaccard = round(float64(inter) / float64(union))
		}
		rep.References = append(rep.References, rs)
	}

	for i := range covered {
		if covered[i] {
			rep.MatchedWords++
		}
	}
	ua := 0
	for _, u := range unattributed {
		if u {
			ua++
		}
	}
	if len(dw) > 0 {
		rep.OverlapPercent = round(100 * float64(rep.MatchedWords) / float64(len(dw)))
		rep.UnattributedPercent = round(100 * float64(ua) / float64(len(dw)))
	}
	sort.Slice(rep.Matches, func(i, j int) bool { return rep.Matches[i].Start < rep.Matches[j].Start })
	sort.Slice(rep.References, func(i, j int) bool { return rep.References[i].Containment > rep.References[j].Containment })

	switch {
	case len(refs) == 0:
		rep.Interpretation = "No references supplied."
	case rep.UnattributedPercent >= 15:
		rep.Interpretation = "Substantial overlap without quotation marks or nearby citations. Review the highlighted passages: quote them, cite them, or rewrite in your own words and cite the idea."
	case rep.UnattributedPercent > 0:
		rep.Interpretation = "Some unattributed overlap found. Short shared phrases can be coincidental or standard terminology; review each passage."
	case rep.MatchedWords > 0:
		rep.Interpretation = "Overlap found, but each passage is quoted or has a citation nearby. Check that the citations point to the right sources."
	default:
		rep.Interpretation = "No shared " + itoa(k) + "-word sequences with the supplied references. Paraphrased ideas still need citations."
	}
	return rep
}

func shingles(ws []string, k int) []string {
	if len(ws) < k {
		return nil
	}
	out := make([]string, 0, len(ws)-k+1)
	for i := 0; i+k <= len(ws); i++ {
		out = append(out, strings.Join(ws[i:i+k], " "))
	}
	return out
}

func inside(a, b int, spans [][2]int) bool {
	for _, s := range spans {
		if a >= s[0] && b <= s[1] {
			return true
		}
	}
	return false
}

// citedNearby: a citation marker occurs in any sentence overlapping the
// passage, or in the sentence immediately after it.
func citedNearby(a, b int, sents []textutil.Sentence) bool {
	for i, s := range sents {
		if s.End <= a || s.Start >= b {
			continue
		}
		if reCiteNear.MatchString(s.Text) {
			return true
		}
		if i+1 < len(sents) && sents[i+1].Start >= b && reCiteNear.MatchString(sents[i+1].Text) && len(textutil.Words(sents[i+1].Text)) <= 12 {
			return true
		}
	}
	return false
}

func runeIndex(s string) func(int) int {
	pos := make([]int, len(s)+1)
	r := 0
	for i := 0; i < len(s); {
		_, sz := utf8.DecodeRuneInString(s[i:])
		for j := i; j < i+sz; j++ {
			pos[j] = r
		}
		i += sz
		r++
	}
	pos[len(s)] = r
	return func(b int) int { return pos[b] }
}

func stemmed(s string) []string {
	ws := textutil.ContentWords(s)
	for i, w := range ws {
		ws[i] = textutil.Stem(w)
	}
	return ws
}

func computeIDF(docs [][]string) map[string]float64 {
	df := map[string]int{}
	for _, d := range docs {
		seen := map[string]bool{}
		for _, w := range d {
			if !seen[w] {
				df[w]++
				seen[w] = true
			}
		}
	}
	n := float64(len(docs))
	idf := make(map[string]float64, len(df))
	for w, c := range df {
		idf[w] = math.Log((1+n)/(1+float64(c))) + 1 // smoothed IDF
	}
	return idf
}

func tfidf(ws []string, idf map[string]float64) map[string]float64 {
	m := map[string]float64{}
	for _, w := range ws {
		m[w]++
	}
	for w, v := range m {
		m[w] = v * idf[w]
	}
	return m
}

func cosine(a, b map[string]float64) float64 {
	var dot, na, nb float64
	for k, v := range a {
		na += v * v
		dot += v * b[k]
	}
	for _, v := range b {
		nb += v * v
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

func refID(r Reference, i int) string {
	if r.ID != "" {
		return r.ID
	}
	return "ref-" + itoa(i+1)
}

func itoa(i int) string { return strconv.Itoa(i) }

func round(x float64) float64 { return math.Round(x*1000) / 1000 }
