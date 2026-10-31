package memory_test

import (
	"context"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/memory"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Estados §9, K-1 (backlog-r2 2.6): an audience question you answered
// «sí» is your instruction in the principal's next startup; one not
// answered is not; neither carries an internal id (rule 7).
func TestK1_StartupCarriesTheAddressedAudienceQuestionOnly(t *testing.T) {
	db, _ := seeded(t)
	ctx := context.Background()
	sections, err := db.ListObjects(ctx, "p1", store.ObjectSection)
	if err != nil || len(sections) == 0 {
		t.Fatalf("sections: %v %v", sections, err)
	}
	sec := sections[0]
	const addressed, open = "¿Por qué solo filtros finos?", "¿Vale para el agua de pozo?"
	var openLabel string
	err = db.Write(ctx, "p1", func(tx *store.Tx) error {
		judge := store.EventInput{Type: store.EventStateChanged, Actor: "agent", AgentRole: "judge"}
		for _, q := range []struct{ text, state string }{{addressed, "addressed"}, {open, "open"}} {
			aq, err := tx.CreateObject(store.ObjectAudienceQuestion, q.state,
				map[string]any{"question": q.text, "section_id": sec.ID}, judge)
			if err != nil {
				return err
			}
			if q.state == "open" {
				openLabel = aq.Label
			}
			if _, err := tx.AddEdge(store.EdgeAbout, aq.ID, sec.ID, "", nil, store.EventInput{Type: store.EventEdge, Actor: "agent"}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed audience questions: %v", err)
	}
	mem := memory.New(db)
	brief, err := mem.Startup(ctx, "p1", memory.StartupNewSession, 1048576, fakeCount)
	if err != nil {
		t.Fatalf("Startup: %v", err)
	}
	text := briefText(brief)
	if !strings.Contains(text, addressed) || !strings.Contains(text, "Instruction of the person") {
		t.Fatalf("the startup lacks the addressed question:\n%s", text)
	}
	if strings.Contains(text, open) {
		t.Fatal("the startup carries a question you did not answer")
	}
	item, err := mem.Read(ctx, "p1", brief.InstanceID, openLabel)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if strings.Contains(text, sec.ID) || strings.Contains(item.Text, sec.ID) || !strings.Contains(item.Text, sec.Label) {
		t.Fatalf("an internal id reaches the principal (rule 7): %q", item.Text)
	}
}
