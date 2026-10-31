package evidence

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/pdfextract"
)

// No test reaches the network: every client in these tests resolves
// names with fakeNet.lookup and connects with fakeNet.dial, which only
// reaches the httptest servers registered for a public-looking IP and
// fails the test for anything else.

const testContact = "tester@tupaqnin.test"

// TestMain turns the test binary into the PDF extraction child when
// asked (B-9), and points pdfextract at it for the PDF test.
func TestMain(m *testing.M) {
	if os.Getenv("TQ_PDF_HELPER") == "child" {
		os.Exit(pdfextract.RunChild(os.Stdin, os.Stdout, os.Stderr))
	}
	pdfextract.ChildCommand = func(ctx context.Context) (*exec.Cmd, error) {
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^$")
		cmd.Env = append(os.Environ(), "TQ_PDF_HELPER=child")
		return cmd, nil
	}
	os.Exit(m.Run())
}

// seen is one request a test server received.
type seen struct {
	at     time.Time
	host   string
	target string
	header http.Header
}

// fakeNet stands in for DNS and for the network.
type fakeNet struct {
	t       *testing.T
	mu      sync.Mutex
	hosts   map[string][]netip.Addr  // name → addresses
	rebind  map[string][2]netip.Addr // name → first answer, later answers
	asked   map[string]int           // lookups per name
	servers map[netip.Addr]*httptest.Server
	dials   []string
	reqs    []seen
}

func newFakeNet(t *testing.T) *fakeNet {
	return &fakeNet{t: t, hosts: map[string][]netip.Addr{}, rebind: map[string][2]netip.Addr{},
		asked: map[string]int{}, servers: map[netip.Addr]*httptest.Server{}}
}

// serve registers a handler for host, reachable at a public-looking IP.
func (f *fakeNet) serve(host, ip string, h http.HandlerFunc) {
	addr := netip.MustParseAddr(ip)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.reqs = append(f.reqs, seen{at: time.Now(), host: r.Host, target: r.URL.RequestURI(), header: r.Header.Clone()})
		f.mu.Unlock()
		h(w, r)
	}))
	f.t.Cleanup(srv.Close)
	f.mu.Lock()
	f.hosts[host] = []netip.Addr{addr}
	f.servers[addr] = srv
	f.mu.Unlock()
}

func (f *fakeNet) lookup(_ context.Context, host string) ([]netip.Addr, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked[host]++
	if pair, ok := f.rebind[host]; ok {
		if f.asked[host] == 1 {
			return []netip.Addr{pair[0]}, nil
		}
		return []netip.Addr{pair[1]}, nil
	}
	if addrs, ok := f.hosts[host]; ok {
		return addrs, nil
	}
	return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
}

func (f *fakeNet) dial(ctx context.Context, network, address string) (net.Conn, error) {
	ap, err := netip.ParseAddrPort(address)
	if err != nil {
		f.t.Errorf("dial %q: not an IP address: the guard must dial resolved IPs only", address)
		return nil, err
	}
	f.mu.Lock()
	f.dials = append(f.dials, address)
	srv := f.servers[ap.Addr()]
	f.mu.Unlock()
	if srv == nil {
		f.t.Errorf("dial %s: no test server there (a test must never reach the network)", address)
		return nil, errors.New("fakeNet: unknown address")
	}
	var d net.Dialer
	return d.DialContext(ctx, network, srv.Listener.Addr().String())
}

func (f *fakeNet) requests() []seen {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]seen(nil), f.reqs...)
}

func (f *fakeNet) dialCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.dials)
}

// recording is one API answer in the 3.0 format.
type recording struct {
	Service     string `json:"service"`
	Query       string `json:"query"`
	URL         string `json:"url"`
	Status      int    `json:"status"`
	ContentType string `json:"content_type"`
	Body        string `json:"body"`
}

func loadRecordings(t *testing.T, service string) map[string]recording {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("testdata", "recordings", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("recordings: %v (%d files)", err, len(files))
	}
	out := map[string]recording{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var rec recording
		if err := json.Unmarshal(b, &rec); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if rec.Service != service {
			continue
		}
		u, err := url.Parse(rec.URL)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		out[u.EscapedPath()+"?"+u.RawQuery] = rec
	}
	return out
}

