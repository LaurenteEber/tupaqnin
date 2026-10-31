package evidence

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/LaurenteEber/tupaqnin/internal/pdfextract"
)

// blockedPrefixes are ranges refused besides what netip already
// classifies as private, loopback, link-local, multicast or
// unspecified. [EL DISEÑO NO LO DICE] B-60 names private, local and
// link-local addresses; the most conservative reading also refuses
// shared (CGNAT), reserved, benchmarking and translation ranges, which
// can reach a private network.
var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("2002::/16"),
	// IPv4-compatible IPv6 (::a.b.c.d) and Teredo, which netip counts as
	// global unicast (verifier A1 on 3.1).
	netip.MustParsePrefix("::/96"),
	netip.MustParsePrefix("2001::/32"),
}

// errHostNotFound is a name that does not resolve. A page whose domain
// does not resolve is ErrNotFound (B-59 c); an API host that does not
// resolve is ErrUnavailable, never a missing record (verifier F1).
var errHostNotFound = errors.New("evidence: host not found")

// blockedIP reports whether the client must not connect to ip (B-60).
func blockedIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() || ip.IsMulticast() ||
		ip.IsUnspecified() || !ip.IsGlobalUnicast() {
		return true
	}
	for _, p := range blockedPrefixes {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// blockedName refuses Fly's private names (*.internal) and localhost
// before any lookup (B-60).
func blockedName(host string) bool {
	h := strings.TrimSuffix(strings.ToLower(host), ".")
	return h == "" || h == "localhost" || strings.HasSuffix(h, ".localhost") ||
		h == "internal" || strings.HasSuffix(h, ".internal")
}

// dialControl checks the socket address right before the connection
// is made (B-60, precision of 31-oct): a name whose DNS answer changes
// cannot get past it.
func dialControl(_, address string, _ syscall.RawConn) error {
	ap, err := netip.ParseAddrPort(address)
	if err != nil || blockedIP(ap.Addr()) {
		return ErrBlocked
	}
	return nil
}

func defaultLookup(ctx context.Context, host string) ([]netip.Addr, error) {
	return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
}

func defaultDial(ctx context.Context, network, address string) (net.Conn, error) {
	d := net.Dialer{Timeout: Timeout, Control: dialControl}
	return d.DialContext(ctx, network, address)
}

// resolve returns the addresses of host and refuses them all if any
// one is blocked.
func (c *Client) resolve(ctx context.Context, host string) ([]netip.Addr, error) {
	if blockedName(host) {
		return nil, ErrBlocked
	}
	if ip, err := netip.ParseAddr(strings.Trim(host, "[]")); err == nil {
		if blockedIP(ip) {
			return nil, ErrBlocked
		}
		return []netip.Addr{ip.Unmap()}, nil
	}
	addrs, err := c.lookupIP(ctx, host)
	if err != nil {
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
			return nil, errHostNotFound
		}
		return nil, ErrUnavailable
	}
	if len(addrs) == 0 {
		return nil, errHostNotFound
	}
	out := make([]netip.Addr, 0, len(addrs))
	for _, a := range addrs {
		if blockedIP(a) {
			return nil, ErrBlocked
		}
		out = append(out, a.Unmap())
	}
	return out, nil
}

// dialContext resolves the name again at connect time, checks every
// address and connects to the IP itself, so no second resolution can
// swap it (B-60).
func (c *Client) dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, ErrBlocked
	}
	addrs, err := c.resolve(ctx, host)
	if err != nil {
		return nil, err
	}
	var lastErr error = ErrUnavailable
	for _, a := range addrs {
		conn, err := c.dialIP(ctx, network, net.JoinHostPort(a.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// checkURL allows only http and https URLs with an allowed host, and
// resolves the host once before the request (the dial checks again).
func (c *Client) checkURL(ctx context.Context, u *url.URL) error {
	if u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return ErrBlocked
	}
	_, err := c.resolve(ctx, u.Hostname())
	return err
}

func (c *Client) newHTTPClient() *http.Client {
	tr := &http.Transport{
		Proxy:                  nil, // never through an environment proxy
		DialContext:            c.dialContext,
		ForceAttemptHTTP2:      true,
		TLSHandshakeTimeout:    Timeout,
		ResponseHeaderTimeout:  Timeout,
		MaxResponseHeaderBytes: 64 << 10,
		MaxIdleConns:           16,
		IdleConnTimeout:        30 * time.Second,
	}
	return &http.Client{
		Transport: tr,
		Timeout:   Timeout,
		Jar:       nil, // no cookies, ever
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > MaxRedirects {
				return ErrTooManyRedirects
			}
			if err := c.checkURL(req.Context(), req.URL); err != nil {
				return err
			}
			// Nothing the client sends carries credentials; drop them
			// anyway on every hop (B-60).
			for _, h := range []string{"Authorization", "Proxy-Authorization", "Cookie"} {
				req.Header.Del(h)
			}
			return nil
		},
	}
}

// response is one answer read under the B-60 limits.
type response struct {
	finalURL  string
	mediaType string
	body      []byte
}

// get calls an API URL the package built: a host that does not resolve
// is ErrUnavailable (verifier F1), only the API's own 404 is ErrNotFound.
func (c *Client) get(ctx context.Context, rawURL string, allowed []string) (response, error) {
	res, err := c.download(ctx, rawURL, allowed)
	if errors.Is(err, errHostNotFound) {
		return response{}, ErrUnavailable
	}
	return res, err
}

// download fetches rawURL and accepts only the given media types; a
// host that does not resolve is errHostNotFound for the caller to map.
func (c *Client) download(ctx context.Context, rawURL string, allowed []string) (response, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return response{}, ErrBlocked
	}
	if err := c.checkURL(ctx, u); err != nil {
		return response{}, err
	}
	release, err := c.waitArxiv(ctx, u.Hostname())
	if err != nil {
		return response{}, err
	}
	defer release()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return response{}, ErrBlocked
	}
	req.Header.Set("User-Agent", c.userAgent())
	resp, err := c.http.Do(req)
	if err != nil {
		return response{}, transportError(ctx, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return response{}, ErrNotFound
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		return response{}, fmt.Errorf("%w: http %d", ErrUnavailable, resp.StatusCode)
	}
	// The transport decompresses gzip itself (resp.Uncompressed); any
	// other encoding could not be measured after decompression.
	if enc := resp.Header.Get("Content-Encoding"); enc != "" && !strings.EqualFold(enc, "identity") && !resp.Uncompressed {
		return response{}, fmt.Errorf("%w: content encoding", ErrContentType)
	}
	mediaType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || !contains(allowed, mediaType) {
		return response{}, ErrContentType
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxBytes+1))
	if err != nil {
		return response{}, transportError(ctx, err)
	}
	if len(body) > MaxBytes {
		return response{}, ErrTooLarge
	}
	return response{finalURL: resp.Request.URL.String(), mediaType: mediaType, body: body}, nil
}

