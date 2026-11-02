package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

const (
	wantCSP  = "default-src 'self'; script-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'"
	wantHSTS = "max-age=31536000"
)

// raw sends body as it is, with the given headers.
func raw(h http.Handler, method, path string, body io.Reader, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, body)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// errorCode reads the API's error code from a response.
func errorCode(rec *httptest.ResponseRecorder) string {
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return body["error"]
}

func wantRefused(t *testing.T, what string, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status || errorCode(rec) != code {
		t.Fatalf("%s: got %d (%s), want %d %s", what, rec.Code, rec.Body.String(), status, code)
	}
}

func countSources(t *testing.T, db *store.DB, projectID string) int {
	t.Helper()
	sources, err := db.ListObjects(context.Background(), projectID, store.ObjectSource)
	if err != nil {
		t.Fatalf("ListObjects: %v", err)
	}
	return len(sources)
}

// sectionBody is a section save of the given text as JSON.
func sectionBody(t *testing.T, version int, text string) []byte {
	t.Helper()
	out, err := json.Marshal(map[string]any{"base_version": version, "text": text})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return out
}

// B3-22 row 10: HSTS on every response, and the CSP stays as it is.
func TestB3_22_10_HSTSEverywhere(t *testing.T) {
	h, _, _ := newAPI(t)
	for _, tc := range []struct {
		path string
		code int
	}{
		{"/healthz", http.StatusOK},
		{"/", http.StatusOK},
		{"/api/projects", http.StatusOK},
		{"/no-such-page", http.StatusNotFound},
		{"/api/projects/does-not-exist/request", http.StatusNotFound},
	} {
		rec := do(t, h, http.MethodGet, tc.path, nil)
		if rec.Code != tc.code {
			t.Errorf("%s: got %d, want %d", tc.path, rec.Code, tc.code)
		}
		if got := rec.Header().Get("Strict-Transport-Security"); got != wantHSTS {
			t.Errorf("%s: HSTS %q, want %q", tc.path, got, wantHSTS)
		}
		if got := rec.Header().Get("Content-Security-Policy"); got != wantCSP {
			t.Errorf("%s: CSP %q, want %q", tc.path, got, wantCSP)
		}
	}
	// Refusals of the edge carry it too.
	refused := raw(h, http.MethodPost, "/api/projects", strings.NewReader("{}"), map[string]string{"Sec-Fetch-Site": "cross-site"})
	if got := refused.Header().Get("Strict-Transport-Security"); refused.Code != http.StatusForbidden || got != wantHSTS {
		t.Errorf("cross-site refusal: %d, HSTS %q", refused.Code, got)
	}
}

// B3-22 row 10: 300 KB to a section is 413 and writes nothing, with or
// without a declared length.
func TestB3_22_10_SectionBodyOverTheCapIs413(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	seedUnderstanding(t, db, projectID)
	base := "/api/projects/" + projectID
	before := sectionVersion(t, h, base)
	body := sectionBody(t, before.Version, strings.Repeat("a", 300<<10))

	wantRefused(t, "declared length", raw(h, http.MethodPut, base+"/sections/sec-1", bytes.NewReader(body), nil),
		http.StatusRequestEntityTooLarge, "body_too_large")
	// An io.Reader of unknown length arrives without Content-Length, as a
	// chunked body does; no handler may turn it into a 400.
	wantRefused(t, "unknown length", raw(h, http.MethodPut, base+"/sections/sec-1", io.MultiReader(bytes.NewReader(body)), nil),
		http.StatusRequestEntityTooLarge, "body_too_large")

	after := sectionVersion(t, h, base)
	if after.Version != before.Version || after.Text != before.Text {
		t.Fatalf("section changed: version %d → %d", before.Version, after.Version)
	}
}

// B3-22 row 10: no route of the table, nor what is not in it, answers
// anything but 413 to a body over the cap. The upload has its own.
func TestB3_22_10_EveryRouteCapsTheBody(t *testing.T) {
	h, _, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	oversize := bytes.Repeat([]byte("a"), 300<<10)
	walked := 0
	for _, row := range server.RouteTableForTest() {
		method, pattern, _ := strings.Cut(row, " ")
		if pattern == "/api/projects/{projectID}/sources" && method == http.MethodPost {
			continue
		}
		walked++
		path := strings.NewReplacer("{projectID}", projectID, "{label}", "sec-1", "{n}", "1", "{turnID}", "turn-1", "{token}", "t").Replace(pattern)
		// An unknown length, so a handler that reads less than the cap
		// cannot get an answer in first.
		wantRefused(t, row, raw(h, method, path, io.MultiReader(bytes.NewReader(oversize)), nil),
			http.StatusRequestEntityTooLarge, "body_too_large")
	}
	if walked < 50 {
		t.Fatalf("walked %d routes, want the whole table", walked)
	}
	for _, path := range []string{"/", "/no-such-page"} {
		wantRefused(t, path, raw(h, http.MethodGet, path, bytes.NewReader(oversize), nil),
			http.StatusRequestEntityTooLarge, "body_too_large")
	}
}