// serveRecordings answers with the recording whose path and query
// match the request; anything else fails the test.
func (f *fakeNet) serveRecordings(service, host, ip string) {
	recs := loadRecordings(f.t, service)
	f.serve(host, ip, func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.EscapedPath() + "?" + r.URL.RawQuery
		rec, ok := recs[key]
		if !ok {
			f.t.Errorf("%s: no recording for %s", service, key)
			http.Error(w, "no recording", http.StatusTeapot)
			return
		}
		w.Header().Set("Content-Type", rec.ContentType)
		w.WriteHeader(rec.Status)
		_, _ = w.Write([]byte(rec.Body))
	})
}

// apiNet serves the three APIs from their recordings.
func apiNet(t *testing.T) (*fakeNet, *Client) {
	f := newFakeNet(t)
	f.serveRecordings("openalex", "api.openalex.test", "93.184.216.10")
	f.serveRecordings("arxiv", "export.arxiv.test", "93.184.216.11")
	f.serveRecordings("crossref", "api.crossref.test", "93.184.216.12")
	return f, testClient(f)
}

func testClient(f *fakeNet) *Client {
	c := New(Options{ContactEmail: testContact, BaseURLs: map[string]string{
		ServiceOpenAlex: "http://api.openalex.test",
		ServiceArxiv:    "http://export.arxiv.test",
		ServiceCrossref: "http://api.crossref.test",
	}})
	c.lookupIP = f.lookup
	c.dialIP = f.dial
	return c
}

// --- B-55: search ---

func TestB55_OpenAlexSearchRebuildsAbstract(t *testing.T) {
	t.Parallel()
	f, c := apiNet(t)
	hits, err := c.SearchLiterature(context.Background(), Query{Text: "microplastics drinking water detection minimum particle size", Where: ServiceOpenAlex})
	if err != nil {
		t.Fatalf("SearchLiterature: %v", err)
	}
	if len(hits) != 10 {
		t.Fatalf("hits: got %d, want 10", len(hits))
	}
	h := hits[0]
	if h.Title != "Microplastics in drinking-water" || h.Year != 2019 || h.DOI != "10.60810/openumwelt-3831" ||
		!h.OpenText || h.URL != "https://edepot.wur.nl/498693" || h.Service != ServiceOpenAlex || h.ID != "W2972951570" {
		t.Errorf("hit 1: got %+v", h)
	}
	if len(h.Authors) < 3 || h.Authors[0] != "P. Marsden" || h.Authors[1] != "Koelmans, Bart" {
		t.Errorf("hit 1 authors: got %v", h.Authors)
	}
	wantStart := "Studies reporting the presence of microplastics in treated tap and bottled water have raised questions"
	if !strings.HasPrefix(h.Abstract, wantStart) {
		t.Errorf("hit 1 abstract not rebuilt in order: got %.120q", h.Abstract)
	}
	if !strings.HasPrefix(hits[1].Abstract, "BACKGROUND: Microplastics (MPs) are omnipresent in the environment,") {
		t.Errorf("hit 2 abstract: got %.120q", hits[1].Abstract)
	}
	reqs := f.requests()
	if len(reqs) != 1 {
		t.Fatalf("requests: got %d, want 1", len(reqs))
	}
	if ua := reqs[0].header.Get("User-Agent"); !strings.Contains(ua, "mailto:"+testContact) {
		t.Errorf("User-Agent %q lacks the contact (B-60)", ua)
	}
	if strings.Contains(reqs[0].target, testContact) {
		t.Errorf("the contact leaked into the URL: %s", reqs[0].target)
	}
}

