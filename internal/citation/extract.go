// Package citation extracts citation markers, flags claims that may need a
// source, formats references in APA 7, MLA 9, IEEE and Chicago
// (notes-bibliography) styles, and can verify DOIs against Crossref.
package citation

import (
	"regexp"
	"strings"

	"github.com/humanix-ai/humanix/internal/textutil"
)

type Found struct {
	Type  string `json:"type"` // doi | url | author_year | numeric
	Value string `json:"value"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

type Claim struct {
	Sentence string `json:"sentence"`
	Start    int    `json:"start"`
	End      int    `json:"end"`
	Reason   string `json:"reason"`
}

type Extraction struct {
	Citations      []Found `json:"citations"`
	UncitedClaims  []Claim `json:"claims_possibly_needing_citation"`
	SentencesTotal int     `json:"sentences_total"`
	SentencesCited int     `json:"sentences_with_citation"`
	Note           string  `json:"note"`
}

var (
	reDOI        = regexp.MustCompile(`\b10\.\d{4,9}/[-._;()/:A-Za-z0-9]+`)
	reURL        = regexp.MustCompile(`https?://[^\s<>"'\]]+`)
	reAuthorYear = regexp.MustCompile(`\((?:see |e\.g\., )?[A-Z][\p{L}'’\-]+(?: et al\.)?(?:,? (?:&|and) [A-Z][\p{L}'’\-]+)?,? (?:\d{4}[a-z]?|n\.d\.)(?:[;,] ?[^()]{0,120})?\)`)
	reNarrative  = regexp.MustCompile(`\b[A-Z][\p{L}'’\-]+(?: et al\.)?(?: (?:&|and) [A-Z][\p{L}'’\-]+)? \(\d{4}[a-z]?\)`)
	reNumeric    = regexp.MustCompile(`\[\d+(?:\s*[,–-]\s*\d+)*\]`)

	claimSignals = []struct {
		re     *regexp.Regexp
		reason string
	}{
		{regexp.MustCompile(`\b\d+(?:\.\d+)?\s?(?:%|percent\b|per cent\b)`), "contains a statistic"},
		{regexp.MustCompile(`(?i)\b(studies|research|evidence|data|surveys?|experts?|scientists|researchers)\s+(show|shows|showed|suggest|suggests|found|find|finds|indicate|indicates|demonstrate[sd]?|confirm[sed]?)\b`), "refers to research findings"},
		{regexp.MustCompile(`(?i)\baccording to\b`), "attributes a claim to a source"},
		{regexp.MustCompile(`(?i)\bit (?:has been|is) (?:shown|proven|established|estimated|reported)\b`), "reports an established finding"},
		{regexp.MustCompile(`(?i)\b(million|billion|thousand)s?\b`), "contains a large quantity"},
		{regexp.MustCompile(`(?i)\b(in|since|by) (1[5-9]\d\d|20\d\d)\b`), "states a dated fact"},
	}
)

// Extract finds citations and flags sentences that look like factual claims
// without any citation marker. The claim detector is a keyword heuristic:
// it favours recall, so expect some false positives.
func Extract(text string) Extraction {
	text = textutil.Normalize(text)
	ex := Extraction{Citations: []Found{}, UncitedClaims: []Claim{},
		Note: "Claim flags are heuristic prompts for review, not judgements that a source is required."}
	idx := runeIndexer(text)
	add := func(typ string, re *regexp.Regexp, trim bool) {
		for _, m := range re.FindAllStringIndex(text, -1) {
			v := text[m[0]:m[1]]
			e := m[1]
			if trim {
				t := strings.TrimRight(v, ".,;:)")
				// keep balanced parentheses inside DOIs like 10.1016/S0140-6736(20)30183-5
				if strings.Count(v, "(") > strings.Count(t, ")") && strings.HasSuffix(v, ")") && strings.Count(t, "(") > strings.Count(t, ")") {
					t += ")"
				}
				e -= len(v) - len(t)
				v = t
			}
			ex.Citations = append(ex.Citations, Found{typ, v, idx(m[0]), idx(e)})
		}
	}
	add("doi", reDOI, true)
	add("url", reURL, true)
	add("author_year", reAuthorYear, false)
	add("author_year", reNarrative, false)
	add("numeric", reNumeric, false)
	ex.Citations = dedupe(ex.Citations)

	sents := textutil.Sentences(text)
	ex.SentencesTotal = len(sents)
	for _, s := range sents {
		cited := false
		for _, c := range ex.Citations {
			if c.Start >= s.Start && c.Start < s.End {
				cited = true
				break
			}
		}
		if cited {
			ex.SentencesCited++
			continue
		}
		for _, sig := range claimSignals {
			if sig.re.MatchString(s.Text) {
				ex.UncitedClaims = append(ex.UncitedClaims, Claim{s.Text, s.Start, s.End, sig.reason})
				break
			}
		}
	}
	return ex
}

// dedupe drops a URL that only wraps an already-found DOI, and exact duplicates of span.
func dedupe(fs []Found) []Found {
	out := []Found{}
	for _, f := range fs {
		skip := false
		for _, g := range fs {
			if f == g {
				continue
			}
			if f.Start >= g.Start && f.End <= g.End && (f.End-f.Start) < (g.End-g.Start) && f.Type != "doi" {
				skip = true // contained in a larger match (e.g. narrative inside parenthetical)
				break
			}
		}
		if !skip {
			out = append(out, f)
		}
	}
	// stable sort by position
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Start < out[j-1].Start; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func runeIndexer(s string) func(int) int {
	pos := make([]int, len(s)+1)
	r := 0
	for i := range s {
		pos[i] = r
		r++
	}
	pos[len(s)] = r
	return func(b int) int {
		for b > 0 && b < len(s) && (s[b]&0xC0) == 0x80 {
			b--
		}
		return pos[b]
	}
}
