package graph

import (
	"context"
	"strings"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// ExportMarkdown is the document as one Markdown file (backlog-r2 2.8,
// K-10 b, B-47): the sections in position order (K-17), each with its
// current title as a heading and its current text, without «qué debe
// abordar», ids or history; under the title of each section that is not
// signed, the quote line notSigned (the catalog's mark, B-47).
// [EL DISEÑO NO LO DICE]: only the state signed counts as signed; a
// section changed_since_signoff is marked, because its current text is
// not what was signed. The title is a level-2 heading and the file has
// no title of its own; a section without text keeps its heading.
func ExportMarkdown(ctx context.Context, q store.Querier, projectID, notSigned string) (string, error) {
	sections, err := OrderedSections(ctx, q, projectID)
	if err != nil {
		return "", err
	}
	blocks := make([]string, 0, len(sections))
	for _, sec := range sections {
		title, _ := bodyValue(sec, "title").(string)
		title = strings.Join(strings.Fields(title), " ")
		parts := []string{"## " + title}
		if sec.State != "signed" {
			parts = append(parts, "> "+notSigned)
		}
		if text, _ := bodyValue(sec, "text").(string); strings.TrimSpace(text) != "" {
			parts = append(parts, strings.Trim(text, "\n"))
		}
		blocks = append(blocks, strings.Join(parts, "\n\n")+"\n")
	}
	return strings.Join(blocks, "\n"), nil
}
