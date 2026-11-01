// Package quality computes transparent, reproducible writing metrics:
// classic readability formulas, lexical diversity, repetition and
// plain-language issues. Every number is a deterministic function of the
// input text; nothing here estimates authorship (human vs. machine).
package quality

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/humanix-ai/humanix/internal/plain"
	"github.com/humanix-ai/humanix/internal/textutil"
)

// Audience presets: target U.S. school grade levels commonly used in
// plain-language and health-literacy guidance. They are conventions, not laws.
var Audiences = map[string]struct {
	Label       string
	TargetGrade float64
	MaxAvgWords float64
}{
	"general":    {"General public", 8, 20},
	"health":     {"Patient / health information", 6, 15},
	"government": {"Public notices & forms", 8, 20},
	"youth":      {"Young readers", 5, 12},
	"academic":   {"Academic / specialist", 14, 25},
}

type Counts struct {
	Characters        int     `json:"characters"`
	Letters           int     `json:"letters"`
	Words             int     `json:"words"`
	Sentences         int     `json:"sentences"`
	Paragraphs        int     `json:"paragraphs"`
	Syllables         int     `json:"syllables"`
	ComplexWords      int     `json:"complex_words"` // >= 3 syllables
	AvgWordsPerSent   float64 `json:"avg_words_per_sentence"`
	SentenceLengthSD  float64 `json:"sentence_length_sd"`
	LongestSentence   int     `json:"longest_sentence_words"`
	AvgSyllablesWord  float64 `json:"avg_syllables_per_word"`
	ReadingTimeMinute float64 `json:"reading_time_minutes"` // at 200 wpm
}

type Readability struct {
	FleschReadingEase float64  `json:"flesch_reading_ease"`
	FleschKincaid     float64  `json:"flesch_kincaid_grade"`
	GunningFog        float64  `json:"gunning_fog"`
	SMOG              float64  `json:"smog"`
	ColemanLiau       float64  `json:"coleman_liau"`
	ARI               float64  `json:"automated_readability_index"`
	ConsensusGrade    float64  `json:"consensus_grade"`
	EaseBand          string   `json:"ease_band"`
	Notes             []string `json:"notes"`
}

type Lexical struct {
	TTR          float64 `json:"type_token_ratio"`
	MATTR        float64 `json:"mattr"`
	MATTRWindow  int     `json:"mattr_window"`
	MTLD         float64 `json:"mtld"`
	MTLDReliable bool    `json:"mtld_reliable"`
}

type TermCount struct {
	Term    string  `json:"term"`
	Count   int     `json:"count"`
	Per1000 float64 `json:"per_1000_words"`
}

type Repetition struct {
	RepeatedTerms        []TermCount `json:"repeated_terms"`
	RepeatedTrigramRatio float64     `json:"repeated_trigram_ratio"`
	RepeatedOpeners      []TermCount `json:"repeated_sentence_openers"`
}

type SentenceIssue struct {
	Index int    `json:"index"`
	Start int    `json:"start"`
	End   int    `json:"end"`
	Words int    `json:"words"`
	Text  string `json:"text"`
}

type SpanIssue struct {
	Text       string `json:"text"`
	Suggestion string `json:"suggestion,omitempty"`
	Start      int    `json:"start"`
	End        int    `json:"end"`
}

type Issues struct {
	LongSentences     []SentenceIssue `json:"long_sentences"`
	PassiveCandidates []SpanIssue     `json:"passive_voice_candidates"`
	WordyPhrases      []SpanIssue     `json:"wordy_or_complex_phrases"`
}

type AudienceFit struct {
	Key         string  `json:"key"`
	Label       string  `json:"label"`
	TargetGrade float64 `json:"target_grade"`
	MeetsTarget bool    `json:"meets_target"`
	GradeGap    float64 `json:"grade_gap"`
}

type Recommendation struct {
	Priority string `json:"priority"` // high | medium | low
	Message  string `json:"message"`
	Evidence string `json:"evidence"`
}

type Report struct {
	Counts          Counts           `json:"counts"`
	Readability     Readability      `json:"readability"`
	Lexical         Lexical          `json:"lexical"`
	Repetition      Repetition       `json:"repetition"`
	Issues          Issues           `json:"issues"`
	Audience        AudienceFit      `json:"audience"`
	Recommendations []Recommendation `json:"recommendations"`
	Language        string           `json:"language_assumed"`
	Disclaimer      string           `json:"disclaimer"`
}

const LongSentenceWords = 25