func TestB55_OpenAlexSearchYearFrom(t *testing.T) {
	t.Parallel()
	_, c := apiNet(t)
	hits, err := c.SearchLiterature(context.Background(), Query{
		Text: "microplastics drinking water detection methods FTIR Raman minimum size", Where: ServiceOpenAlex, YearFrom: 2018})
	if err != nil {
		t.Fatalf("SearchLiterature: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("no hits")
	}
	for _, h := range hits {
		if h.Year != 0 && h.Year < 2018 {
			t.Errorf("hit %q from %d, before year_from 2018", h.Title, h.Year)
		}
	}
}

func TestB55_ArxivSearchParsesAtom(t *testing.T) {
	t.Parallel()
	_, c := apiNet(t)
	hits, err := c.SearchLiterature(context.Background(), Query{Text: "poverty brain development", Where: ServiceArxiv})
	if err != nil {
		t.Fatalf("SearchLiterature: %v", err)
	}
	if len(hits) != 10 {
		t.Fatalf("hits: got %d, want 10", len(hits))
	}
	h := hits[0]
	if h.Title != "Approximate invariance of metabolic energy per synapse during development in mammalian brains" ||
		h.Year != 2012 || h.DOI != "10.1371/journal.pone.0033425" || !h.OpenText ||
		h.URL != "https://arxiv.org/pdf/1204.3928v1" || h.Service != ServiceArxiv || h.ID != "1204.3928v1" {
		t.Errorf("hit 1: got %+v", h)
	}
	if len(h.Authors) != 1 || h.Authors[0] != "Jan Karbowski" {
		t.Errorf("hit 1 authors: got %v", h.Authors)
	}
	if !strings.HasPrefix(h.Abstract, "During mammalian development the cerebral metabolic rate correlates") {
		t.Errorf("hit 1 abstract: got %.100q", h.Abstract)
	}
	if strings.Contains(hits[1].Title, "\n") || strings.Contains(hits[1].Title, "  ") {
		t.Errorf("hit 2 title not collapsed: %q", hits[1].Title)
	}
}

func TestB55_SearchServesOnlyOpenAlexAndArxiv(t *testing.T) {
	t.Parallel()
	f, c := apiNet(t)
	for _, where := range []string{"web", "project", "crossref", ""} {
		_, err := c.SearchLiterature(context.Background(), Query{Text: "microplastics", Where: where})
		if !errors.Is(err, ErrInvalidQuery) {
			t.Errorf("where %q: got %v, want ErrInvalidQuery", where, err)
		}
	}
	if _, err := c.SearchLiterature(context.Background(), Query{Text: "  ", Where: ServiceOpenAlex}); !errors.Is(err, ErrInvalidQuery) {
		t.Errorf("empty text: got %v, want ErrInvalidQuery", err)
	}
	if n := len(f.requests()); n != 0 {
		t.Errorf("an invalid query sent %d requests", n)
	}
}

// --- B-59: registry lookup ---

func TestB59_LookupMissingDOIIsNotFound(t *testing.T) {
	t.Parallel()
	f, c := apiNet(t)
	_, err := c.Lookup(context.Background(), Reference{DOI: "10.9999/does-not-exist"})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Lookup: got %v, want ErrNotFound", err)
	}
	var hosts []string
	for _, r := range f.requests() {
		hosts = append(hosts, r.host)
	}
	if strings.Join(hosts, ",") != "api.openalex.test,api.crossref.test" {
		t.Errorf("lookup order: got %v, want OpenAlex then Crossref", hosts)
	}
}

func TestB59_LookupRealDOIReturnsRecord(t *testing.T) {
	t.Parallel()
	_, c := apiNet(t)
	for _, doi := range []string{"10.1371/journal.pone.0236838", "https://doi.org/10.1371/JOURNAL.PONE.0236838", "doi:10.1371/journal.pone.0236838"} {
		rec, err := c.Lookup(context.Background(), Reference{DOI: doi})
		if err != nil {
			t.Fatalf("Lookup(%q): %v", doi, err)
		}
		if rec.Service != ServiceOpenAlex || rec.DOI != "10.1371/journal.pone.0236838" || rec.Year != 2020 ||
			rec.ID != "W3045011717" || !rec.OpenText || rec.Authors[0] != "Evangelos Danopoulos" ||
			!strings.HasPrefix(rec.Abstract, "BACKGROUND: Microplastics (MPs)") {
			t.Errorf("Lookup(%q): got %+v", doi, rec)
		}
	}
}