// Previous behaviour: a section of 200 KB, the design's maximum, saves
// and comes back whole.
func TestB3_22_10_SectionOf200KBStillSaves(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	seedUnderstanding(t, db, projectID)
	base := "/api/projects/" + projectID
	text := strings.Repeat("Una línea de la sección.\n", (200<<10)/len("Una línea de la sección.\n"))
	rec := raw(h, http.MethodPut, base+"/sections/sec-1", bytes.NewReader(sectionBody(t, 1, text)), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("save: got %d (%s), want 200", rec.Code, rec.Body.String())
	}
	if got := sectionVersion(t, h, base); got.Text != text || got.Version != 2 {
		t.Fatalf("section: version %d, %d bytes, want version 2 and %d bytes", got.Version, len(got.Text), len(text))
	}
}

// B3-22 row 10: a source over the upload's cap is 413 with the code it
// always had, stores nothing, and a source at the limit still goes in.
func TestB3_22_10_SourceOverTheCapIs413(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID

	wantRefused(t, "6 MB", upload(t, h, base, "file", "grande.txt", bytes.Repeat([]byte("a"), 6<<20)),
		http.StatusRequestEntityTooLarge, "source_too_large")
	wantRefused(t, "5 MB + 1 byte", upload(t, h, base, "file", "grande.txt", bytes.Repeat([]byte("a"), (5<<20)+1)),
		http.StatusRequestEntityTooLarge, "source_too_large")
	if got := countSources(t, db, projectID); got != 0 {
		t.Fatalf("sources after refusals: %d, want 0", got)
	}

	data := bytes.Repeat([]byte("a"), 5<<20)
	rec := upload(t, h, base, "file", "justo.txt", data)
	if rec.Code != http.StatusCreated {
		t.Fatalf("5 MB: got %d (%s), want 201", rec.Code, rec.Body.String())
	}
	dl := do(t, h, http.MethodGet, base+"/sources/src-1/download", nil)
	if dl.Code != http.StatusOK || dl.Body.Len() != len(data) {
		t.Fatalf("download: got %d, %d bytes, want 200 and %d", dl.Code, dl.Body.Len(), len(data))
	}
}

// The upload's cap is decided as the mux decides its route, on decoded
// segments: an escaped spelling of the same route gets the same cap.
func TestB3_22_10_EscapedUploadPathKeepsItsCap(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID
	got := uploadTo(t, h, base+"/sou%72ces", "medio.txt", bytes.Repeat([]byte("a"), 1<<20))
	if got.Code != http.StatusCreated {
		t.Fatalf("escaped path, 1 MB: got %d (%s), want 201", got.Code, got.Body.String())
	}
	if n := countSources(t, db, projectID); n != 1 {
		t.Fatalf("sources: %d, want 1", n)
	}
}

// Previous behaviour: what is not multipart is still the 400 it was;
// the 413 is for size alone.
func TestB3_22_10_UploadThatIsNotMultipartStays400(t *testing.T) {
	h, _, _ := newAPI(t)
	base := "/api/projects/" + newProject(t, h, nil)
	wantRefused(t, "not multipart", raw(h, http.MethodPost, base+"/sources", strings.NewReader("hola"), nil),
		http.StatusBadRequest, "source_too_large")
}

