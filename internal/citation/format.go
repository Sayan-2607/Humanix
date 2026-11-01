package citation

import (
	"fmt"
	"html"
	"strings"
	"unicode"
)

type Author struct {
	Given  string `json:"given"`
	Family string `json:"family"`
}

// Source holds bibliographic metadata. Type: article | book | web.
type Source struct {
	Type      string   `json:"type"`
	Authors   []Author `json:"authors"`
	Title     string   `json:"title"`
	Container string   `json:"container"` // journal, website or book series
	Publisher string   `json:"publisher"`
	Year      string   `json:"year"`
	Volume    string   `json:"volume"`
	Issue     string   `json:"issue"`
	Pages     string   `json:"pages"`
	DOI       string   `json:"doi"`
	URL       string   `json:"url"`
	Accessed  string   `json:"accessed"` // e.g. "1 November 2026"
	Verified  bool     `json:"verified"` // true only when metadata came from Crossref
}

type Formatted struct {
	Style    string   `json:"style"`
	Text     string   `json:"text"` // italics marked with *asterisks*
	HTML     string   `json:"html"` // escaped, italics as <i>
	Warnings []string `json:"warnings"`
}

var Styles = []string{"apa", "mla", "ieee", "chicago"}

// Format renders src in the requested style.
func Format(src Source, style string) (Formatted, error) {
	style = strings.ToLower(strings.TrimSpace(style))
	src = clean(src)
	var parts []piece
	switch style {
	case "apa":
		parts = apa(src)
	case "mla":
		parts = mla(src)
	case "ieee":
		parts = ieee(src)
	case "chicago":
		parts = chicago(src)
	default:
		return Formatted{}, fmt.Errorf("unknown style %q (use apa, mla, ieee or chicago)", style)
	}
	f := Formatted{Style: style, Warnings: validate(src)}
	var t, h strings.Builder
	for _, p := range parts {
		if p.italic {
			t.WriteString("*" + p.s + "*")
			h.WriteString("<i>" + html.EscapeString(p.s) + "</i>")
		} else {
			t.WriteString(p.s)
			h.WriteString(html.EscapeString(p.s))
		}
	}
	f.Text = tidy(t.String())
	f.HTML = tidy(h.String())
	return f, nil
}

type piece struct {
	s      string
	italic bool
}

func p(s string) piece  { return piece{s, false} }
func it(s string) piece { return piece{s, true} }

func clean(s Source) Source {
	s.Type = strings.ToLower(strings.TrimSpace(s.Type))
	if s.Type == "" {
		if s.Container != "" && (s.Volume != "" || s.DOI != "") {
			s.Type = "article"
		} else if s.URL != "" && s.Publisher == "" {
			s.Type = "web"
		} else {
			s.Type = "book"
		}
	}
	s.Title = strings.TrimRight(strings.TrimSpace(s.Title), ".")
	s.DOI = strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(s.DOI), "https://doi.org/"), "doi:")
	var as []Author
	for _, a := range s.Authors {
		a.Given, a.Family = strings.TrimSpace(a.Given), strings.TrimSpace(a.Family)
		if a.Family != "" || a.Given != "" {
			as = append(as, a)
		}
	}
	s.Authors = as
	return s
}

func validate(s Source) []string {
	w := []string{}
	if s.Title == "" {
		w = append(w, "Missing title.")
	}
	if len(s.Authors) == 0 {
		w = append(w, "No authors given; formatted as an organisation/title-first reference.")
	}
	if s.Year == "" {
		w = append(w, "Missing year (shown as n.d.).")
	}
	if s.Type == "article" && s.Container == "" {
		w = append(w, "Missing journal name.")
	}
	if s.Type == "web" && s.URL == "" {
		w = append(w, "Web source without URL.")
	}
	if !s.Verified {
		w = append(w, "Metadata not verified against a registry; check it against the source.")
	}
	return w
}

func initials(given string) string {
	var out []string
	for _, part := range strings.Fields(given) {
		for _, sub := range strings.Split(part, "-") {
			r := []rune(strings.TrimRight(sub, "."))
			if len(r) > 0 {
				out = append(out, string(unicode.ToUpper(r[0]))+".")
			}
		}
	}
	return strings.Join(out, " ")
}

func year(s Source) string {
	if s.Year == "" {
		return "n.d."
	}
	return s.Year
}