// Analyze computes the report. audience may be empty ("general").
func Analyze(text, audience string) Report {
	text = textutil.Normalize(text)
	if _, ok := Audiences[audience]; !ok {
		audience = "general"
	}
	toks := textutil.Tokens(text)
	sents := textutil.Sentences(text)
	words := make([]string, len(toks))
	for i, t := range toks {
		words[i] = t.Lower
	}

	r := Report{
		Language: "en",
		Disclaimer: "Readability formulas estimate surface difficulty from word and sentence length; " +
			"they do not measure accuracy, logic or tone. Metrics are calibrated for English. " +
			"No metric here indicates whether text was written by a person or by AI.",
	}
	c := &r.Counts
	c.Characters = textutil.RuneLen(text)
	for _, ch := range text {
		if unicode.IsLetter(ch) {
			c.Letters++
		}
	}
	c.Words = len(words)
	if c.Words == 0 {
		r.Recommendations = []Recommendation{}
		r.Issues = Issues{[]SentenceIssue{}, []SpanIssue{}, []SpanIssue{}}
		r.Repetition = Repetition{[]TermCount{}, 0, []TermCount{}}
		r.Readability.Notes = []string{"No words found."}
		return r
	}
	paras := 0
	for _, p := range strings.Split(text, "\n\n") {
		if strings.TrimSpace(p) != "" {
			paras++
		}
	}
	c.Paragraphs = paras

	// per-sentence word counts (by token offsets)
	sentWords := make([]int, len(sents))
	ti := 0
	for si, s := range sents {
		for ti < len(toks) && toks[ti].Start < s.End {
			if toks[ti].Start >= s.Start {
				sentWords[si]++
			}
			ti++
		}
	}
	nonEmpty := 0
	for _, n := range sentWords {
		if n > 0 {
			nonEmpty++
		}
	}
	c.Sentences = max(nonEmpty, 1)

	for _, w := range words {
		sy := textutil.Syllables(w)
		c.Syllables += sy
		if sy >= 3 && !textutil.IsNumeric(w) {
			c.ComplexWords++
		}
	}
	W, S := float64(c.Words), float64(c.Sentences)
	c.AvgWordsPerSent = W / S
	c.AvgSyllablesWord = float64(c.Syllables) / W
	c.ReadingTimeMinute = round(W/200, 1)
	var sum, sq float64
	for _, n := range sentWords {
		if n == 0 {
			continue
		}
		sum += float64(n)
		sq += float64(n * n)
		if n > c.LongestSentence {
			c.LongestSentence = n
		}
	}
	mean := sum / S
	c.SentenceLengthSD = round(math.Sqrt(math.Max(sq/S-mean*mean, 0)), 2)

	// readability (standard published coefficients)
	rd := &r.Readability
	rd.FleschReadingEase = round(206.835-1.015*(W/S)-84.6*(float64(c.Syllables)/W), 1)
	rd.FleschKincaid = round(0.39*(W/S)+11.8*(float64(c.Syllables)/W)-15.59, 1)
	rd.GunningFog = round(0.4*((W/S)+100*(float64(c.ComplexWords)/W)), 1)
	rd.SMOG = round(1.0430*math.Sqrt(float64(c.ComplexWords)*(30/S))+3.1291, 1)
	L := float64(c.Letters) / W * 100
	S100 := S / W * 100
	rd.ColemanLiau = round(0.0588*L-0.296*S100-15.8, 1)
	rd.ARI = round(4.71*(float64(c.Letters)/W)+0.5*(W/S)-21.43, 1)
	grades := []float64{rd.FleschKincaid, rd.GunningFog, rd.SMOG, rd.ColemanLiau, rd.ARI}
	sort.Float64s(grades)
	rd.ConsensusGrade = round(math.Max(grades[2], 0), 1) // median
	rd.EaseBand = easeBand(rd.FleschReadingEase)
	rd.Notes = []string{}
	if c.Sentences < 30 {
		rd.Notes = append(rd.Notes, "SMOG was validated on samples of 30+ sentences; treat it as indicative for shorter texts.")
	}
	if c.Words < 100 {
		rd.Notes = append(rd.Notes, "Fewer than 100 words: all readability scores have high variance.")
	}

	// lexical diversity
	lx := &r.Lexical
	lx.TTR = round(float64(uniq(words))/W, 3)
	lx.MATTRWindow = 50
	if len(words) < 50 {
		lx.MATTRWindow = len(words)
	}
	lx.MATTR = round(mattr(words, lx.MATTRWindow), 3)
	lx.MTLD = round((mtldPass(words)+mtldPass(reverse(words)))/2, 1)
	lx.MTLDReliable = len(words) >= 100

	r.Repetition = repetition(words, sents, W)
	r.Issues = issues(text, sents, sentWords)

	a := Audiences[audience]
	r.Audience = AudienceFit{
		Key: audience, Label: a.Label, TargetGrade: a.TargetGrade,
		MeetsTarget: rd.ConsensusGrade <= a.TargetGrade,
		GradeGap:    round(math.Max(rd.ConsensusGrade-a.TargetGrade, 0), 1),
	}
	r.Recommendations = recommend(r, a.MaxAvgWords)
	return r
}

