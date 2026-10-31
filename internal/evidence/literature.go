package evidence

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// API media types. They are not FetchText's list: these answers are
// parsed as data, never shown as a source text.
var (
	jsonTypes = []string{"application/json"}
	atomTypes = []string{"application/atom+xml", "application/xml", "text/xml"}
)

// SearchLiterature runs one search_literature call (B-55) on OpenAlex
// (/works?search=) or arXiv (query) and returns at most MaxHits
// results. OpenAlex's abstract is rebuilt from its inverted index.
func (c *Client) SearchLiterature(ctx context.Context, q Query) ([]Hit, error) {
	text := strings.TrimSpace(q.Text)
	if text == "" {
		return nil, ErrInvalidQuery
	}
	switch q.Where {
	case ServiceOpenAlex:
		v := url.Values{"search": {text}, "per-page": {fmt.Sprint(MaxHits)}}
		if q.YearFrom > 0 {
			v.Set("filter", fmt.Sprintf("from_publication_date:%d-01-01", q.YearFrom))
		}
		res, err := c.get(ctx, c.base[ServiceOpenAlex]+"/works?"+v.Encode(), jsonTypes)
		if err != nil {
			return nil, err
		}
		var page struct {
			Results []openAlexWork `json:"results"`
		}
		if err := json.Unmarshal(res.body, &page); err != nil {
			return nil, ErrUnavailable
		}
		hits := make([]Hit, 0, len(page.Results))
		for _, w := range page.Results {
			hits = append(hits, w.hit())
		}
		return limit(hits), nil
	case ServiceArxiv:
		v := url.Values{"search_query": {"all:" + text}, "start": {"0"}, "max_results": {fmt.Sprint(MaxHits)}}
		res, err := c.get(ctx, c.base[ServiceArxiv]+"/api/query?"+v.Encode(), atomTypes)
		if err != nil {
			return nil, err
		}
		hits, err := parseArxiv(res.body)
		if err != nil {
			return nil, ErrUnavailable
		}
		if q.YearFrom > 0 {
			// [EL DISEÑO NO LO DICE] arXiv's year_from: filtered on the
			// publication year after the search, not in arXiv's query.
			kept := hits[:0]
			for _, h := range hits {
				if h.Year >= q.YearFrom {
					kept = append(kept, h)
				}
			}
			hits = kept
		}
		return limit(hits), nil
	default:
		return nil, ErrInvalidQuery
	}
}

func limit(hits []Hit) []Hit {
	if len(hits) > MaxHits {
		return hits[:MaxHits]
	}
	return hits
}

// Lookup checks that a reference exists (B-59): by DOI in OpenAlex
// and, when OpenAlex does not have it, in Crossref; without a DOI, by
// exact title in OpenAlex. It returns ErrNotFound when the record does
// not exist; any other failure is not ErrNotFound.
func (c *Client) Lookup(ctx context.Context, ref Reference) (Record, error) {
	if doi := normalizeDOI(ref.DOI); doi != "" {
		rec, err := c.lookupOpenAlexDOI(ctx, doi)
		if !errors.Is(err, ErrNotFound) {
			return rec, err
		}
		return c.lookupCrossrefDOI(ctx, doi)
	}
	if title := strings.TrimSpace(ref.Title); title != "" {
		return c.lookupTitle(ctx, title)
	}
	return Record{}, ErrNotFound
}

func (c *Client) lookupOpenAlexDOI(ctx context.Context, doi string) (Record, error) {
	res, err := c.get(ctx, c.base[ServiceOpenAlex]+"/works/doi:"+escapeDOI(doi), jsonTypes)
	if err != nil {
		return Record{}, err
	}
	var w openAlexWork
	if err := json.Unmarshal(res.body, &w); err != nil || w.ID == "" {
		return Record{}, ErrUnavailable
	}
	return w.record(), nil
}

