package cycle

import (
	"context"
	"errors"
	"strings"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Sections by hand (K-17, precised 30-oct by P-49; specification §3).
var (
	ErrTitleRequired = errors.New("title_required")
	ErrFocusTooLong  = errors.New("focus_too_long")
	ErrInvalidRole   = errors.New("invalid_role")
	ErrInvalidOrder  = errors.New("invalid_order")
	ErrNotASection   = errors.New("section_not_found")
)

// FocusMaxWords is the length of «qué debe abordar» (K-17) [inicial].
const FocusMaxWords = 60

func checkFocus(focus string) error {
	if len(strings.Fields(focus)) > FocusMaxWords {
		return ErrFocusTooLong
	}
	return nil
}

// CreateSection creates your section: a title (required), the «aquí se
// concluye» mark (role "conclusion", born confirmed because you chose
// it; empty, no role is set and the principal proposes it, K-2) and
// «qué debe abordar», yours and in literal. It is born empty, in draft
// and last.
func CreateSection(ctx context.Context, db *store.DB, projectID, title, role, focus string) (store.Object, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return store.Object{}, ErrTitleRequired
	}
	if role != "" && role != "conclusion" {
		return store.Object{}, ErrInvalidRole
	}
	if err := checkFocus(focus); err != nil {
		return store.Object{}, err
	}
	roleBody := map[string]any{"value": "", "state": ""}
	if role != "" {
		roleBody = map[string]any{"value": role, "state": "confirmed"}
	}
	var obj store.Object
	err := db.Write(ctx, projectID, func(tx *store.Tx) error {
		position, err := graph.NextSectionPosition(ctx, tx, projectID)
		if err != nil {
			return err
		}
		obj, err = tx.CreateObject(store.ObjectSection, "draft", map[string]any{
			"title": title, "focus": focus, "position": position, "text": "",
			"role": roleBody, "open_changes": []any{},
		}, store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		if err != nil || role == "" {
			return err
		}
		_, err = tx.AppendEvent(store.EventInput{Type: store.EventRoleChanged, Actor: "human", ObjectID: obj.ID,
			Data: map[string]any{"label": obj.Label, "from": "", "to": role, "state": "confirmed"}})
		return err
	})
	return obj, err
}

// ChangeSectionDetails changes the title and «qué debe abordar» of any
// section (P-49); nil leaves a field as it is. It records one
// section_details_changed with every field that changed. It never
// changes the state: title and focus stay out of the fingerprint and
// of what is signed.
func ChangeSectionDetails(ctx context.Context, db *store.DB, projectID, label string, title, focus *string) error {
	if title != nil {
		t := strings.TrimSpace(*title)
		if t == "" {
			return ErrTitleRequired
		}
		title = &t
	}
	if focus != nil {
		if err := checkFocus(*focus); err != nil {
			return err
		}
	}
	return db.Write(ctx, projectID, func(tx *store.Tx) error {
		obj, err := tx.GetObject(ctx, projectID, label)
		if err != nil || obj.Type != store.ObjectSection {
			return ErrNotASection
		}
		b := bodyOf(obj)
		var changes []any
		for _, f := range []struct {
			name string
			to   *string
		}{{"title", title}, {"focus", focus}} {
			if f.to == nil || bodyStr(b, f.name) == *f.to {
				continue
			}
			changes = append(changes, map[string]any{"field": f.name, "from": bodyStr(b, f.name), "to": *f.to})
			b[f.name] = *f.to
		}
		if len(changes) == 0 {
			return nil
		}
		return writeDetails(tx, obj, b, changes)
	})
}

// ReorderSections sets the order of the sections (P-49): labels must
// name every section once. It records one section_details_changed per
// section whose position changes.
func ReorderSections(ctx context.Context, db *store.DB, projectID string, labels []string) error {
	return db.Write(ctx, projectID, func(tx *store.Tx) error {
		sections, err := tx.ListObjects(ctx, projectID, store.ObjectSection)
		if err != nil {
			return err
		}
		byLabel := make(map[string]store.Object, len(sections))
		for _, s := range sections {
			byLabel[s.Label] = s
		}
		if len(labels) != len(sections) {
			return ErrInvalidOrder
		}
		seen := make(map[string]bool, len(labels))
		for _, l := range labels {
			if _, ok := byLabel[l]; !ok || seen[l] {
				return ErrInvalidOrder
			}
			seen[l] = true
		}
		for i, l := range labels {
			obj := byLabel[l]
			from := graph.SectionPosition(obj)
			if from == i+1 {
				continue
			}
			b := bodyOf(obj)
			b["position"] = i + 1
			if err := writeDetails(tx, obj, b, []any{map[string]any{"field": "position", "from": from, "to": i + 1}}); err != nil {
				return err
			}
		}
		return nil
	})
}

func writeDetails(tx *store.Tx, obj store.Object, b map[string]any, changes []any) error {
	_, err := tx.UpdateObject(obj.ID, obj.CurrentVersion, obj.State, b, store.EventInput{
		Type: store.EventSectionDetailsChanged, Actor: "human",
		Data: map[string]any{"label": obj.Label, "changes": changes},
	})
	if errors.Is(err, store.ErrNoChange) {
		return nil
	}
	return err
}