// transportError maps a client error to a sentinel without the URL.
func transportError(ctx context.Context, err error) error {
	for _, known := range []error{ErrBlocked, ErrTooManyRedirects, ErrNotFound, errHostNotFound} {
		if errors.Is(err, known) {
			return known
		}
	}
	if ctx.Err() != nil && !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return ctx.Err()
	}
	var ne net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout()) {
		return ErrTimeout
	}
	return ErrUnavailable
}

// waitArxiv keeps at most one arXiv request every ArxivSpacing (B-60).
func (c *Client) waitArxiv(ctx context.Context, host string) (release func(), err error) {
	if c.noSpacing || !c.isArxiv(host) {
		return func() {}, nil
	}
	// arXiv requests go one at a time and the 3 s count from the end of
	// the previous one, so arXiv never sees two closer than 3 s, however
	// long a request took to arrive (B-60).
	c.arxivMu.Lock()
	if wait := ArxivSpacing - time.Since(c.arxivLast); !c.arxivLast.IsZero() && wait > 0 {
		t := time.NewTimer(wait)
		defer t.Stop()
		select {
		case <-ctx.Done():
			c.arxivMu.Unlock()
			return nil, ctx.Err()
		case <-t.C:
		}
	}
	return func() {
		c.arxivLast = time.Now()
		c.arxivMu.Unlock()
	}, nil
}

// isArxiv covers the API host and arxiv.org itself (its PDFs).
func (c *Client) isArxiv(host string) bool {
	h := strings.ToLower(host)
	if h == "arxiv.org" || strings.HasSuffix(h, ".arxiv.org") {
		return true
	}
	if u, err := url.Parse(c.base[ServiceArxiv]); err == nil && strings.EqualFold(u.Hostname(), h) {
		return true
	}
	return false
}

// FetchText downloads url under the B-60 rules and returns its text:
// HTML without scripts or styles, a PDF through the child-process
// extractor (B-9), plain text as is. Callers pass only URLs that came
// from an API or a search result, never one a model wrote (B-58).
func (c *Client) FetchText(ctx context.Context, rawURL string) (Text, error) {
	res, err := c.download(ctx, rawURL, AllowedContentTypes)
	if errors.Is(err, errHostNotFound) {
		return Text{}, ErrNotFound // a page whose domain does not resolve (B-59 c)
	}
	if err != nil {
		return Text{}, err
	}
	out := Text{URL: res.finalURL, ContentType: res.mediaType, Raw: res.body}
	switch res.mediaType {
	case "text/html":
		out.Text = htmlText(res.body)
	case "application/pdf":
		text, err := c.extractPDF(ctx, res.body)
		if err != nil {
			return Text{}, fmt.Errorf("evidence: pdf: %w", err)
		}
		out.Text = text
	default:
		// [EL DISEÑO NO LO DICE] the charset of text/plain: read as
		// UTF-8, invalid bytes replaced, no conversion.
		out.Text = strings.ToValidUTF8(string(res.body), string(utf8.RuneError))
	}
	return out, nil
}

// PDFWait is how long FetchText waits for the PDF extractor when
// another extraction holds it (backlog-r3 3.3: it waits its turn, with
// its deadline, instead of getting ErrBusy). [EL DISEÑO NO LO DICE]:
// the deadline; the extractor's own timeout, the longest the running
// extraction can last.
const PDFWait = pdfextract.Timeout

// pdfPoll is how often FetchText asks the extractor again while it
// waits.
const pdfPoll = 100 * time.Millisecond

// extractPDF runs the child-process extractor (B-9) and, while another
// extraction holds it (pdfextract admits one at a time), waits its turn
// until pdfWait passes or ctx ends.
func (c *Client) extractPDF(ctx context.Context, data []byte) (string, error) {
	deadline := time.Now().Add(c.pdfWait)
	for {
		text, err := c.pdfExtract(ctx, data)
		if !errors.Is(err, pdfextract.ErrBusy) || !time.Now().Before(deadline) {
			return text, err
		}
		t := time.NewTimer(pdfPoll)
		select {
		case <-ctx.Done():
			t.Stop()
			return "", ctx.Err()
		case <-t.C:
		}
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
