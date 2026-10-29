// Package case5 embeds the case-5 demo template (B-12).
//
// The real template lives in docs/demo/caso-5/plantilla/ and Eber
// copies it over this one; until then this is a test template with
// the same format: request.json plus one file per source.
package case5

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
)

//go:embed template
var templateFS embed.FS

// SourceFile is one source entry of request.json (B-12).
type SourceFile struct {
	Filename string `json:"filename"`
	Kind     string `json:"kind"` // file · calculation
}

// Template is the parsed request.json of the demo template.
type Template struct {
	Name        string       `json:"name"`
	Text        string       `json:"text"`
	Audience    string       `json:"audience"`
	Criteria    []string     `json:"criteria"`
	Premise     string       `json:"premise"`
	Exploratory bool         `json:"exploratory"`
	Sources     []SourceFile `json:"sources"`
}

// Load returns the template and the bytes of each source file, in
// the order request.json lists them.
func Load() (Template, map[string][]byte, error) {
	raw, err := templateFS.ReadFile("template/request.json")
	if err != nil {
		return Template{}, nil, fmt.Errorf("case5 template: %w", err)
	}
	var tpl Template
	if err := json.Unmarshal(raw, &tpl); err != nil {
		return Template{}, nil, fmt.Errorf("case5 template: %w", err)
	}
	files := make(map[string][]byte, len(tpl.Sources))
	for _, src := range tpl.Sources {
		data, err := fs.ReadFile(templateFS, "template/sources/"+src.Filename)
		if err != nil {
			return Template{}, nil, fmt.Errorf("case5 template source %s: %w", src.Filename, err)
		}
		files[src.Filename] = data
	}
	return tpl, files, nil
}