func doiURL(d string) string { return "https://doi.org/" + d }

// ---------- APA 7 ----------
func apaAuthors(as []Author) string {
	names := make([]string, len(as))
	for i, a := range as {
		if a.Given == "" {
			names[i] = a.Family
		} else {
			names[i] = a.Family + ", " + initials(a.Given)
		}
	}
	switch n := len(names); {
	case n == 1:
		return names[0]
	case n == 2:
		return names[0] + ", & " + names[1]
	case n <= 20:
		return strings.Join(names[:n-1], ", ") + ", & " + names[n-1]
	default:
		return strings.Join(names[:19], ", ") + ", . . . " + names[n-1]
	}
}

func apa(s Source) []piece {
	var out []piece
	head := ""
	if len(s.Authors) > 0 {
		head = apaAuthors(s.Authors)
		if !strings.HasSuffix(head, ".") {
			head += "."
		}
		head += " (" + year(s) + "). "
		out = append(out, p(head))
	}
	switch s.Type {
	case "article":
		if len(s.Authors) == 0 {
			out = append(out, p(s.Title+". ("+year(s)+"). "))
		} else {
			out = append(out, p(s.Title+". "))
		}
		out = append(out, it(s.Container))
		if s.Volume != "" {
			out = append(out, p(", "), it(s.Volume))
			if s.Issue != "" {
				out = append(out, p("("+s.Issue+")"))
			}
		}
		if s.Pages != "" {
			out = append(out, p(", "+s.Pages))
		}
		out = append(out, p(". "))
	default: // book, web: title italic
		out = append(out, it(s.Title))
		if len(s.Authors) == 0 {
			out = append(out, p(". ("+year(s)+"). "))
		} else {
			out = append(out, p(". "))
		}
		if s.Type == "web" && s.Container != "" {
			out = append(out, p(s.Container+". "))
		} else if s.Publisher != "" {
			out = append(out, p(s.Publisher+". "))
		}
	}
	if s.DOI != "" {
		out = append(out, p(doiURL(s.DOI)))
	} else if s.URL != "" {
		out = append(out, p(s.URL))
	}
	return out
}

// ---------- MLA 9 ----------
func mlaAuthors(as []Author) string {
	full := func(a Author) string { return strings.TrimSpace(a.Given + " " + a.Family) }
	inv := func(a Author) string {
		if a.Given == "" {
			return a.Family
		}
		return a.Family + ", " + a.Given
	}
	switch len(as) {
	case 1:
		return inv(as[0])
	case 2:
		return inv(as[0]) + ", and " + full(as[1])
	default:
		return inv(as[0]) + ", et al"
	}
}

func mla(s Source) []piece {
	var out []piece
	if len(s.Authors) > 0 {
		a := mlaAuthors(s.Authors)
		if !strings.HasSuffix(a, ".") {
			a += "."
		}
		out = append(out, p(a+" "))
	}
	switch s.Type {
	case "article":
		out = append(out, p("“"+s.Title+".” "), it(s.Container))
		if s.Volume != "" {
			out = append(out, p(", vol. "+s.Volume))
		}
		if s.Issue != "" {
			out = append(out, p(", no. "+s.Issue))
		}
		out = append(out, p(", "+year(s)))
		if s.Pages != "" {
			out = append(out, p(", pp. "+s.Pages))
		}
		out = append(out, p(". "))
		if s.DOI != "" {
			out = append(out, p(doiURL(s.DOI)+"."))
		} else if s.URL != "" {
			out = append(out, p(s.URL+"."))
		}
	case "web":
		out = append(out, p("“"+s.Title+".” "))
		if s.Container != "" {
			out = append(out, it(s.Container), p(", "))
		}
		out = append(out, p(year(s)))
		if s.URL != "" {
			out = append(out, p(", "+s.URL))
		}
		out = append(out, p(". "))
		if s.Accessed != "" {
			out = append(out, p("Accessed "+s.Accessed+"."))
		}
	default:
		out = append(out, it(s.Title), p(". "))
		if s.Publisher != "" {
			out = append(out, p(s.Publisher+", "))
		}
		out = append(out, p(year(s)+"."))
	}
	return out
}

