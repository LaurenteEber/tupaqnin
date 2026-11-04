package graph

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// B-73, B-77, B-99: every origin Go gives a proposal has its text in both
// catalogs. It reads the constants from their block, so a new one is covered.
func TestB99_EveryProposalOriginHasATextInBothCatalogs(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "proposal.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	var origins []string
	for _, decl := range file.Decls {
		block, ok := decl.(*ast.GenDecl)
		if !ok || block.Tok != token.CONST || !strings.HasPrefix(block.Doc.Text(), "Why a proposal was born") {
			continue
		}
		for _, spec := range block.Specs {
			for _, value := range spec.(*ast.ValueSpec).Values {
				origin, err := strconv.Unquote(value.(*ast.BasicLit).Value)
				if err != nil {
					t.Fatal(err)
				}
				origins = append(origins, origin)
			}
		}
	}
	if len(origins) < 10 {
		t.Fatalf("origins read from proposal.go: %v", origins)
	}
	for _, lang := range []string{"en", "es"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "web", "src", "i18n", lang+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var catalog map[string]string
		if err := json.Unmarshal(raw, &catalog); err != nil {
			t.Fatal(err)
		}
		for _, origin := range origins {
			if strings.TrimSpace(catalog["proposal.origin."+origin]) == "" {
				t.Errorf("%s: no text for proposal.origin.%s", lang, origin)
			}
		}
	}
}
