package citation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Crossref resolves DOIs through the public Crossref REST API
// (https://api.crossref.org). Supplying a contact email routes requests to
// Crossref's "polite" pool, as Crossref recommends.
type Crossref struct {
	BaseURL string
	Mailto  string
	Client  *http.Client
}

var ErrNotFound = errors.New("DOI not found in Crossref")

var reValidDOI = regexp.MustCompile(`^10\.\d{4,9}/\S+$`)

// NormalizeDOI strips common prefixes and validates the shape of a DOI.
func NormalizeDOI(d string) (string, error) {
	d = strings.TrimSpace(d)
	for _, pre := range []string{"https://doi.org/", "http://doi.org/", "https://dx.doi.org/", "doi:", "DOI:"} {
		d = strings.TrimPrefix(d, pre)
	}
	d = strings.TrimSpace(d)
	if !reValidDOI.MatchString(d) {
		return "", fmt.Errorf("not a valid DOI: %q", d)
	}
	return d, nil
}

type crossrefMsg struct {
	Message struct {
		Type           string   `json:"type"`
		Title          []string `json:"title"`
		ContainerTitle []string `json:"container-title"`
		Publisher      string   `json:"publisher"`
		Volume         string   `json:"volume"`
		Issue          string   `json:"issue"`
		Page           string   `json:"page"`
		DOI            string   `json:"DOI"`
		URL            string   `json:"URL"`
		Author         []struct {
			Given  string `json:"given"`
			Family string `json:"family"`
			Name   string `json:"name"`
		} `json:"author"`
		Issued struct {
			DateParts [][]int `json:"date-parts"`
		} `json:"issued"`
	} `json:"message"`
}

// Lookup fetches metadata for a DOI.
func (c Crossref) Lookup(ctx context.Context, doi string) (Source, error) {
	d, err := NormalizeDOI(doi)
	if err != nil {
		return Source{}, err
	}
	base := c.BaseURL
	if base == "" {
		base = "https://api.crossref.org"
	}
	u := strings.TrimRight(base, "/") + "/works/" + url.PathEscape(d)
	if c.Mailto != "" {
		u += "?mailto=" + url.QueryEscape(c.Mailto)
	}
	cl := c.Client
	if cl == nil {
		cl = &http.Client{Timeout: 8 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return Source{}, err
	}
	ua := "HUMANIX/1.0 (+https://github.com/humanix-ai/humanix)"
	if c.Mailto != "" {
		ua += " mailto:" + c.Mailto
	}
	req.Header.Set("User-Agent", ua)
	resp, err := cl.Do(req)
	if err != nil {
		return Source{}, fmt.Errorf("crossref request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return Source{}, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return Source{}, fmt.Errorf("crossref returned HTTP %d", resp.StatusCode)
	}
	var m crossrefMsg
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&m); err != nil {
		return Source{}, fmt.Errorf("decode crossref response: %w", err)
	}
	msg := m.Message
	s := Source{
		Publisher: msg.Publisher, Volume: msg.Volume, Issue: msg.Issue,
		Pages: strings.ReplaceAll(msg.Page, "-", "–"), DOI: msg.DOI, URL: msg.URL, Verified: true,
	}
	switch msg.Type {
	case "journal-article", "proceedings-article", "posted-content":
		s.Type = "article"
	case "book", "monograph", "edited-book", "reference-book", "book-chapter":
		s.Type = "book"
	default:
		s.Type = "article"
	}
	if len(msg.Title) > 0 {
		s.Title = strings.Join(strings.Fields(msg.Title[0]), " ")
	}
	if len(msg.ContainerTitle) > 0 {
		s.Container = msg.ContainerTitle[0]
	}
	if len(msg.Issued.DateParts) > 0 && len(msg.Issued.DateParts[0]) > 0 && msg.Issued.DateParts[0][0] > 0 {
		s.Year = strconv.Itoa(msg.Issued.DateParts[0][0])
	}
	for _, a := range msg.Author {
		if a.Family == "" && a.Name != "" {
			s.Authors = append(s.Authors, Author{Family: a.Name})
		} else {
			s.Authors = append(s.Authors, Author{Given: a.Given, Family: a.Family})
		}
	}
	return s, nil
}
