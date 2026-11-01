// Package plain implements a transparent, rule-based plain-language rewriter.
//
// Design principle: every change is a documented, reversible substitution
// with its location. The rewriter never touches quoted text, code, URLs,
// citations, numbers or user-protected terms, and it never deletes content.
// Context-sensitive substitutions are returned as suggestions for a human
// to accept, not applied.
package plain

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Options controls the rewriter.
type Options struct {
	// Level: "conservative" applies only multi-word "wordy" fixes;
	// "standard" also replaces complex single words. Default "standard".
	Level string `json:"level"`
	// PreserveTerms are never modified (case-insensitive).
	PreserveTerms []string `json:"preserve_terms"`
}

// Change describes one applied substitution or one suggestion.
type Change struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Kind   string `json:"kind"`
	Start  int    `json:"start"` // rune offset in the ORIGINAL text
	End    int    `json:"end"`
	Reason string `json:"reason,omitempty"`
}

// Result of a rewrite.
type Result struct {
	Revised     string   `json:"revised_text"`
	Applied     []Change `json:"applied"`
	Suggestions []Change `json:"suggestions"`
}

type compiled struct {
	p  Phrase
	re *regexp.Regexp
}

var compiledPhrases = func() []compiled {
	out := make([]compiled, 0, len(Phrases))
	for _, p := range Phrases {
		parts := strings.Fields(p.From)
		for i := range parts {
			parts[i] = regexp.QuoteMeta(parts[i])
		}
		re := regexp.MustCompile(`(?i)\b` + strings.Join(parts, `\s+`) + `\b`)
		out = append(out, compiled{p, re})
	}
	return out
}()

var protectedPatterns = []*regexp.Regexp{
	regexp.MustCompile(`"[^"\n]*"`),
	regexp.MustCompile(`“[^”\n]*”`),
	regexp.MustCompile("`[^`\n]*`"),
	regexp.MustCompile(`https?://\S+`),
	regexp.MustCompile(`\b[\w.+-]+@[\w-]+\.[\w.-]+\b`),
	regexp.MustCompile(`\([^()\n]{0,200}\b\d{4}[a-z]?\b[^()\n]{0,50}\)`), // (Author, 2020)
	regexp.MustCompile(`\[\d+(?:\s*[,–-]\s*\d+)*\]`),                     // [1], [2-4]
}

type span struct{ a, b int }

func overlaps(s span, list []span) bool {
	for _, t := range list {
		if s.a < t.b && t.a < s.b {
			return true
		}
	}
	return false
}

// Simplify rewrites text according to opts.
func Simplify(text string, opts Options) Result {
	level := strings.ToLower(opts.Level)
	if level == "" {
		level = "standard"
	}

	var protected []span
	for _, re := range protectedPatterns {
		for _, m := range re.FindAllStringIndex(text, -1) {
			protected = append(protected, span{m[0], m[1]})
		}
	}
	for _, term := range opts.PreserveTerms {
		term = strings.TrimSpace(term)
		if term == "" {
			continue
		}
		re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(term))
		for _, m := range re.FindAllStringIndex(text, -1) {
			protected = append(protected, span{m[0], m[1]})
		}
	}

	type hit struct {
		s     span
		p     Phrase
		apply bool
	}
	var taken []span
	var hits []hit
	for _, c := range compiledPhrases { // longest phrases first
		apply := c.p.Safe && (level == "standard" || c.p.Kind == "wordy")
		for _, m := range c.re.FindAllStringIndex(text, -1) {
			s := span{m[0], m[1]}
			if overlaps(s, protected) || overlaps(s, taken) {
				continue
			}
			taken = append(taken, s)
			hits = append(hits, hit{s, c.p, apply})
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].s.a < hits[j].s.a })

	var b strings.Builder
	res := Result{Applied: []Change{}, Suggestions: []Change{}}
	last := 0
	for _, h := range hits {
		orig := text[h.s.a:h.s.b]
		rep := matchCase(orig, h.p.To)
		ch := Change{
			From: orig, To: rep, Kind: h.p.Kind, Reason: h.p.Reason,
			Start: utf8.RuneCountInString(text[:h.s.a]),
			End:   utf8.RuneCountInString(text[:h.s.b]),
		}
		if !h.apply {
			if ch.Reason == "" {
				ch.Reason = "not applied automatically at this level"
			}
			res.Suggestions = append(res.Suggestions, ch)
			continue
		}
		prefix := fixArticle(text[last:h.s.a], rep)
		b.WriteString(prefix)
		b.WriteString(rep)
		last = h.s.b
		res.Applied = append(res.Applied, ch)
	}
	b.WriteString(text[last:])
	res.Revised = b.String()
	return res
}

// matchCase transfers the capitalisation pattern of orig onto rep.
func matchCase(orig, rep string) string {
	if orig == strings.ToUpper(orig) && strings.ToUpper(orig) != strings.ToLower(orig) && utf8.RuneCountInString(orig) > 1 {
		return strings.ToUpper(rep)
	}
	r, _ := utf8.DecodeRuneInString(orig)
	if unicode.IsUpper(r) {
		rr, sz := utf8.DecodeRuneInString(rep)
		return string(unicode.ToUpper(rr)) + rep[sz:]
	}
	return rep
}

var reTrailingArticle = regexp.MustCompile(`(?i)\b(a|an)(\s+)$`)

// fixArticle corrects a/an immediately before a replacement.
func fixArticle(prefix, rep string) string {
	m := reTrailingArticle.FindStringSubmatchIndex(prefix)
	if m == nil {
		return prefix
	}
	art := prefix[m[2]:m[3]]
	first := unicode.ToLower([]rune(rep)[0])
	want := "a"
	if strings.ContainsRune("aeiou", first) {
		want = "an"
	}
	if strings.EqualFold(art, want) {
		return prefix
	}
	if unicode.IsUpper([]rune(art)[0]) {
		want = strings.ToUpper(want[:1]) + want[1:]
	}
	return prefix[:m[2]] + want + prefix[m[3]:]
}