// Specification §6: http.CrossOriginProtection in front of the mux. A
// cross-site POST is 403, creates nothing and leaves its log line.
func TestB3_22_CrossSitePostIs403(t *testing.T) {
	a := newAccess(t)
	_, token := newLink(t, a.db, "jury", 10)
	cookie := a.visitor(t, token)
	projects := a.count(t, `SELECT COUNT(*) FROM project`)

	for _, headers := range []map[string]string{
		{"Sec-Fetch-Site": "cross-site"},
		{"Sec-Fetch-Site": "same-site"},
		{"Origin": "https://evil.example"},
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/projects", strings.NewReader(`{"name":"x"}`))
		req.AddCookie(cookie)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		a.h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%v: got %d (%s), want 403", headers, rec.Code, rec.Body.String())
		}
	}
	if got := a.count(t, `SELECT COUNT(*) FROM project`); got != projects {
		t.Errorf("projects: %d → %d", projects, got)
	}

	var logged bytes.Buffer
	h := server.New(slog.New(slog.NewJSONHandler(&logged, nil)))
	rec := raw(h, http.MethodPost, "/healthz", nil, map[string]string{"Sec-Fetch-Site": "cross-site"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("log request: got %d", rec.Code)
	}
	var line map[string]any
	if err := json.Unmarshal(logged.Bytes(), &line); err != nil {
		t.Fatalf("log line %q: %v", logged.String(), err)
	}
	if line["path"] != "/healthz" || line["status"] != float64(http.StatusForbidden) {
		t.Fatalf("log line: %v", line)
	}
}

// Previous behaviour: same-origin POSTs, requests with no browser
// headers and a link opened from another site all still work.
func TestB3_22_SameOriginAndLinkStillWork(t *testing.T) {
	a := newAccess(t)
	_, token := newLink(t, a.db, "jury", 10)

	// A link clicked in another site's page is a cross-site GET.
	entry := httptest.NewRequest(http.MethodGet, "/i/"+token, nil)
	entry.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, entry)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" || len(rec.Result().Cookies()) != 1 {
		t.Fatalf("link: got %d, Location %q, %d cookies", rec.Code, rec.Header().Get("Location"), len(rec.Result().Cookies()))
	}
	cookie := rec.Result().Cookies()[0]

	req := httptest.NewRequest(http.MethodPost, "/api/projects", strings.NewReader(`{"name":"x"}`))
	req.Host = "tupaqnin.example"
	req.AddCookie(cookie)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Origin", "https://tupaqnin.example")
	rec = httptest.NewRecorder()
	a.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("same-origin POST: got %d (%s), want 201", rec.Code, rec.Body.String())
	}
}

// B3-22 row 7: closed list of types, sources served as a download.
func TestB3_22_7_UploadTypes(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID
	payload := []byte("<script>alert(document.cookie)</script>")

	for _, name := range []string{"x.html", "x.HTML", "x.svg", "x.js", "x.htm", "x.md.html"} {
		wantRefused(t, name, upload(t, h, base, "file", name, payload), http.StatusBadRequest, "source_unsupported_type")
	}
	if got := countSources(t, db, projectID); got != 0 {
		t.Fatalf("sources after refusals: %d, want 0", got)
	}

	for i, tc := range []struct{ name, mediaType string }{
		{"a.md", "text/markdown"}, {"b.txt", "text/plain"}, {"c.csv", "text/csv"},
	} {
		data := append(append([]byte{}, payload...), byte('0'+i))
		if rec := upload(t, h, base, "file", tc.name, data); rec.Code != http.StatusCreated {
			t.Fatalf("%s: got %d (%s), want 201", tc.name, rec.Code, rec.Body.String())
		}
		dl := do(t, h, http.MethodGet, base+"/sources/src-"+string(rune('1'+i))+"/download", nil)
		if dl.Code != http.StatusOK || !bytes.Equal(dl.Body.Bytes(), data) {
			t.Fatalf("%s download: got %d %q", tc.name, dl.Code, dl.Body.String())
		}
		if got := dl.Header().Get("Content-Disposition"); !strings.HasPrefix(got, "attachment") || !strings.Contains(got, tc.name) {
			t.Errorf("%s: Content-Disposition %q", tc.name, got)
		}
		if got := dl.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%s: X-Content-Type-Options %q", tc.name, got)
		}
		if got := dl.Header().Get("Content-Type"); got != tc.mediaType {
			t.Errorf("%s: Content-Type %q, want %q", tc.name, got, tc.mediaType)
		}
	}

	usePDFHelper(t)
	if rec := upload(t, h, base, "file", "d.pdf", minimalPDF(t, "Texto")); rec.Code != http.StatusCreated {
		t.Fatalf("pdf: got %d (%s), want 201", rec.Code, rec.Body.String())
	}
}

