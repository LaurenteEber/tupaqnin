package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/fingerprint"
	"github.com/LaurenteEber/tupaqnin/internal/pdfextract"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// TestPDFHelperProcess is the extraction child for these tests.
func TestPDFHelperProcess(t *testing.T) {
	if os.Getenv("TQ_PDF_HELPER") == "child" {
		os.Exit(pdfextract.RunChild(os.Stdin, os.Stdout, os.Stderr))
	}
}

func usePDFHelper(t *testing.T) {
	t.Helper()
	prev := pdfextract.ChildCommand
	pdfextract.ChildCommand = func(ctx context.Context) (*exec.Cmd, error) {
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestPDFHelperProcess", "--")
		cmd.Env = append(os.Environ(), "TQ_PDF_HELPER=child")
		return cmd, nil
	}
	t.Cleanup(func() { pdfextract.ChildCommand = prev })
}

func upload(t *testing.T, h http.Handler, base, kind, filename string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("kind", kind)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("form file: %v", err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, base+"/sources", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func newProject(t *testing.T, h http.Handler, body any) string {
	t.Helper()
	rec := do(t, h, http.MethodPost, "/api/projects", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create project: got %d (%s)", rec.Code, rec.Body.String())
	}
	return decode[server.CreateProjectResponse](t, rec).ProjectID
}

func TestB9_RequestIsStoredLiteral(t *testing.T) {
	h, db, _ := newAPI(t)
	base := "/api/projects/" + newProject(t, h, map[string]any{"name": "Tesis"})

	in := map[string]any{
		"text": "Escribe el capítulo <tal cual>.", "audience": "Mi comité",
		"criteria": []string{"Solo estudios de campo."}, "premise": "La embotellada trae más.", "exploratory": false,
	}
	rec := do(t, h, http.MethodPut, base+"/request", in)
	if rec.Code != http.StatusOK {
		t.Fatalf("put request: got %d (%s)", rec.Code, rec.Body.String())
	}
	got := decode[server.RequestView](t, rec)
	if got.Text != in["text"] || got.Audience != "Mi comité" ||
		len(got.Criteria) != 1 || got.Criteria[0] != "Solo estudios de campo." ||
		got.Premise != "La embotellada trae más." || got.Exploratory {
		t.Fatalf("request not literal: %+v", got)
	}

	projects, err := db.ListProjects(context.Background())
	if err != nil || len(projects) != 1 || projects[0].Name != "Tesis" {
		t.Fatalf("list projects: %+v, %v", projects, err)
	}
}

func TestB9_UnsupportedAndOversizeSourcesRejectedWithCatalogCode(t *testing.T) {
	h, _, _ := newAPI(t)
	base := "/api/projects/" + newProject(t, h, nil)

	rec := upload(t, h, base, "file", "tabla.xlsx", []byte("no"))
	if rec.Code != http.StatusBadRequest ||
		decode[map[string]string](t, rec)["error"] != "source_unsupported_type" {
		t.Fatalf("xlsx: got %d (%s), want source_unsupported_type", rec.Code, rec.Body.String())
	}

	big := bytes.Repeat([]byte("a"), (5<<20)+1)
	rec = upload(t, h, base, "file", "grande.txt", big)
	if decode[map[string]string](t, rec)["error"] != "source_too_large" {
		t.Fatalf("oversize: got %d (%s), want source_too_large", rec.Code, rec.Body.String())
	}
}

func TestB9_PDFSourceStoresExtractedTextAndItsFingerprint(t *testing.T) {
	usePDFHelper(t)
	h, db, _ := newAPI(t)
	base := "/api/projects/" + newProject(t, h, nil)

	rec := upload(t, h, base, "file", "a.pdf", minimalPDF(t, "Texto del PDF"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload pdf: got %d (%s)", rec.Code, rec.Body.String())
	}
	view := decode[server.SourceView](t, rec)
	// The stored body carries the extracted text and its fingerprint.
	sources, err := db.ListObjects(context.Background(), strings.TrimPrefix(base, "/api/projects/"), store.ObjectSource)
	if err != nil || len(sources) != 1 {
		t.Fatalf("sources: %v, %v", sources, err)
	}
	if !strings.Contains(string(sources[0].Body), "Texto del PDF") ||
		!strings.Contains(string(sources[0].Body), view.Fingerprint) {
		t.Fatalf("source body: %s", sources[0].Body)
	}
	var stored struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(sources[0].Body, &stored); err != nil {
		t.Fatalf("source body JSON: %v", err)
	}
	if view.Fingerprint != fingerprint.Source(stored.Text) {
		t.Fatalf("fingerprint is not of the extracted text: %q", view.Fingerprint)
	}

	// Uploading the identical source again creates no new version.
	rec = upload(t, h, base, "file", "a.pdf", minimalPDF(t, "Texto del PDF"))
	again := decode[server.SourceView](t, rec)
	if again.Label != view.Label || again.Version != view.Version {
		t.Fatalf("identical upload: got %+v, want %+v", again, view)
	}
}

func TestB12_ProjectFromTemplateBringsRequestCriteriaAndSources(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, map[string]any{"template": "case-5"})
	base := "/api/projects/" + projectID

	rec := do(t, h, http.MethodGet, base+"/request", nil)
	got := decode[server.RequestView](t, rec)
	if got.Text == "" || len(got.Criteria) != 2 || len(got.Sources) != 15 {
		t.Fatalf("template request: criteria=%d sources=%d text=%q", len(got.Criteria), len(got.Sources), got.Text)
	}
	calculations := 0
	for _, src := range got.Sources {
		if src.Kind == "calculation" {
			calculations++
		}
	}
	if calculations != 1 {
		t.Fatalf("template calculation sources: got %d, want 1", calculations)
	}
	// P-24: before submit, no criterion or premise object exists yet.
	for _, typ := range []store.ObjectType{store.ObjectCriterion, store.ObjectPremise} {
		objs, err := db.ListObjects(context.Background(), projectID, typ)
		if err != nil || len(objs) != 0 {
			t.Fatalf("before submit: %s objects = %d, %v; want 0", typ, len(objs), err)
		}
	}
}

func TestP24_RemovedCriterionNeverBecomesAnObjectAndSubmitCreatesThem(t *testing.T) {
	h, db, starter := newAPI(t)
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID

	put := func(criteria []string) {
		rec := do(t, h, http.MethodPut, base+"/request", map[string]any{
			"text": "Pedido.", "audience": "Comité", "criteria": criteria,
			"premise": "Una idea.", "exploratory": false,
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("put request: got %d (%s)", rec.Code, rec.Body.String())
		}
	}
	put([]string{"Criterio uno.", "Criterio que se quita."})
	put([]string{"Criterio uno."}) // the second one is removed before submit

	rec := do(t, h, http.MethodPost, base+"/request/submit", nil)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("submit: got %d (%s)", rec.Code, rec.Body.String())
	}
	if starter.text != "understanding" {
		t.Fatalf("submit did not open an understanding turn: got %q", starter.text)
	}
	criteria, err := db.ListObjects(context.Background(), projectID, store.ObjectCriterion)
	if err != nil || len(criteria) != 1 {
		t.Fatalf("criteria after submit: %d, %v; want exactly the kept one", len(criteria), err)
	}
	if got := decode[server.RequestView](t, do(t, h, http.MethodGet, base+"/request", nil)); len(got.Criteria) != 1 || got.Criteria[0] != "Criterio uno." {
		t.Fatalf("view after submit: %+v", got)
	}
	premises, err := db.ListObjects(context.Background(), projectID, store.ObjectPremise)
	if err != nil || len(premises) != 1 || premises[0].State != "current" {
		t.Fatalf("premise after submit: %+v, %v", premises, err)
	}
	if criteria[0].State != "current" {
		t.Fatalf("criterion state: got %q, want current (born confirmed)", criteria[0].State)
	}

	// A second submit creates nothing new.
	rec = do(t, h, http.MethodPost, base+"/request/submit", nil)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("second submit: got %d", rec.Code)
	}
	criteria, _ = db.ListObjects(context.Background(), projectID, store.ObjectCriterion)
	if len(criteria) != 1 {
		t.Fatalf("criteria after second submit: got %d, want 1", len(criteria))
	}
}

// minimalPDF builds a one-page PDF whose page shows the given text.
func minimalPDF(t *testing.T, text string) []byte {
	t.Helper()
	content := "BT /F1 24 Tf 100 700 Td (" + text + ") Tj ET"
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [4 0 R] /Count 1 >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 5 0 R /Resources << /Font << /F1 3 0 R >> >> >>",
		"<< /Length " + itoa(len(content)) + " >>\nstream\n" + content + "\nendstream",
	}
	var b strings.Builder
	b.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objs)+1)
	for i, body := range objs {
		offsets[i+1] = b.Len()
		b.WriteString(itoa(i+1) + " 0 obj\n" + body + "\nendobj\n")
	}
	xref := b.Len()
	b.WriteString("xref\n0 6\n0000000000 65535 f \n")
	for i := 1; i <= 5; i++ {
		b.WriteString(pad10(offsets[i]) + " 00000 n \n")
	}
	b.WriteString("trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n" + itoa(xref) + "\n%%EOF\n")
	return []byte(b.String())
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func pad10(n int) string {
	s := itoa(n)
	for len(s) < 10 {
		s = "0" + s
	}
	return s
}

// P-31: sources are listed by their number (src-2 before src-10).
func TestP31_SourcesAreSortedByNumber(t *testing.T) {
	h, _, _ := newAPI(t)
	projectID := newProject(t, h, map[string]any{"template": "case-5"})

	got := decode[server.RequestView](t, do(t, h, http.MethodGet, "/api/projects/"+projectID+"/request", nil))
	for i, src := range got.Sources {
		if want := "src-" + itoa(i+1); src.Label != want {
			t.Fatalf("source %d: got %s, want %s (order %v)", i, src.Label, want, got.Sources)
		}
	}
}
