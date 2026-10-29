// Package pdfextract extracts plain text from a PDF (B-9).
//
// A PDF is hostile input and the parser cannot be trusted to finish:
// a goroutine stuck in a loop cannot be killed in Go, so a timeout on
// a goroutine would leak CPU and memory. The extraction therefore
// runs in a child process (this same binary, re-invoked with the
// hidden Subcommand), the file is passed on stdin, and the parent
// kills the child when the deadline passes. The child counts the
// pages before extracting and refuses more than MaxPages.
package pdfextract

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"
)

// Subcommand is the hidden subcommand that turns the binary into the
// extraction child. It is not part of the public CLI.
const Subcommand = "pdf-extract"

// MaxPages is the page limit for one PDF (B-9, [inicial]).
const MaxPages = 100

// Timeout is the extraction deadline (B-9, [inicial]): the parent
// kills the child when it passes.
const Timeout = 10 * time.Second

// MaxInput bounds what the child reads from stdin; the API rejects
// larger sources before this (specification §6: source ≤ 5 MB).
const MaxInput = 5 << 20

// Error codes. Each one is a catalog key suffix: the API returns the
// code and the web catalog (T-1) renders source.error.<code>. The raw
// parser error never leaves the child (AGENTS.md rule 9).
var (
	ErrInvalid = errors.New("pdfextract: invalid PDF")
	ErrNoText  = errors.New("pdfextract: PDF has no text")
	ErrTooMany = errors.New("pdfextract: PDF has too many pages")
	ErrTimeout = errors.New("pdfextract: extraction timed out")
	ErrFailed  = errors.New("pdfextract: extraction failed")
)

// Code returns the catalog code for an extraction error.
func Code(err error) string {
	switch {
	case errors.Is(err, ErrNoText):
		return "source_pdf_no_text"
	case errors.Is(err, ErrTooMany):
		return "source_pdf_too_many_pages"
	case errors.Is(err, ErrTimeout):
		return "source_pdf_timeout"
	case errors.Is(err, ErrInvalid):
		return "source_pdf_invalid"
	default:
		return "source_pdf_failed"
	}
}

// ChildCommand builds the child command. It is a variable so tests
// in other packages can substitute a helper process (including one
// that hangs); production always uses the real binary.
var ChildCommand = func(ctx context.Context) (*exec.Cmd, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return exec.CommandContext(ctx, exe, Subcommand), nil
}

// Extract returns the plain text of the PDF in data. It runs the
// extraction in a child process and kills it at Timeout.
func Extract(ctx context.Context, data []byte) (string, error) {
	return extract(ctx, data, Timeout)
}

func extract(ctx context.Context, data []byte, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd, err := ChildCommand(ctx)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrFailed, err)
	}
	cmd.Stdin = bytes.NewReader(data)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	if ctx.Err() != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		// The deadline passed: CommandContext has killed the child.
		return "", ErrTimeout
	}
	if runErr != nil {
		switch strings.TrimSpace(stderr.String()) {
		case "source_pdf_no_text":
			return "", ErrNoText
		case "source_pdf_too_many_pages":
			return "", ErrTooMany
		case "source_pdf_invalid":
			return "", ErrInvalid
		default:
			return "", fmt.Errorf("%w: %v", ErrFailed, runErr)
		}
	}
	return stdout.String(), nil
}

// RunChild is the child side: it reads the PDF from stdin, counts its
// pages before extracting, and writes the text to stdout. On failure
// it writes only the catalog code to stderr and returns a non-zero
// exit code; the parser error itself stays in the child.
func RunChild(stdin io.Reader, stdout, stderr io.Writer) int {
	data, err := io.ReadAll(io.LimitReader(stdin, MaxInput+1))
	if err != nil || len(data) > MaxInput {
		fmt.Fprintln(stderr, "source_pdf_invalid")
		return 2
	}
	text, err := extractBytes(data)
	if err != nil {
		fmt.Fprintln(stderr, Code(err))
		return 2
	}
	if _, err := io.WriteString(stdout, text); err != nil {
		return 2
	}
	return 0
}

// extractBytes parses data with the PDF library. Page count is
// checked before any page is extracted (B-9).
func extractBytes(data []byte) (string, error) {
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", ErrInvalid
	}
	if r.NumPage() > MaxPages {
		return "", ErrTooMany
	}
	plain, err := r.GetPlainText()
	if err != nil {
		return "", ErrInvalid
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(plain); err != nil {
		return "", ErrInvalid
	}
	if strings.TrimSpace(buf.String()) == "" {
		return "", ErrNoText
	}
	return buf.String(), nil
}