func TestB59_LookupFallsBackToCrossref(t *testing.T) {
	t.Parallel()
	_, c := apiNet(t)
	rec, err := c.Lookup(context.Background(), Reference{DOI: "10.5555/crossref-only"})
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	want := Record{Service: ServiceCrossref, ID: "10.5555/crossref-only", Title: "A report only Crossref knows",
		Authors: []string{"Ana Quispe", "Luis Mamani"}, Year: 2021, DOI: "10.5555/crossref-only",
		URL: "https://doi.org/10.5555/crossref-only", Abstract: "Bottled water held more particles than tap water."}
	if fmt.Sprintf("%+v", rec) != fmt.Sprintf("%+v", want) {
		t.Errorf("Lookup:\n got %+v\nwant %+v", rec, want)
	}
}

func TestB59_LookupExactTitle(t *testing.T) {
	t.Parallel()
	_, c := apiNet(t)
	rec, err := c.Lookup(context.Background(), Reference{Title: "Microplastics in drinking-water"})
	if err != nil {
		t.Fatalf("Lookup(exact title): %v", err)
	}
	if rec.Title != "Microplastics in drinking-water" || rec.DOI != "10.60810/openumwelt-3831" {
		t.Errorf("Lookup(exact title): got %+v", rec)
	}
	if _, err := c.Lookup(context.Background(), Reference{Title: "Microplastics in drinking water and air"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("Lookup(no exact title): got %v, want ErrNotFound", err)
	}
	if _, err := c.Lookup(context.Background(), Reference{}); !errors.Is(err, ErrNotFound) {
		t.Errorf("Lookup(empty): got %v, want ErrNotFound", err)
	}
}

// --- B-60: safe download ---

func pageNet(t *testing.T) (*fakeNet, *Client) {
	f := newFakeNet(t)
	return f, testClient(f)
}

func TestB60_PrivateAddressesRejected(t *testing.T) {
	t.Parallel()
	f, c := pageNet(t)
	f.hosts["evil.example"] = []netip.Addr{netip.MustParseAddr("10.1.2.3")}
	f.hosts["mixed.example"] = []netip.Addr{netip.MustParseAddr("93.184.216.99"), netip.MustParseAddr("127.0.0.1")}
	f.hosts["mapped.example"] = []netip.Addr{netip.MustParseAddr("::ffff:169.254.169.254")}
	for _, u := range []string{
		"http://127.0.0.1/", "http://10.0.0.5/", "http://169.254.169.254/latest/meta-data/",
		"http://my-app.internal/", "http://top1.nearest.of.my-app.internal:8080/", "http://localhost/",
		"http://[::1]/", "http://[fdaa::3]/", "http://0.0.0.0/", "http://100.64.1.1/",
		"http://evil.example/", "http://mixed.example/", "http://mapped.example/",
	} {
		_, err := c.FetchText(context.Background(), u)
		if !errors.Is(err, ErrBlocked) {
			t.Errorf("FetchText(%s): got %v, want ErrBlocked", u, err)
		}
	}
	if n := f.dialCount(); n != 0 {
		t.Errorf("blocked addresses were dialed %d times", n)
	}
}

func TestB60_PrivateAddressesRejectedAfterRedirect(t *testing.T) {
	t.Parallel()
	f, c := pageNet(t)
	f.hosts["evil.example"] = []netip.Addr{netip.MustParseAddr("10.1.2.3")}
	f.serve("pages.example", "93.184.216.20", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, r.URL.Query().Get("to"), http.StatusFound)
	})
	for _, to := range []string{"http://127.0.0.1/", "http://10.0.0.5/", "http://169.254.169.254/latest/meta-data/",
		"http://metadata.internal/", "http://evil.example/", "file:///etc/passwd"} {
		_, err := c.FetchText(context.Background(), "http://pages.example/go?to="+url.QueryEscape(to))
		if !errors.Is(err, ErrBlocked) {
			t.Errorf("redirect to %s: got %v, want ErrBlocked", to, err)
		}
	}
	f.mu.Lock()
	dials := append([]string(nil), f.dials...)
	f.mu.Unlock()
	for _, d := range dials {
		if !strings.HasPrefix(d, "93.184.216.20:") {
			t.Errorf("dialed %s after a redirect", d)
		}
	}
}