func easeBand(f float64) string {
	switch {
	case f >= 90:
		return "very easy"
	case f >= 80:
		return "easy"
	case f >= 70:
		return "fairly easy"
	case f >= 60:
		return "plain English"
	case f >= 50:
		return "fairly difficult"
	case f >= 30:
		return "difficult"
	default:
		return "very difficult"
	}
}

func uniq(ws []string) int {
	m := make(map[string]struct{}, len(ws))
	for _, w := range ws {
		m[w] = struct{}{}
	}
	return len(m)
}

// mattr: moving-average type-token ratio (Covington & McFall, 2010).
func mattr(ws []string, win int) float64 {
	if win <= 0 || len(ws) == 0 {
		return 0
	}
	counts := map[string]int{}
	for _, w := range ws[:win] {
		counts[w]++
	}
	total := float64(len(counts)) / float64(win)
	n := 1
	for i := win; i < len(ws); i++ {
		out := ws[i-win]
		counts[out]--
		if counts[out] == 0 {
			delete(counts, out)
		}
		counts[ws[i]]++
		total += float64(len(counts)) / float64(win)
		n++
	}
	return total / float64(n)
}

// mtldPass: one directional pass of MTLD (McCarthy & Jarvis, 2010), TTR threshold 0.72.
func mtldPass(ws []string) float64 {
	const thr = 0.72
	factors := 0.0
	types := map[string]struct{}{}
	count := 0
	for _, w := range ws {
		count++
		types[w] = struct{}{}
		if float64(len(types))/float64(count) <= thr {
			factors++
			types = map[string]struct{}{}
			count = 0
		}
	}
	if count > 0 {
		ttr := float64(len(types)) / float64(count)
		factors += (1 - ttr) / (1 - thr)
	}
	if factors == 0 {
		return float64(len(ws))
	}
	return float64(len(ws)) / factors
}

func reverse(ws []string) []string {
	out := make([]string, len(ws))
	for i, w := range ws {
		out[len(ws)-1-i] = w
	}
	return out
}

func repetition(words []string, sents []textutil.Sentence, W float64) Repetition {
	rep := Repetition{RepeatedTerms: []TermCount{}, RepeatedOpeners: []TermCount{}}
	freq := map[string]int{}
	for _, w := range words {
		if !textutil.Stopwords[w] && !textutil.IsNumeric(w) && len(w) > 2 {
			freq[textutil.Stem(w)]++
		}
	}
	for t, n := range freq {
		if n >= 3 && float64(n)/W*1000 >= 15 {
			rep.RepeatedTerms = append(rep.RepeatedTerms, TermCount{t, n, round(float64(n)/W*1000, 1)})
		}
	}
	sort.Slice(rep.RepeatedTerms, func(i, j int) bool {
		if rep.RepeatedTerms[i].Count != rep.RepeatedTerms[j].Count {
			return rep.RepeatedTerms[i].Count > rep.RepeatedTerms[j].Count
		}
		return rep.RepeatedTerms[i].Term < rep.RepeatedTerms[j].Term
	})
	if len(rep.RepeatedTerms) > 10 {
		rep.RepeatedTerms = rep.RepeatedTerms[:10]
	}

	if len(words) >= 3 {
		tri := map[string]int{}
		total := 0
		for i := 0; i+2 < len(words); i++ {
			a, b, c := words[i], words[i+1], words[i+2]
			if textutil.Stopwords[a] && textutil.Stopwords[b] && textutil.Stopwords[c] {
				continue
			}
			tri[a+" "+b+" "+c]++
			total++
		}
		repeated := 0
		for _, n := range tri {
			if n > 1 {
				repeated += n
			}
		}
		if total > 0 {
			rep.RepeatedTrigramRatio = round(float64(repeated)/float64(total), 3)
		}
	}

	open := map[string]int{}
	for _, s := range sents {
		ws := textutil.Words(s.Text)
		if len(ws) > 0 {
			open[ws[0]]++
		}
	}
	for w, n := range open {
		if n >= 3 && float64(n) >= 0.2*float64(len(sents)) {
			rep.RepeatedOpeners = append(rep.RepeatedOpeners, TermCount{Term: w, Count: n})
		}
	}
	sort.Slice(rep.RepeatedOpeners, func(i, j int) bool { return rep.RepeatedOpeners[i].Count > rep.RepeatedOpeners[j].Count })
	return rep
}

var irregularParticiples = `known shown given taken made done seen written found held built sent told paid brought
thought kept left lost chosen driven eaten forgotten hidden spoken stolen broken frozen begun drawn grown thrown worn
torn sold bought caught taught set put cut won led fed met spent understood undertaken overseen withdrawn`