func (c *Client) lookupCrossrefDOI(ctx context.Context, doi string) (Record, error) {
	res, err := c.get(ctx, c.base[ServiceCrossref]+"/works/"+escapeDOI(doi), jsonTypes)
	if err != nil {
		return Record{}, err
	}
	var page struct {
		Message struct {
			DOI    string   `json:"DOI"`
			URL    string   `json:"URL"`
			Title  []string `json:"title"`
			Author []struct {
				Given  string `json:"given"`
				Family string `json:"family"`
				Name   string `json:"name"`
			} `json:"author"`
			Issued struct {
				DateParts [][]int `json:"date-parts"`
			} `json:"issued"`
			Abstract string `json:"abstract"`
		} `json:"message"`
	}
	if err := json.Unmarshal(res.body, &page); err != nil || page.Message.DOI == "" {
		return Record{}, ErrUnavailable
	}
	m := page.Message
	rec := Record{Service: ServiceCrossref, DOI: normalizeDOI(m.DOI), URL: m.URL}
	rec.ID = rec.DOI
	if len(m.Title) > 0 {
		rec.Title = collapse(m.Title[0])
	}
	for _, a := range m.Author {
		name := collapse(a.Given + " " + a.Family)
		if name == "" {
			name = collapse(a.Name)
		}
		if name != "" {
			rec.Authors = append(rec.Authors, name)
		}
	}
	if len(m.Issued.DateParts) > 0 && len(m.Issued.DateParts[0]) > 0 {
		rec.Year = m.Issued.DateParts[0][0]
	}
	if m.Abstract != "" {
		// Crossref's abstract is JATS markup: its text only.
		rec.Abstract = collapse(htmlText([]byte(m.Abstract)))
	}
	return rec, nil
}

// lookupTitle searches OpenAlex by title and keeps only a result whose
// title is the same. [EL DISEÑO NO LO DICE] «título exacto»: equal
// after NFC, case folding and collapsed spaces; commas are dropped
// from the filter value because OpenAlex reads them as separators.
func (c *Client) lookupTitle(ctx context.Context, title string) (Record, error) {
	v := url.Values{"filter": {"title.search:" + strings.ReplaceAll(title, ",", " ")}, "per-page": {fmt.Sprint(MaxHits)}}
	res, err := c.get(ctx, c.base[ServiceOpenAlex]+"/works?"+v.Encode(), jsonTypes)
	if err != nil {
		return Record{}, err
	}
	var page struct {
		Results []openAlexWork `json:"results"`
	}
	if err := json.Unmarshal(res.body, &page); err != nil {
		return Record{}, ErrUnavailable
	}
	want := sameTitle(title)
	for _, w := range page.Results {
		if sameTitle(w.DisplayName) == want {
			return w.record(), nil
		}
	}
	return Record{}, ErrNotFound
}

func sameTitle(s string) string {
	return strings.ToLower(collapse(norm.NFC.String(s)))
}

// normalizeDOI returns a bare, lower-case DOI ("" if there is none).
func normalizeDOI(raw string) string {
	d := strings.TrimSpace(raw)
	lower := strings.ToLower(d)
	for _, p := range []string{"https://doi.org/", "http://doi.org/", "https://dx.doi.org/", "http://dx.doi.org/", "doi:"} {
		if strings.HasPrefix(lower, p) {
			d = d[len(p):]
			break
		}
	}
	d = strings.ToLower(strings.TrimSpace(d))
	// Only a DOI's own shape reaches the API path (verifier A2, B-58): a
	// registrant prefix, a slash, a suffix without spaces, query or
	// fragment, and no dot segments.
	if !doiShape.MatchString(d) {
		return ""
	}
	for _, seg := range strings.Split(d, "/") {
		if seg == "." || seg == ".." {
			return ""
		}
	}
	return d
}

var doiShape = regexp.MustCompile(`^10\.[0-9]{1,9}(\.[0-9]+)*/[^\s?#]+$`)