func TestB60_OnlyHTTPAndHTTPS(t *testing.T) {
	t.Parallel()
	f, c := pageNet(t)
	for _, u := range []string{"file:///etc/passwd", "ftp://pages.example/x", "gopher://pages.example/", "javascript:alert(1)", "pages.example/x", ""} {
		if _, err := c.FetchText(context.Background(), u); !errors.Is(err, ErrBlocked) {
			t.Errorf("FetchText(%q): got %v, want ErrBlocked", u, err)
		}
	}
	if n := f.dialCount(); n != 0 {
		t.Errorf("dialed %d times", n)
	}
}

func TestB60_AtMostThreeRedirects(t *testing.T) {
	t.Parallel()
	f, c := pageNet(t)
	f.serve("pages.example", "93.184.216.21", func(w http.ResponseWriter, r *http.Request) {
		var n int
		_, _ = fmt.Sscanf(r.URL.Query().Get("n"), "%d", &n)
		if n > 0 {
			http.Redirect(w, r, fmt.Sprintf("/hop?n=%d", n-1), http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("arrived"))
	})
	text, err := c.FetchText(context.Background(), "http://pages.example/hop?n=3")
	if err != nil || text.Text != "arrived" || text.URL != "http://pages.example/hop?n=0" {
		t.Errorf("3 redirects: got %+v, %v; want the text after them", text, err)
	}
	if _, err := c.FetchText(context.Background(), "http://pages.example/hop?n=4"); !errors.Is(err, ErrTooManyRedirects) {
		t.Errorf("4 redirects: got %v, want ErrTooManyRedirects", err)
	}
}

func TestB60_RebindingRejectedAtDial(t *testing.T) {
	t.Parallel()
	f, c := pageNet(t)
	f.serve("public.example", "93.184.216.22", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("must not be read"))
	})
	f.rebind["rebind.example"] = [2]netip.Addr{netip.MustParseAddr("93.184.216.22"), netip.MustParseAddr("127.0.0.1")}
	_, err := c.FetchText(context.Background(), "http://rebind.example/")
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("FetchText: got %v, want ErrBlocked", err)
	}
	f.mu.Lock()
	asked := f.asked["rebind.example"]
	f.mu.Unlock()
	if asked < 2 {
		t.Errorf("the name was resolved %d times: the dial must resolve again and check", asked)
	}
	if n := f.dialCount(); n != 0 {
		t.Errorf("dialed %d times: the IP must be checked before connecting", n)
	}
}

func TestB60_DialControlRejectsPrivateIP(t *testing.T) {
	t.Parallel()
	for _, addr := range []string{"127.0.0.1:80", "10.0.0.5:443", "169.254.169.254:80", "[::1]:80", "[fdaa::2]:80",
		"[::ffff:127.0.0.1]:80", "192.168.1.1:80", "172.16.0.1:80", "0.0.0.0:80", "224.0.0.1:80"} {
		if err := dialControl("tcp", addr, nil); !errors.Is(err, ErrBlocked) {
			t.Errorf("dialControl(%s): got %v, want ErrBlocked", addr, err)
		}
	}
	for _, addr := range []string{"93.184.216.34:443", "[2606:4700::6810:84e5]:443"} {
		if err := dialControl("tcp", addr, nil); err != nil {
			t.Errorf("dialControl(%s): got %v, want nil", addr, err)
		}
	}
	// The default dialer carries the control: a real connection to a
	// loopback listener is refused before it is made.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	c := New(Options{})
	if _, err := c.dialIP(context.Background(), "tcp", ln.Addr().String()); !errors.Is(err, ErrBlocked) {
		t.Errorf("default dial to %s: got %v, want ErrBlocked", ln.Addr(), err)
	}
}

func TestB60_Limits(t *testing.T) {
	t.Parallel()
	if MaxBytes != 5<<20 || Timeout != 5*time.Second || MaxRedirects != 3 || ArxivSpacing != 3*time.Second {
		t.Errorf("limits: got %d bytes, %v, %d redirects, arXiv %v; want 5 MB, 5 s, 3, 3 s", MaxBytes, Timeout, MaxRedirects, ArxivSpacing)
	}
	if got := strings.Join(AllowedContentTypes, ","); got != "text/html,application/pdf,text/plain" {
		t.Errorf("allowed content types: %s", got)
	}
}