var rePassive = regexp.MustCompile(`(?i)\b(am|is|are|was|were|be|been|being)\s+(?:(?:not|also|often|usually|always|never|then|now|\w+ly)\s+)?([a-z]+ed|` +
	strings.Join(strings.Fields(irregularParticiples), "|") + `)\b`)

func issues(text string, sents []textutil.Sentence, sentWords []int) Issues {
	is := Issues{LongSentences: []SentenceIssue{}, PassiveCandidates: []SpanIssue{}, WordyPhrases: []SpanIssue{}}
	for i, s := range sents {
		if sentWords[i] > LongSentenceWords {
			is.LongSentences = append(is.LongSentences, SentenceIssue{i, s.Start, s.End, sentWords[i], s.Text})
		}
	}
	for _, m := range rePassive.FindAllStringSubmatchIndex(text, -1) {
		part := strings.ToLower(text[m[4]:m[5]])
		if strings.HasSuffix(part, "eed") || len(part) < 5 && strings.HasSuffix(part, "ed") {
			continue
		}
		is.PassiveCandidates = append(is.PassiveCandidates, SpanIssue{
			Text:  text[m[0]:m[1]],
			Start: textutil.RuneLen(text[:m[0]]), End: textutil.RuneLen(text[:m[1]]),
		})
	}
	res := plain.Simplify(text, plain.Options{Level: "standard"})
	for _, c := range append(res.Applied, res.Suggestions...) {
		is.WordyPhrases = append(is.WordyPhrases, SpanIssue{c.From, c.To, c.Start, c.End})
	}
	sort.Slice(is.WordyPhrases, func(i, j int) bool { return is.WordyPhrases[i].Start < is.WordyPhrases[j].Start })
	return is
}

func recommend(r Report, maxAvg float64) []Recommendation {
	out := []Recommendation{}
	add := func(p, m, e string) { out = append(out, Recommendation{p, m, e}) }
	rd, c := r.Readability, r.Counts
	if !r.Audience.MeetsTarget {
		p := "medium"
		if r.Audience.GradeGap >= 3 {
			p = "high"
		}
		add(p, "Lower the reading level for this audience: use shorter sentences and everyday words.",
			fmtf("Consensus grade %.1f vs. target %.0f for %s.", rd.ConsensusGrade, r.Audience.TargetGrade, strings.ToLower(r.Audience.Label)))
	}
	if c.AvgWordsPerSent > maxAvg {
		add("high", "Shorten sentences; aim for one main idea per sentence.",
			fmtf("Average sentence length %.1f words (target ≤ %.0f).", c.AvgWordsPerSent, maxAvg))
	}
	if n := len(r.Issues.LongSentences); n > 0 {
		add("medium", "Split the longest sentences.", fmtf("%d sentence(s) exceed %d words; longest has %d.", n, LongSentenceWords, c.LongestSentence))
	}
	if n := len(r.Issues.WordyPhrases); n > 0 {
		add("medium", "Replace wordy or uncommon expressions with plain alternatives (see Plain Language tab).", fmtf("%d phrase(s) found.", n))
	}
	if n := len(r.Issues.PassiveCandidates); n > 0 && float64(n)/float64(c.Sentences) > 0.25 {
		add("low", "Prefer active voice where the actor matters (who does what).",
			fmtf("%d passive-voice candidate(s) in %d sentences.", n, c.Sentences))
	}
	if c.Words > 0 && float64(c.ComplexWords)/float64(c.Words) > 0.15 {
		add("medium", "Reduce words with three or more syllables, or explain technical terms once.",
			fmtf("%.0f%% of words have 3+ syllables.", 100*float64(c.ComplexWords)/float64(c.Words)))
	}
	if len(r.Repetition.RepeatedOpeners) > 0 {
		o := r.Repetition.RepeatedOpeners[0]
		add("low", "Vary how sentences begin.", fmtf("%d sentences start with \"%s\".", o.Count, o.Term))
	}
	if r.Repetition.RepeatedTrigramRatio > 0.15 {
		add("low", "Several three-word sequences repeat; check for redundant statements.",
			fmtf("Repeated trigram ratio %.2f.", r.Repetition.RepeatedTrigramRatio))
	}
	if c.SentenceLengthSD < 3 && c.Sentences >= 5 {
		add("low", "Sentence lengths are very uniform; mixing short and longer sentences can improve rhythm.",
			fmtf("Sentence length SD %.1f words.", c.SentenceLengthSD))
	}
	return out
}

func round(x float64, d int) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return 0
	}
	p := math.Pow(10, float64(d))
	return math.Round(x*p) / p
}
