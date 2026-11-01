package plain

// Phrase is a wordy or complex expression with a plain-language alternative.
// The list is adapted from common plain-language guidance (for example the
// U.S. Federal Plain Language Guidelines' "simple words and phrases" list).
// Entries are restricted to substitutions that keep grammar intact in
// ordinary use; context-dependent ones are marked Safe=false and are only
// suggested, never applied automatically.
type Phrase struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Kind   string `json:"kind"` // "wordy" (multi-word filler) or "complex" (rarer word)
	Safe   bool   `json:"safe"` // eligible for automatic replacement
	Reason string `json:"reason,omitempty"`
}

// Phrases is ordered longest-first at init time so that longer matches win.
var Phrases = []Phrase{
	// wordy multi-word expressions
	{"in order to", "to", "wordy", true, ""},
	{"due to the fact that", "because", "wordy", true, ""},
	{"owing to the fact that", "because", "wordy", true, ""},
	{"in view of the fact that", "because", "wordy", true, ""},
	{"in spite of the fact that", "although", "wordy", true, ""},
	{"despite the fact that", "although", "wordy", true, ""},
	{"in the event that", "if", "wordy", true, ""},
	{"at this point in time", "now", "wordy", true, ""},
	{"at the present time", "now", "wordy", true, ""},
	{"in the near future", "soon", "wordy", true, ""},
	{"prior to", "before", "wordy", true, ""},
	{"subsequent to", "after", "wordy", true, ""},
	{"a large number of", "many", "wordy", true, ""},
	{"a majority of", "most", "wordy", true, ""},
	{"with regard to", "about", "wordy", true, ""},
	{"with respect to", "about", "wordy", true, ""},
	{"in relation to", "about", "wordy", true, ""},
	{"pertaining to", "about", "wordy", true, ""},
	{"for the purpose of", "for", "wordy", true, ""},
	{"is able to", "can", "wordy", true, ""},
	{"are able to", "can", "wordy", true, ""},
	{"has the ability to", "can", "wordy", true, ""},
	{"have the ability to", "can", "wordy", true, ""},
	{"in accordance with", "under", "wordy", false, "\"under\" fits rules and laws; for instructions \"following\" may fit better"},
	{"on a daily basis", "daily", "wordy", true, ""},
	{"on a regular basis", "regularly", "wordy", true, ""},
	{"each and every", "every", "wordy", true, ""},
	{"first and foremost", "first", "wordy", true, ""},
	{"until such time as", "until", "wordy", true, ""},
	{"in close proximity to", "near", "wordy", true, ""},
	{"in the absence of", "without", "wordy", true, ""},
	{"make a decision", "decide", "wordy", true, ""},
	{"make a determination", "decide", "wordy", true, ""},
	{"take into consideration", "consider", "wordy", true, ""},
	{"give consideration to", "consider", "wordy", true, ""},
	{"conduct an investigation", "investigate", "wordy", true, ""},
	{"has a tendency to", "tends to", "wordy", true, ""},
	{"with the exception of", "except", "wordy", true, ""},
	{"in a timely manner", "promptly", "wordy", true, ""},
	{"at all times", "always", "wordy", true, ""},
	{"in excess of", "more than", "wordy", true, ""},
	{"on the basis of", "based on", "wordy", true, ""},
	{"during the course of", "during", "wordy", true, ""},
	{"in the course of", "during", "wordy", true, ""},
	{"by means of", "by", "wordy", true, ""},
	{"it is important to note that", "note that", "wordy", false, "often removable entirely; review the sentence"},
	{"a number of", "some", "wordy", false, "\"some\" or \"several\" depending on quantity"},
	{"whether or not", "whether", "wordy", false, "only equivalent when \"or not\" adds nothing"},

	// complex single words
	{"utilize", "use", "complex", true, ""},
	{"utilizes", "uses", "complex", true, ""},
	{"utilized", "used", "complex", true, ""},
	{"utilizing", "using", "complex", true, ""},
	{"utilization", "use", "complex", true, ""},
	{"commence", "start", "complex", true, ""},
	{"commencement", "start", "complex", true, ""},
	{"commences", "starts", "complex", true, ""},
	{"commenced", "started", "complex", true, ""},
	{"terminate", "end", "complex", true, ""},
	{"terminated", "ended", "complex", true, ""},
	{"approximately", "about", "complex", true, ""},
	{"assist", "help", "complex", true, ""},
	{"assists", "helps", "complex", true, ""},
	{"assisted", "helped", "complex", true, ""},
	{"assistance", "help", "complex", true, ""},
	{"facilitate", "help", "complex", false, "may mean \"make possible\" rather than \"help\""},
	{"endeavor", "try", "complex", false, "noun use (\"an endeavor\") needs \"effort\" instead"},
	{"endeavour", "try", "complex", false, "noun use needs \"effort\" instead"},
	{"ascertain", "find out", "complex", true, ""},
	{"demonstrate", "show", "complex", true, ""},
	{"demonstrates", "shows", "complex", true, ""},
	{"demonstrated", "showed", "complex", true, ""},
	{"sufficient", "enough", "complex", true, ""},
	{"obtain", "get", "complex", true, ""},
	{"obtained", "got", "complex", false, "\"got\" can sound informal; \"received\" may fit"},
	{"purchase", "buy", "complex", false, "noun use (\"a purchase\") needs a different word"},
	{"regarding", "about", "complex", true, ""},
	{"numerous", "many", "complex", true, ""},
	{"remainder", "rest", "complex", true, ""},
	{"individuals", "people", "complex", true, ""},
	{"initiate", "start", "complex", true, ""},
	{"initiated", "started", "complex", true, ""},
	{"modify", "change", "complex", true, ""},
	{"modified", "changed", "complex", true, ""},
	{"notify", "tell", "complex", false, "\"notify X of Y\" becomes \"tell X about Y\""},
	{"expedite", "speed up", "complex", true, ""},
	{"ameliorate", "improve", "complex", true, ""},
	{"cognizant of", "aware of", "complex", true, ""},
	{"additionally", "also", "complex", true, ""},
	{"consequently", "so", "complex", false, "check sentence rhythm after replacing"},
	{"implement", "carry out", "complex", false, "technical sense (\"implement software\") differs"},
	{"requirement", "need", "complex", false, "legal/technical requirements should keep the term"},
	{"methodology", "method", "complex", false, "acceptable in research writing"},
}

func init() {
	// longest first so multi-word phrases take precedence
	for i := 1; i < len(Phrases); i++ {
		for j := i; j > 0 && len(Phrases[j].From) > len(Phrases[j-1].From); j-- {
			Phrases[j], Phrases[j-1] = Phrases[j-1], Phrases[j]
		}
	}
}