func TestB60_ResponseOver5MBIsCut(t *testing.T) {
	t.Parallel()
	f, c := pageNet(t)
	f.serve("big.example", "93.184.216.23", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		size := MaxBytes + 1
		if r.URL.Path == "/exact" {
			size = MaxBytes
		}
		_, _ = w.Write(bytes.Repeat([]byte("a"), size))
	})
	if _, err := c.FetchText(context.Background(), "http://big.example/over"); !errors.Is(err, ErrTooLarge) {
		t.Errorf("5 MB + 1: got %v, want ErrTooLarge", err)
	}
	text, err := c.FetchText(context.Background(), "http://big.example/exact")
	if err != nil || len(text.Text) != MaxBytes {
		t.Errorf("exactly 5 MB: got %d bytes, %v", len(text.Text), err)
	}
}

func TestB60_CompressedResponseOver5MBAfterDecompressionIsCut(t *testing.T) {
	t.Parallel()
	f, c := pageNet(t)
	f.serve("gzip.example", "93.184.216.24", func(w http.ResponseWriter, r *http.Request) {
		size := 6 << 20
		if r.URL.Path == "/small" {
			size = 1 << 20
		}
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		_, _ = zw.Write(bytes.Repeat([]byte("a"), size))
		_ = zw.Close()
		if buf.Len() > 100<<10 {
			t.Errorf("compressed body is %d bytes: the test needs it far below 5 MB", buf.Len())
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write(buf.Bytes())
	})
	if _, err := c.FetchText(context.Background(), "http://gzip.example/big"); !errors.Is(err, ErrTooLarge) {
		t.Errorf("gzip of 6 MB: got %v, want ErrTooLarge", err)
	}
	text, err := c.FetchText(context.Background(), "http://gzip.example/small")
	if err != nil || len(text.Text) != 1<<20 {
		t.Errorf("gzip of 1 MB: got %d bytes, %v; want the decompressed 1 MB", len(text.Text), err)
	}
}

func TestB60_ResponseOver5SecondsIsCut(t *testing.T) {
	t.Parallel()
	f, c := pageNet(t)
	f.serve("slow.example", "93.184.216.25", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("first bytes, then nothing"))
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-time.After(20 * time.Second):
		}
	})
	start := time.Now()
	_, err := c.FetchText(context.Background(), "http://slow.example/")
	elapsed := time.Since(start)
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("slow body: got %v, want ErrTimeout", err)
	}
	if elapsed < Timeout-100*time.Millisecond || elapsed > Timeout+2*time.Second {
		t.Errorf("cut after %v, want about %v", elapsed, Timeout)
	}
}

func TestB60_ContentTypeOutsideListRejected(t *testing.T) {
	t.Parallel()
	f, c := pageNet(t)
	f.serve("types.example", "93.184.216.26", func(w http.ResponseWriter, r *http.Request) {
		ct := r.URL.Query().Get("ct")
		if ct != "-" {
			w.Header().Set("Content-Type", ct)
		} else {
			w.Header()["Content-Type"] = nil
		}
		_, _ = w.Write([]byte("plain words"))
	})
	for _, ct := range []string{"application/json", "image/svg+xml", "application/octet-stream", "text/javascript", "application/xhtml+xml", "-"} {
		if _, err := c.FetchText(context.Background(), "http://types.example/?ct="+url.QueryEscape(ct)); !errors.Is(err, ErrContentType) {
			t.Errorf("Content-Type %q: got %v, want ErrContentType", ct, err)
		}
	}
	text, err := c.FetchText(context.Background(), "http://types.example/?ct="+url.QueryEscape("text/plain; charset=utf-8"))
	if err != nil || text.Text != "plain words" || text.ContentType != "text/plain" || string(text.Raw) != "plain words" {
		t.Errorf("text/plain: got %+v, %v", text, err)
	}
}

func TestB60_HTMLWithoutScriptsOrStyles(t *testing.T) {
	t.Parallel()
	f := newFakeNet(t)
	f.serveRecordings("web", "pages.example", "93.184.216.27")
	c := testClient(f)
	// The recording's URL is https; the test server speaks http.
	text, err := c.FetchText(context.Background(), "http://pages.example/report")
	if err != nil {
		t.Fatalf("FetchText: %v", err)
	}
	want := "Microplastics report\nParticles smaller than 20 µm were not counted.\nBottled water held more particles than tap water."
	if text.Text != want {
		t.Errorf("text:\n got %q\nwant %q", text.Text, want)
	}
	for _, bad := range []string{"alert", "document.cookie", "color: red", "evil.example", "Enable scripts", "Report"} {
		if strings.Contains(text.Text, bad) {
			t.Errorf("text keeps %q", bad)
		}
	}
	if text.ContentType != "text/html" || !bytes.Contains(text.Raw, []byte("<script")) {
		t.Errorf("raw page not kept as downloaded (it is the source, B-60): type %q", text.ContentType)
	}
}