// B3-22 row 8: a malicious section comes back as the text it is, in the
// API and in export.md, and neither answer is a page.
func TestB3_22_8_MaliciousDocument(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	seedUnderstanding(t, db, projectID)
	base := "/api/projects/" + projectID
	text := "<script>alert(1)</script>\n\n<img src=\"https://evil.example/x.png\">\n\n[x](javascript:alert(1))"

	if rec := do(t, h, http.MethodPut, base+"/sections/sec-1", map[string]any{"base_version": 1, "text": text}); rec.Code != http.StatusOK {
		t.Fatalf("save: got %d (%s)", rec.Code, rec.Body.String())
	}

	api := do(t, h, http.MethodGet, base+"/sections/sec-1", nil)
	if got := decode[server.SectionDetail](t, api).Text; got != text {
		t.Errorf("API text: got %q, want %q", got, text)
	}
	if got := api.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Errorf("API Content-Type %q", got)
	}

	export := do(t, h, http.MethodGet, base+"/export.md", nil)
	if export.Code != http.StatusOK || !strings.Contains(export.Body.String(), text) {
		t.Errorf("export.md: got %d, text not literal: %q", export.Code, export.Body.String())
	}
	if got := export.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/markdown") {
		t.Errorf("export Content-Type %q", got)
	}
	if got := export.Header().Get("Content-Disposition"); !strings.HasPrefix(got, "attachment") {
		t.Errorf("export Content-Disposition %q", got)
	}

	for what, rec := range map[string]*httptest.ResponseRecorder{"api": api, "export": export} {
		if got := rec.Header().Get("Content-Security-Policy"); got != wantCSP {
			t.Errorf("%s: CSP %q, want %q", what, got, wantCSP)
		}
		if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%s: X-Content-Type-Options %q", what, got)
		}
	}
}

// The edge over a real connection: the caps answer before the client has
// a broken pipe, and a browser's same-origin POST passes with its Origin.
func TestB3_22_10_OverARealConnection(t *testing.T) {
	h, _, _ := newAPI(t)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	send := func(method, path string, body io.Reader, headers map[string]string) (int, string) {
		t.Helper()
		req, err := http.NewRequest(method, srv.URL+path, body)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
		defer func() { _ = resp.Body.Close() }()
		raw, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(raw)
	}

	code, body := send(http.MethodPost, "/api/projects", strings.NewReader(`{"name":"x"}`),
		map[string]string{"Sec-Fetch-Site": "same-origin", "Origin": srv.URL})
	if code != http.StatusCreated {
		t.Fatalf("same-origin POST: got %d (%s)", code, body)
	}
	var created server.CreateProjectResponse
	if err := json.Unmarshal([]byte(body), &created); err != nil {
		t.Fatalf("created: %v", err)
	}
	base := "/api/projects/" + created.ProjectID

	oversize := io.MultiReader(strings.NewReader(strings.Repeat("a", 300<<10))) // chunked
	if code, body := send(http.MethodPut, base+"/request", oversize, nil); code != http.StatusRequestEntityTooLarge || !strings.Contains(body, "body_too_large") {
		t.Errorf("300 KB chunked: got %d (%s)", code, body)
	}

	var form bytes.Buffer
	mw := multipart.NewWriter(&form)
	_ = mw.WriteField("kind", "file")
	fw, _ := mw.CreateFormFile("file", "grande.txt")
	_, _ = fw.Write(bytes.Repeat([]byte("a"), 6<<20))
	_ = mw.Close()
	code, body = send(http.MethodPost, base+"/sources", &form, map[string]string{"Content-Type": mw.FormDataContentType()})
	if code != http.StatusRequestEntityTooLarge || !strings.Contains(body, "source_too_large") {
		t.Errorf("6 MB source: got %d (%s)", code, body)
	}
}

// uploadTo posts a multipart file to the exact path given.
func uploadTo(t *testing.T, h http.Handler, path, filename string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("kind", "file")
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("form file: %v", err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	_ = mw.Close()
	return raw(h, http.MethodPost, path, &body, map[string]string{"Content-Type": mw.FormDataContentType()})
}

// countingReader counts what the server takes from the connection.
type countingReader struct {
	r    io.Reader
	read int
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.read += n
	return n, err
}

// B3-22 row 10: nobody without a session can make the server hold a large
// body: their requests take no large body, so the cut comes early.
func TestB3_22_10_StrangerBodyIsCutEarly(t *testing.T) {
	h, _, _ := newAPI(t)
	for _, path := range []string{"/healthz", "/", "/i/unknown", "/no-such-page"} {
		body := &countingReader{r: bytes.NewReader(bytes.Repeat([]byte("a"), 250<<10))}
		req := httptest.NewRequest(http.MethodPost, path, body)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		wantRefused(t, path, rec, http.StatusRequestEntityTooLarge, "body_too_large")
		if body.read > 8<<10 {
			t.Fatalf("%s: read %d bytes of a stranger's body, want at most 8 KB", path, body.read)
		}
	}
}