// escapeDOI escapes each path segment of a DOI and keeps its slashes.
func escapeDOI(doi string) string {
	parts := strings.Split(doi, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

// openAlexWork is one work as OpenAlex returns it.
type openAlexWork struct {
	ID              string `json:"id"`
	DisplayName     string `json:"display_name"`
	PublicationYear int    `json:"publication_year"`
	DOI             string `json:"doi"`
	OpenAccess      struct {
		IsOA  bool   `json:"is_oa"`
		OAURL string `json:"oa_url"`
	} `json:"open_access"`
	PrimaryLocation *struct {
		LandingPageURL string `json:"landing_page_url"`
	} `json:"primary_location"`
	Authorships []struct {
		Author struct {
			DisplayName string `json:"display_name"`
		} `json:"author"`
	} `json:"authorships"`
	Abstract map[string][]int `json:"abstract_inverted_index"`
}

func (w openAlexWork) record() Record {
	var authors []string
	for _, a := range w.Authorships {
		if name := collapse(a.Author.DisplayName); name != "" {
			authors = append(authors, name)
		}
	}
	doi := normalizeDOI(w.DOI)
	// [EL DISEÑO NO LO DICE] the hit's URL: the open-access URL when
	// there is one, else the landing page, else the DOI's address.
	link := w.OpenAccess.OAURL
	if link == "" && w.PrimaryLocation != nil {
		link = w.PrimaryLocation.LandingPageURL
	}
	if link == "" && doi != "" {
		link = "https://doi.org/" + doi
	}
	return Record{
		Service: ServiceOpenAlex, ID: strings.TrimPrefix(w.ID, "https://openalex.org/"),
		Title: collapse(w.DisplayName), Authors: authors, Year: w.PublicationYear, DOI: doi,
		URL: link, OpenText: w.OpenAccess.IsOA, Abstract: invertedAbstract(w.Abstract),
	}
}

func (w openAlexWork) hit() Hit {
	r := w.record()
	return Hit{Service: r.Service, ID: r.ID, Title: r.Title, Authors: r.Authors, Year: r.Year,
		DOI: r.DOI, URL: r.URL, OpenText: r.OpenText, Abstract: r.Abstract}
}

// invertedAbstract rebuilds an abstract from OpenAlex's inverted index
// (word → positions).
func invertedAbstract(index map[string][]int) string {
	type pos struct {
		at   int
		word string
	}
	var words []pos
	for w, positions := range index {
		for _, p := range positions {
			words = append(words, pos{p, w})
		}
	}
	sort.Slice(words, func(i, j int) bool {
		if words[i].at != words[j].at {
			return words[i].at < words[j].at
		}
		return words[i].word < words[j].word
	})
	parts := make([]string, 0, len(words))
	last := -1
	for _, w := range words {
		if w.at == last {
			continue // one word per position
		}
		parts = append(parts, w.word)
		last = w.at
	}
	return strings.Join(parts, " ")
}

// parseArxiv reads an arXiv Atom feed; every arXiv result is open.
func parseArxiv(body []byte) ([]Hit, error) {
	var feed struct {
		Entries []struct {
			ID        string `xml:"id"`
			Title     string `xml:"title"`
			Summary   string `xml:"summary"`
			Published string `xml:"published"`
			DOI       string `xml:"http://arxiv.org/schemas/atom doi"`
			Links     []struct {
				Href  string `xml:"href,attr"`
				Rel   string `xml:"rel,attr"`
				Title string `xml:"title,attr"`
			} `xml:"link"`
			Authors []struct {
				Name string `xml:"name"`
			} `xml:"author"`
		} `xml:"entry"`
	}
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, err
	}
	hits := make([]Hit, 0, len(feed.Entries))
	for _, e := range feed.Entries {
		h := Hit{Service: ServiceArxiv, Title: collapse(e.Title), DOI: normalizeDOI(e.DOI), OpenText: true,
			Abstract: collapse(e.Summary)}
		if i := strings.LastIndex(e.ID, "/abs/"); i >= 0 {
			h.ID = e.ID[i+len("/abs/"):]
		}
		for _, a := range e.Authors {
			if name := collapse(a.Name); name != "" {
				h.Authors = append(h.Authors, name)
			}
		}
		if len(e.Published) >= 4 {
			_, _ = fmt.Sscanf(e.Published[:4], "%d", &h.Year)
		}
		// The PDF when arXiv links one, else the abstract page.
		for _, l := range e.Links {
			if l.Title == "pdf" {
				h.URL = l.Href
			}
		}
		if h.URL == "" {
			for _, l := range e.Links {
				if l.Rel == "alternate" {
					h.URL = l.Href
				}
			}
		}
		hits = append(hits, h)
	}
	return hits, nil
}

func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }
