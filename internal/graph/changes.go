package graph

import (
	"encoding/json"

	"github.com/LaurenteEber/tupaqnin/internal/fingerprint"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Usage is what a using edge keeps of what it used (B-45, A-1): the
// version it saw and its content stamp. Only the content counts as a
// change, never the state: of a decision, its statement, instead_of and
// would_choose_if, or that it became superseded or revoked; of a
// figure, its value; of a source, its fingerprint (specification §4).
type Usage struct {
	Version int    `json:"version"`
	Content string `json:"content"`
}

// UsageOf is the Usage a using edge stores for obj: the decision of a
// used_in edge, the figure of a covers edge, the source of a uses edge.
func UsageOf(obj store.Object) Usage {
	return Usage{Version: obj.CurrentVersion, Content: UsageStamp(obj)}
}

// UsageStamp is the content of obj that counts as a change (B-45).
func UsageStamp(obj store.Object) string {
	var b map[string]any
	_ = json.Unmarshal(obj.Body, &b)
	str := func(k string) string { s, _ := b[k].(string); return s }
	switch obj.Type {
	case store.ObjectDecision:
		ended := obj.State == "superseded" || obj.State == "revoked"
		raw, _ := json.Marshal([]any{str("statement"), str("instead_of"), str("would_choose_if"), ended})
		return fingerprint.Source(string(raw))
	case store.ObjectFigure:
		return str("value")
	case store.ObjectSource:
		if fp := str("fingerprint"); fp != "" {
			return fp
		}
		return fingerprint.Source(str("text"))
	}
	return ""
}

// UsedAnother reports whether a using edge saw another content than
// obj's current one. An edge without data (older than migration 0004)
// counts as another: it only matters in a transaction that changed obj.
// [EL DISEÑO NO LO DICE]: an edge without what it used; it is read as
// written with the previous version («en la duda, salta»).
func UsedAnother(e store.Edge, obj store.Object) bool {
	if len(e.Data) == 0 {
		return true
	}
	var u Usage
	if err := json.Unmarshal(e.Data, &u); err != nil {
		return true
	}
	return u.Content != UsageStamp(obj)
}

// OpenChange is one change in a section's list (A-1): its label
// (chg-n), the kind of object that changed and that object's label.
// Internal ids never enter the list (rule 7 of AGENTS).
type OpenChange struct {
	Label string `json:"label"`
	Kind  string `json:"kind"`
	About string `json:"about"`
}

// OpenChanges reads a section's list of open changes (specification
// §1.1, section.open_changes). Changes are not pendings; they block the
// signoff in every state (estados §1, E-4).
func OpenChanges(section store.Object) []OpenChange {
	var b struct {
		OpenChanges []json.RawMessage `json:"open_changes"`
	}
	_ = json.Unmarshal(section.Body, &b)
	out := []OpenChange{}
	for _, raw := range b.OpenChanges {
		var c OpenChange
		if json.Unmarshal(raw, &c) == nil && c.Label != "" {
			out = append(out, c)
		}
	}
	return out
}