// ---------- IEEE ----------
func ieeeAuthors(as []Author) string {
	names := make([]string, len(as))
	for i, a := range as {
		names[i] = strings.TrimSpace(initials(a.Given) + " " + a.Family)
	}
	switch n := len(names); {
	case n == 1:
		return names[0]
	case n == 2:
		return names[0] + " and " + names[1]
	case n <= 6:
		return strings.Join(names[:n-1], ", ") + ", and " + names[n-1]
	default:
		return names[0] + " et al."
	}
}

func ieee(s Source) []piece {
	var out []piece
	if len(s.Authors) > 0 {
		out = append(out, p(ieeeAuthors(s.Authors)+", "))
	}
	switch s.Type {
	case "article":
		out = append(out, p("“"+s.Title+",” "), it(s.Container))
		if s.Volume != "" {
			out = append(out, p(", vol. "+s.Volume))
		}
		if s.Issue != "" {
			out = append(out, p(", no. "+s.Issue))
		}
		if s.Pages != "" {
			out = append(out, p(", pp. "+s.Pages))
		}
		out = append(out, p(", "+year(s)))
		if s.DOI != "" {
			out = append(out, p(", doi: "+s.DOI))
		}
		out = append(out, p("."))
	case "web":
		out = append(out, p("“"+s.Title+".” "))
		if s.Container != "" {
			out = append(out, p(s.Container+". "))
		}
		if s.Accessed != "" {
			out = append(out, p("Accessed: "+s.Accessed+". "))
		}
		if s.URL != "" {
			out = append(out, p("[Online]. Available: "+s.URL))
		}
	default:
		out = append(out, it(s.Title), p(". "))
		if s.Publisher != "" {
			out = append(out, p(s.Publisher+", "))
		}
		out = append(out, p(year(s)+"."))
	}
	return out
}

// ---------- Chicago (notes-bibliography, bibliography entry) ----------
func chicagoAuthors(as []Author) string {
	full := func(a Author) string { return strings.TrimSpace(a.Given + " " + a.Family) }
	first := as[0].Family
	if as[0].Given != "" {
		first += ", " + as[0].Given
	}
	switch n := len(as); {
	case n == 1:
		return first
	case n == 2:
		return first + ", and " + full(as[1])
	case n <= 10:
		names := []string{first}
		for _, a := range as[1 : n-1] {
			names = append(names, full(a))
		}
		return strings.Join(names, ", ") + ", and " + full(as[n-1])
	default:
		names := []string{first}
		for _, a := range as[1:7] {
			names = append(names, full(a))
		}
		return strings.Join(names, ", ") + ", et al"
	}
}

func chicago(s Source) []piece {
	var out []piece
	if len(s.Authors) > 0 {
		a := chicagoAuthors(s.Authors)
		if !strings.HasSuffix(a, ".") {
			a += "."
		}
		out = append(out, p(a+" "))
	}
	switch s.Type {
	case "article":
		out = append(out, p("“"+s.Title+".” "), it(s.Container))
		if s.Volume != "" {
			out = append(out, p(" "+s.Volume))
		}
		if s.Issue != "" {
			out = append(out, p(", no. "+s.Issue))
		}
		out = append(out, p(" ("+year(s)+")"))
		if s.Pages != "" {
			out = append(out, p(": "+s.Pages))
		}
		out = append(out, p(". "))
		if s.DOI != "" {
			out = append(out, p(doiURL(s.DOI)+"."))
		} else if s.URL != "" {
			out = append(out, p(s.URL+"."))
		}
	case "web":
		out = append(out, p("“"+s.Title+".” "))
		if s.Container != "" {
			out = append(out, p(s.Container+". "))
		}
		if s.Accessed != "" {
			out = append(out, p("Accessed "+s.Accessed+". "))
		}
		if s.URL != "" {
			out = append(out, p(s.URL+"."))
		}
	default:
		out = append(out, it(s.Title), p(". "))
		if s.Publisher != "" {
			out = append(out, p(s.Publisher+", "))
		}
		out = append(out, p(year(s)+"."))
	}
	return out
}

func tidy(s string) string {
	s = strings.TrimSpace(s)
	for _, r := range []struct{ a, b string }{{"..", "."}, {" ,", ","}, {"  ", " "}} {
		for strings.Contains(s, r.a) {
			s = strings.ReplaceAll(s, r.a, r.b)
		}
	}
	return s
}