func TestB60_PDFTextThroughChildProcess(t *testing.T) {
	t.Parallel()
	f, c := pageNet(t)
	pdf := minimalPDF("Bottled water held more particles")
	f.serve("pdf.example", "93.184.216.28", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write(pdf)
	})
	text, err := c.FetchText(context.Background(), "http://pdf.example/paper.pdf")
	if err != nil {
		t.Fatalf("FetchText: %v", err)
	}
	if !strings.Contains(text.Text, "Bottled water held more particles") || text.ContentType != "application/pdf" || !bytes.Equal(text.Raw, pdf) {
		t.Errorf("pdf: got text %q type %q", text.Text, text.ContentType)
	}
}

func TestB59_FetchMissingPageIsNotFound(t *testing.T) {
	t.Parallel()
	f, c := pageNet(t)
	f.serve("gone.example", "93.184.216.29", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/404":
			http.NotFound(w, r)
		case "/410":
			w.WriteHeader(http.StatusGone)
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	})
	for _, p := range []string{"/404", "/410"} {
		if _, err := c.FetchText(context.Background(), "http://gone.example"+p); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: got %v, want ErrNotFound", p, err)
		}
	}
	if _, err := c.FetchText(context.Background(), "http://nowhere.example/"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unresolvable domain: got %v, want ErrNotFound", err)
	}
	_, err := c.FetchText(context.Background(), "http://gone.example/503")
	if !errors.Is(err, ErrUnavailable) || errors.Is(err, ErrNotFound) {
		t.Errorf("503: got %v, want ErrUnavailable", err)
	}
}

func TestB60_ArxivSpacedThreeSeconds(t *testing.T) {
	t.Parallel()
	f, c := apiNet(t)
	for _, q := range []string{"microplastics detection", "poverty brain development"} {
		if _, err := c.SearchLiterature(context.Background(), Query{Text: q, Where: ServiceArxiv}); err != nil {
			t.Fatalf("SearchLiterature(%q): %v", q, err)
		}
	}
	reqs := f.requests()
	if len(reqs) != 2 {
		t.Fatalf("requests: got %d, want 2", len(reqs))
	}
	if gap := reqs[1].at.Sub(reqs[0].at); gap < ArxivSpacing {
		t.Errorf("arXiv requests %v apart, want at least %v", gap, ArxivSpacing)
	}
}

func TestB60_RedirectCarriesNoMetaKeyOrCookies(t *testing.T) {
	const secret = "meta-test-secret-key"
	t.Setenv("META_API_KEY", secret)
	f, c := pageNet(t)
	f.serve("a.example", "93.184.216.30", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "abc123", Path: "/", Domain: "example"})
		http.Redirect(w, r, "http://b.example/landing", http.StatusFound)
	})
	f.serve("b.example", "93.184.216.31", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("landed"))
	})
	for i := 0; i < 2; i++ {
		text, err := c.FetchText(context.Background(), "http://a.example/start")
		if err != nil || text.Text != "landed" || text.URL != "http://b.example/landing" {
			t.Fatalf("FetchText: got %+v, %v", text, err)
		}
	}
	reqs := f.requests()
	if len(reqs) != 4 {
		t.Fatalf("requests: got %d, want 4", len(reqs))
	}
	for _, r := range reqs {
		if r.header.Get("Cookie") != "" || r.header.Get("Authorization") != "" || r.header.Get("Proxy-Authorization") != "" {
			t.Errorf("%s%s carried credentials: %v", r.host, r.target, r.header)
		}
		for name, vals := range r.header {
			if strings.Contains(strings.Join(vals, " "), secret) {
				t.Errorf("%s%s: header %s carries the Meta key", r.host, r.target, name)
			}
		}
		if !strings.Contains(r.header.Get("User-Agent"), "mailto:"+testContact) {
			t.Errorf("%s%s: User-Agent %q", r.host, r.target, r.header.Get("User-Agent"))
		}
	}
}

// minimalPDF builds a one-page PDF that shows text.
func minimalPDF(text string) []byte {
	content := fmt.Sprintf("BT /F1 12 Tf 72 720 Td (%s) Tj ET", text)
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [4 0 R] /Count 1 >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 5 0 R /Resources << /Font << /F1 3 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
	}
	var b strings.Builder
	b.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objs)+1)
	for i, body := range objs {
		offsets[i+1] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, body)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for i := 1; i <= len(objs); i++ {
		fmt.Fprintf(&b, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return []byte(b.String())
}

// --- Verifier findings on 3.1 (F1, A1, A2) ---

// B-59: an API host that does not resolve (offline, DNS down) is not a
// missing record: Lookup answers ErrUnavailable, so the reference is
// kept as not verified instead of being discarded as nonexistent.
func TestB59_LookupWithAnUnresolvableAPIHostIsUnavailable(t *testing.T) {
	t.Parallel()
	f := newFakeNet(t)
	c := New(Options{ContactEmail: testContact, BaseURLs: map[string]string{
		ServiceOpenAlex: "http://api.openalex.down",
		ServiceArxiv:    "http://export.arxiv.down",
		ServiceCrossref: "http://api.crossref.down",
	}})
	c.lookupIP = f.lookup
	c.dialIP = f.dial
	for _, ref := range []Reference{{DOI: "10.1016/j.watres.2019.114996"}, {Title: "Microplastics in drinking-water"}} {
		_, err := c.Lookup(context.Background(), ref)
		if errors.Is(err, ErrNotFound) || !errors.Is(err, ErrUnavailable) {
			t.Errorf("Lookup(%+v): got %v, want ErrUnavailable", ref, err)
		}
	}
	if _, err := c.SearchLiterature(context.Background(), Query{Text: "microplastics", Where: ServiceOpenAlex}); errors.Is(err, ErrNotFound) {
		t.Errorf("SearchLiterature with the API host down: got ErrNotFound, want ErrUnavailable")
	}
	// A page whose domain does not resolve still is ErrNotFound (B-59 c).
	if _, err := c.FetchText(context.Background(), "http://gone.example/paper"); !errors.Is(err, ErrNotFound) {
		t.Errorf("FetchText of an unresolvable page: got %v, want ErrNotFound", err)
	}
}

// B-60: IPv4-compatible IPv6 (::7f00:1 is 127.0.0.1) and Teredo
// (2001::/32) addresses are blocked too.
func TestB60_IPv4CompatibleAndTeredoAddressesBlocked(t *testing.T) {
	t.Parallel()
	f, c := pageNet(t)
	f.hosts["compat.example"] = []netip.Addr{netip.MustParseAddr("::7f00:1")}
	for _, u := range []string{"http://[::7f00:1]/", "http://[::a00:5]/", "http://[2001:0:4136:e378:8000:63bf:3fff:fdd2]/", "http://compat.example/"} {
		if _, err := c.FetchText(context.Background(), u); !errors.Is(err, ErrBlocked) {
			t.Errorf("FetchText(%s): got %v, want ErrBlocked", u, err)
		}
	}
	if n := f.dialCount(); n != 0 {
		t.Errorf("blocked addresses were dialed %d times", n)
	}
}

// B-58: a DOI written by a model with dot segments or a query never
// steers the API path: it is not a DOI, so no request is made for it.
func TestB58_MalformedDOIIsNotQueried(t *testing.T) {
	t.Parallel()
	f, c := apiNet(t)
	for _, doi := range []string{"10.1/../../authors?x=1#y", "10.1/./x", "10.1/a b", "10.1/x?y=1", "10.1/"} {
		if _, err := c.Lookup(context.Background(), Reference{DOI: doi}); !errors.Is(err, ErrNotFound) {
			t.Errorf("Lookup(DOI %q): got %v, want ErrNotFound", doi, err)
		}
	}
	if n := len(f.requests()); n != 0 {
		t.Errorf("malformed DOIs made %d requests, want 0", n)
	}
}
