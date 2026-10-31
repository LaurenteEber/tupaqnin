package evidence

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
)

// The API answers recorded by the evidence micro-test (3.0) and the
// hand-made ones of 3.1, in the 3.0 format. With TQ_PROVIDER=fake and in
// the readers' tests the app answers from them and never reaches the
// network (backlog-r3 3.2).
//
//go:embed testdata/recordings/*.json
var recordingsFS embed.FS

// replayRecording is one recorded API answer.
type replayRecording struct {
	URL         string `json:"url"`
	Status      int    `json:"status"`
	ContentType string `json:"content_type"`
	Body        string `json:"body"`
}

// replayPublicIP is the address every name resolves to in a replay: a
// public documentation-range address, so the B-60 checks pass without
// DNS. Nothing ever dials it.
var replayPublicIP = netip.MustParseAddr("93.184.216.34")

// NewReplay builds a Client that answers from the embedded recordings
// and never reaches the network. A request with a recording gets it; an
// OpenAlex or arXiv search without one gets an empty result list; any
// other request (a page, a registry lookup) answers 503, so it reads as
// ErrUnavailable («no verificado»), never as «no existe».
// [EL DISEÑO NO LO DICE]: what the replay answers without a recording;
// the most conservative answer is «unavailable». arXiv's 3 s spacing
// does not apply: nothing reaches arXiv.
func NewReplay() *Client {
	c := New(Options{})
	recs := loadReplay()
	c.lookupIP = func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{replayPublicIP}, nil
	}
	c.dialIP = nil // never used: the transport below answers
	c.noSpacing = true
	c.http.Transport = replayTransport(recs)
	return c
}

func loadReplay() map[string]replayRecording {
	out := map[string]replayRecording{}
	entries, err := recordingsFS.ReadDir("testdata/recordings")
	if err != nil {
		return out
	}
	for _, e := range entries {
		b, err := recordingsFS.ReadFile("testdata/recordings/" + e.Name())
		if err != nil {
			continue
		}
		var rec replayRecording
		if json.Unmarshal(b, &rec) != nil || rec.URL == "" {
			continue
		}
		u, err := url.Parse(rec.URL)
		if err != nil {
			continue
		}
		out[replayKey(u)] = rec
	}
	return out
}

func replayKey(u *url.URL) string {
	return strings.ToLower(u.Host) + u.EscapedPath() + "?" + u.RawQuery
}

type replayTransport map[string]replayRecording

func (t replayTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	status, contentType, body := http.StatusServiceUnavailable, "text/plain", ""
	if rec, ok := t[replayKey(req.URL)]; ok {
		status, contentType, body = rec.Status, rec.ContentType, rec.Body
	} else {
		switch {
		case strings.HasSuffix(req.URL.Path, "/works") && req.URL.Query().Has("search"):
			status, contentType, body = http.StatusOK, "application/json", `{"results":[]}`
		case strings.HasSuffix(req.URL.Path, "/api/query"):
			status, contentType = http.StatusOK, "application/atom+xml"
			body = `<?xml version="1.0" encoding="UTF-8"?><feed xmlns="http://www.w3.org/2005/Atom"></feed>`
		}
	}
	return &http.Response{
		StatusCode: status, Status: http.StatusText(status), Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
		Header:  http.Header{"Content-Type": {contentType}},
		Body:    io.NopCloser(bytes.NewReader([]byte(body))),
		Request: req, ContentLength: int64(len(body)),
	}, nil
}
