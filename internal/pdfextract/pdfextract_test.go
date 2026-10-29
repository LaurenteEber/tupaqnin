package pdfextract

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestHelperProcess is not a test: it is the child process the tests
// spawn in place of the real binary. TQ_PDF_HELPER=child runs the
// real child side; TQ_PDF_HELPER=hang simulates a parser stuck in a
// loop (it never answers).
func TestHelperProcess(t *testing.T) {
	switch os.Getenv("TQ_PDF_HELPER") {
	case "child":
		os.Exit(RunChild(os.Stdin, os.Stdout, os.Stderr))
	case "hang":
		time.Sleep(60 * time.Second)
		os.Exit(0)
	}
}

func useHelper(t *testing.T, mode string) {
	t.Helper()
	prev := ChildCommand
	ChildCommand = func(ctx context.Context) (*exec.Cmd, error) {
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestHelperProcess", "--")
		cmd.Env = append(os.Environ(), "TQ_PDF_HELPER="+mode)
		return cmd, nil
	}
	t.Cleanup(func() { ChildCommand = prev })
}

// makePDF builds a minimal valid PDF with n pages. With text, every
// page shows "Hello page"; without, its content stream is empty.
func makePDF(t *testing.T, n int, text bool) []byte {
	t.Helper()
	type obj struct {
		num  int
		body string
	}
	objs := []obj{
		{1, "<< /Type /Catalog /Pages 2 0 R >>"},
		{3, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"},
	}
	kids := make([]string, 0, n)
	for i := 0; i < n; i++ {
		pageNum := 4 + 2*i
		contentNum := pageNum + 1
		kids = append(kids, fmt.Sprintf("%d 0 R", pageNum))
		content := ""
		if text {
			content = "BT /F1 24 Tf 100 700 Td (Hello page) Tj ET"
		}
		objs = append(objs,
			obj{pageNum, fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents %d 0 R /Resources << /Font << /F1 3 0 R >> >> >>", contentNum)},
			obj{contentNum, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content)},
		)
	}
	objs = append(objs, obj{2, fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), n)})

	var b strings.Builder
	b.WriteString("%PDF-1.4\n")
	offsets := map[int]int{}
	maxNum := 0
	// Objects must be written in numeric order for the xref table.
	for num := 1; num <= 3+2*n; num++ {
		for _, o := range objs {
			if o.num != num {
				continue
			}
			offsets[num] = b.Len()
			fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", o.num, o.body)
			if num > maxNum {
				maxNum = num
			}
		}
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n", maxNum+1)
	b.WriteString("0000000000 65535 f \n")
	for num := 1; num <= maxNum; num++ {
		fmt.Fprintf(&b, "%010d 00000 n \n", offsets[num])
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", maxNum+1, xref)
	return []byte(b.String())
}

func TestB9_ValidPDFExtractsText(t *testing.T) {
	useHelper(t, "child")
	got, err := Extract(context.Background(), makePDF(t, 1, true))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if !strings.Contains(got, "Hello page") {
		t.Fatalf("Extract: got %q, want it to contain %q", got, "Hello page")
	}
}

func TestB9_TruncatedPDFRejected(t *testing.T) {
	useHelper(t, "child")
	valid := makePDF(t, 1, true)
	truncated := valid[:len(valid)/2]
	_, err := Extract(context.Background(), truncated)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("Extract(truncated): got %v, want ErrInvalid", err)
	}
	if code := Code(err); code != "source_pdf_invalid" {
		t.Fatalf("Code: got %q, want source_pdf_invalid", code)
	}
}

func TestB9_PDFWithoutTextRejected(t *testing.T) {
	useHelper(t, "child")
	_, err := Extract(context.Background(), makePDF(t, 1, false))
	if !errors.Is(err, ErrNoText) {
		t.Fatalf("Extract(no text): got %v, want ErrNoText", err)
	}
	if code := Code(err); code != "source_pdf_no_text" {
		t.Fatalf("Code: got %q, want source_pdf_no_text", code)
	}
}

func TestB9_PDFOver100PagesRejectedBeforeExtraction(t *testing.T) {
	useHelper(t, "child")
	// 101 pages without text: the page count is checked before
	// extracting, so the answer is ErrTooMany, not ErrNoText.
	_, err := Extract(context.Background(), makePDF(t, MaxPages+1, false))
	if !errors.Is(err, ErrTooMany) {
		t.Fatalf("Extract(101 pages): got %v, want ErrTooMany", err)
	}
	if code := Code(err); code != "source_pdf_too_many_pages" {
		t.Fatalf("Code: got %q, want source_pdf_too_many_pages", code)
	}
}

func TestB9_HangingExtractionIsKilledAndRejected(t *testing.T) {
	useHelper(t, "hang")
	start := time.Now()
	_, err := extract(context.Background(), makePDF(t, 1, true), 300*time.Millisecond)
	elapsed := time.Since(start)
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("Extract(hanging): got %v, want ErrTimeout", err)
	}
	if code := Code(err); code != "source_pdf_timeout" {
		t.Fatalf("Code: got %q, want source_pdf_timeout", code)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("Extract(hanging): took %v; the child was not killed at the deadline", elapsed)
	}
}

var _ io.Reader = strings.NewReader("")
