package memory

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// The principal reads the person's material on demand with two tools
// (R-3, bloque-2 §3): read_source(source, span) and
// search_sources(text). Each read is recorded as delivered (via =
// read), and a figure or citation must be literal in a source read in
// the project (specification §3, «Precisiones del 30-oct»).

// maxSearchMatches caps the paragraphs one search returns.
// [EL DISEÑO NO LO DICE]: the design names no limit.
const maxSearchMatches = 20

// sourceCardLines is how many first lines a source card shows.
// [EL DISEÑO NO LO DICE]: «primeras líneas» without a number.
const sourceCardLines = 2

// sourceCard is the card of one source in the index (R-3): title,
// type, length and first lines, computed by the app.
func sourceCard(b body) string {
	text := b.str("text")
	title := b.str("filename")
	if title == "" {
		title = firstLine(text)
	}
	kind := b.str("kind")
	if mt := b.str("media_type"); mt != "" {
		kind += ", " + mt
	}
	paragraphs := graph.Blocks(text)
	var first []string
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimSpace(strings.TrimLeft(line, "# ")); line != "" {
			first = append(first, line)
		}
		if len(first) == sourceCardLines {
			break
		}
	}
	return fmt.Sprintf("%s · %s · %d paragraphs, %d words · first lines: %s",
		title, kind, len(paragraphs), len(strings.Fields(text)), strings.Join(first, " / "))
}

// parseSpan reads a paragraph range («3-5», «4») of n paragraphs; an
// empty span, «null» or a range it cannot read is the whole source.
func parseSpan(span string, n int) (from, to int) {
	from, to = 1, n
	span = strings.TrimSpace(span)
	if span == "" || span == "null" {
		return from, to
	}
	a, b, hasRange := strings.Cut(span, "-")
	x, err := strconv.Atoi(strings.TrimSpace(a))
	if err != nil {
		return 1, n
	}
	y := x
	if hasRange {
		if y, err = strconv.Atoi(strings.TrimSpace(b)); err != nil {
			return 1, n
		}
	}
	if x < 1 {
		x = 1
	}
	if y > n {
		y = n
	}
	if x > y {
		return 1, n
	}
	return x, y
}

func (m *Memory) sources(ctx context.Context, projectID string) ([]store.Object, error) {
	return m.db.ListObjects(ctx, projectID, store.ObjectSource)
}

func (m *Memory) recordRead(ctx context.Context, projectID, instanceID string, src store.Object) error {
	return m.db.RecordDelivery(ctx, store.Delivery{
		ProjectID: projectID, InstanceID: instanceID, Via: store.ViaRead,
		ObjectID: src.ID, Version: src.CurrentVersion,
	})
}

// ReadSource answers read_source: the paragraphs of one source in the
// span, numbered, literal. An unknown label answers with the labels
// that exist, for the model, and records nothing.
func (m *Memory) ReadSource(ctx context.Context, projectID, instanceID, label, span string) (string, error) {
	all, err := m.sources(ctx, projectID)
	if err != nil {
		return "", err
	}
	var labels []string
	for _, src := range all {
		labels = append(labels, src.Label)
		if src.Label != label {
			continue
		}
		paragraphs := graph.Blocks(decodeBody(src).str("text"))
		from, to := parseSpan(span, len(paragraphs))
		var sb strings.Builder
		fmt.Fprintf(&sb, "%s, paragraphs %d-%d of %d:\n", src.Label, from, to, len(paragraphs))
		for i := from; i <= to; i++ {
			fmt.Fprintf(&sb, "[%d] %s\n", i, paragraphs[i-1])
		}
		if err := m.recordRead(ctx, projectID, instanceID, src); err != nil {
			return "", err
		}
		return sb.String(), nil
	}
	if len(labels) == 0 {
		return "There are no sources in this project.", nil
	}
	return fmt.Sprintf("No source %s. The sources are: %s.", label, strings.Join(labels, ", ")), nil
}

// SearchSources answers search_sources: every paragraph of the
// project's sources that holds the text (case and accents as written,
// letter case ignored), with its source and number. Each source with a
// match counts as read.
func (m *Memory) SearchSources(ctx context.Context, projectID, instanceID, text string) (string, error) {
	needle := strings.ToLower(strings.TrimSpace(text))
	if utf8.RuneCountInString(needle) == 0 {
		return "Search for some text.", nil
	}
	all, err := m.sources(ctx, projectID)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	matches := 0
	for _, src := range all {
		found := false
		for i, p := range graph.Blocks(decodeBody(src).str("text")) {
			if !strings.Contains(strings.ToLower(p), needle) {
				continue
			}
			if matches == maxSearchMatches {
				break
			}
			fmt.Fprintf(&sb, "%s [%d] %s\n", src.Label, i+1, p)
			matches++
			found = true
		}
		if found {
			if err := m.recordRead(ctx, projectID, instanceID, src); err != nil {
				return "", err
			}
		}
	}
	if matches == 0 {
		return "No paragraph of the sources holds that text.", nil
	}
	if matches == maxSearchMatches {
		fmt.Fprintf(&sb, "(first %d matches; search for something narrower to see others)\n", maxSearchMatches)
	}
	return sb.String(), nil
}
