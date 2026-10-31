// Package evidence searches OpenAlex and arXiv, checks that a
// reference exists in a registry (OpenAlex, then Crossref), and
// downloads a page or a PDF safely and turns it into text (B-55,
// B-58, B-59, B-60, R-9). The readers' tools and the verification of
// findings use it; it never calls a model and stores nothing.
//
// Every request goes through one HTTP client of its own (B-60): only
// http and https; the IP is checked when the connection is made, so a
// name that resolves to a private, loopback or link-local address
// (169.254.169.254, *.internal) is refused, also after each of at
// most 3 redirects; 5 s and 5 MB (after decompression) per resource;
// no Meta key and no cookies; only text/html, application/pdf and
// text/plain are downloaded as text. The app never downloads a URL a
// model wrote (B-58): FetchText takes URLs that came from an API.
package evidence

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"
)

// Services. A reader's `where` is openalex or arxiv (B-55, P-97);
// crossref is only the registry fallback of Lookup (B-59).
const (
	ServiceOpenAlex = "openalex"
	ServiceArxiv    = "arxiv"
	ServiceCrossref = "crossref"
)

// Limits (B-60, B3-22 row 9).
const (
	MaxBytes     = 5 << 20         // per resource, measured after decompression
	Timeout      = 5 * time.Second // per resource, redirects included
	MaxRedirects = 3
	ArxivSpacing = 3 * time.Second // at most one arXiv request every 3 s
	MaxHits      = 10              // results per search (B-55)
)

// AllowedContentTypes are the only media types FetchText accepts
// (B-60).
var AllowedContentTypes = []string{"text/html", "application/pdf", "text/plain"}

// Default API base URLs. TQ_EVIDENCE_BASE_URLS replaces them in tests
// only.
var defaultBaseURLs = map[string]string{
	ServiceOpenAlex: "https://api.openalex.org",
	ServiceArxiv:    "https://export.arxiv.org",
	ServiceCrossref: "https://api.crossref.org",
}

// Errors. They carry no URL, query or page content (AGENTS.md rule 9).
var (
	// ErrNotFound: the record does not exist, or the page answered 404
	// or 410, or its domain does not resolve (B-59).
	ErrNotFound = errors.New("evidence: not found")
	// ErrBlocked: a scheme other than http/https, or an address that
	// is private, loopback, link-local or *.internal (B-60).
	ErrBlocked = errors.New("evidence: address not allowed")
	// ErrTooManyRedirects: more than MaxRedirects redirects.
	ErrTooManyRedirects = errors.New("evidence: too many redirects")
	// ErrTooLarge: more than MaxBytes after decompression.
	ErrTooLarge = errors.New("evidence: response too large")
	// ErrTimeout: the resource took more than Timeout.
	ErrTimeout = errors.New("evidence: response too slow")
	// ErrContentType: a media type outside AllowedContentTypes, or a
	// content encoding the client cannot measure after decompression.
	ErrContentType = errors.New("evidence: content type not allowed")
	// ErrUnavailable: any other failure (HTTP error, connection,
	// unreadable answer). It is not ErrNotFound: B-59 keeps it as
	// «no verificado».
	ErrUnavailable = errors.New("evidence: source unavailable")
	// ErrInvalidQuery: an empty query or a `where` other than openalex
	// or arxiv.
	ErrInvalidQuery = errors.New("evidence: invalid query")
)

// Query is one search_literature call (B-55).
type Query struct {
	Text     string
	Where    string // ServiceOpenAlex or ServiceArxiv
	YearFrom int    // 0: no lower bound
}

// Hit is one search result. URL and DOI come from the API, never from
// a model (B-58).
type Hit struct {
	Service  string // ServiceOpenAlex or ServiceArxiv
	ID       string // OpenAlex work id (W…) or arXiv id (with version)
	Title    string
	Authors  []string
	Year     int
	DOI      string // bare, lower case, without https://doi.org/
	URL      string // where the open text is, as the API gives it
	OpenText bool
	Abstract string
}

// Reference is a reference a model wrote (B-58): only its DOI and its
// title are used to look it up; it has no URL on purpose, because the
// app never downloads an address a model wrote.
type Reference struct {
	DOI   string
	Title string
}

// Record is a registry entry found by Lookup.
type Record struct {
	Service  string // ServiceOpenAlex or ServiceCrossref
	ID       string // OpenAlex work id, or the DOI for Crossref
	Title    string
	Authors  []string
	Year     int
	DOI      string
	URL      string
	OpenText bool
	Abstract string
}

// Text is a downloaded resource and its plain text. Raw is what was
// downloaded (after decompression): it is the source and its
// fingerprint (B-60); storing it is 3.3's.
type Text struct {
	URL         string // final URL, after redirects
	ContentType string // media type without parameters
	Text        string
	Raw         []byte
}

// Options configures a Client.
type Options struct {
	// ContactEmail goes in the User-Agent (TQ_CONTACT_EMAIL, B-60).
	ContactEmail string
	// BaseURLs replaces API base URLs by service (TQ_EVIDENCE_BASE_URLS,
	// tests only).
	BaseURLs map[string]string
}

// Client is the evidence connector. The app shares one Client, so the
// arXiv spacing holds across readers.
type Client struct {
	contact string
	base    map[string]string
	http    *http.Client

	// lookupIP and dialIP are the DNS and the connect step; tests
	// replace them so no test needs DNS or reaches the network. The
	// address check runs before dialIP, and the default dialIP checks
	// the socket address again in its Control function.
	lookupIP func(ctx context.Context, host string) ([]netip.Addr, error)
	dialIP   func(ctx context.Context, network, address string) (net.Conn, error)

	arxivMu   sync.Mutex
	arxivLast time.Time
}

// New builds a Client with its own HTTP client (B-60).
func New(opts Options) *Client {
	c := &Client{contact: strings.TrimSpace(opts.ContactEmail), base: map[string]string{}}
	for k, v := range defaultBaseURLs {
		c.base[k] = v
	}
	for k, v := range opts.BaseURLs {
		if _, ok := c.base[k]; ok && v != "" {
			c.base[k] = strings.TrimRight(v, "/")
		}
	}
	c.lookupIP = defaultLookup
	c.dialIP = defaultDial
	c.http = c.newHTTPClient()
	return c
}

func (c *Client) userAgent() string {
	if c.contact == "" {
		return "tupaqnin/0.3"
	}
	return "tupaqnin/0.3 (mailto:" + c.contact + ")"
}
