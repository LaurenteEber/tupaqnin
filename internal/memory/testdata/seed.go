// Package testdata seeds the project the backlog 0.9 tests use:
// 1 request with audience, 2 criteria with their cases, 1 premise,
// 3 sections, 6 decisions of which 2 are critical, 1 open card and
// 2 human messages, one of them unanswered.
package testdata

import (
	"context"
	"fmt"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

const (
	RequestText       = "Quiero un informe breve para decidir si regamos de noche."
	Audience          = "La junta de vecinos del sector"
	PremiseText       = "El caudal nocturno alcanza para todo el sector."
	AnsweredMessage   = "¿Puedes partir por el alcance?"
	AnsweredResponse  = "del agente: parto por el alcance y dejo las normas para después."
	UnansweredMessage = "Revisa también el costo para la junta, en cifras."
	DeferredWhat      = "Comparar con el riego por goteo"
)

var CriteriaTexts = []string{
	"El informe compara costo y riesgo con cifras.",
	"Toda afirmación sobre normas cita la ordenanza.",
}

// Seeded reports what Seed created, for the tests.
type Seeded struct {
	RequestText       string
	Audience          string
	PremiseText       string
	CriteriaTexts     []string
	AnsweredMessage   string
	UnansweredMessage string
	DeferredWhat      string
	SectionLabels     []string
	DecisionLabels    []string
	CriticalLabels    []string
	InternalIDs       []string
}

func create(ctx context.Context, db *store.DB, project string, t store.ObjectType, state string, body any) (store.Object, error) {
	var obj store.Object
	err := db.Write(ctx, project, func(tx *store.Tx) error {
		var err error
		obj, err = tx.CreateObject(t, state, body,
			store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		return err
	})
	return obj, err
}

func addEdge(ctx context.Context, db *store.DB, project string, t store.EdgeType, from, to string) error {
	return db.Write(ctx, project, func(tx *store.Tx) error {
		_, err := tx.AddEdge(t, from, to, "",
			store.EventInput{Type: store.EventEdge, Actor: "agent", AgentRole: "principal"})
		return err
	})
}

// Seed creates the seeded project in an existing empty database.
func Seed(ctx context.Context, db *store.DB, project string) (Seeded, error) {
	out := Seeded{
		RequestText: RequestText, Audience: Audience, PremiseText: PremiseText,
		CriteriaTexts: CriteriaTexts, AnsweredMessage: AnsweredMessage,
		UnansweredMessage: UnansweredMessage, DeferredWhat: DeferredWhat,
	}
	if _, err := db.Writer.ExecContext(ctx,
		`INSERT OR IGNORE INTO project (id, created_at) VALUES (?, '2026-10-28T00:00:00.000Z')`, project); err != nil {
		return out, fmt.Errorf("seed project: %w", err)
	}
	track := func(obj store.Object, err error) (store.Object, error) {
		if err != nil {
			return obj, err
		}
		out.InternalIDs = append(out.InternalIDs, obj.ID)
		return obj, nil
	}

	req, err := track(create(ctx, db, project, store.ObjectRequest, "", map[string]any{
		"text": RequestText, "audience": Audience,
	}))
	if err != nil {
		return out, err
	}
	var critIDs []string
	for i, text := range CriteriaTexts {
		crit, err := track(create(ctx, db, project, store.ObjectCriterion, "current", map[string]any{
			"text": text, "request_id": req.ID,
		}))
		if err != nil {
			return out, err
		}
		critIDs = append(critIDs, crit.ID)
		cases := []struct {
			text   string
			counts bool
		}{
			{fmt.Sprintf("Cuenta como cumplir el criterio %d: una comparación con cifras.", i+1), true},
			{fmt.Sprintf("No cuenta para el criterio %d: una descripción sin cifras.", i+1), false},
		}
		for _, c := range cases {
			if _, err := track(create(ctx, db, project, store.ObjectUnderstandingCase, "unconfirmed", map[string]any{
				"text": c.text, "counts": c.counts, "criterion_id": crit.ID,
			})); err != nil {
				return out, err
			}
		}
	}
	if _, err := track(create(ctx, db, project, store.ObjectPremise, "current", map[string]any{
		"text": PremiseText, "request_id": req.ID, "exploratory": false,
	})); err != nil {
		return out, err
	}

	roles := []string{"conclusion", "recommendation", "other"}
	var secIDs []string
	for i, role := range roles {
		sec, err := track(create(ctx, db, project, store.ObjectSection, "draft", map[string]any{
			"text": fmt.Sprintf("# Sección %d\n\nPrimera línea de la sección %d, con lo esencial.", i+1, i+1),
			"role": map[string]any{"value": role, "state": "proposed"}, "open_changes": []any{},
		}))
		if err != nil {
			return out, err
		}
		secIDs = append(secIDs, sec.ID)
		out.SectionLabels = append(out.SectionLabels, sec.Label)
	}

	type decSpec struct {
		statement string
		origin    string
		state     string
		marked    bool
		usedIn    []int
	}
	specs := []decSpec{
		{"Regar entre 22:00 y 05:00.", "human", "human", true, []int{0}},
		{"Usar el caudal medido en marzo.", "human", "human", false, []int{2}},
		{"Regar por sectores en dos tandas.", "agent", "agent", false, []int{0, 1}},
		{"Abrir con el resumen de costo.", "agent", "agent", false, []int{2}},
		{"Citar la ordenanza en cada afirmación de normas.", "agent", "agent", false, nil},
		{"El caudal alcanza sin turnos extra.", "agent", "agent", false, nil},
	}
	var decIDs []string
	for i, spec := range specs {
		dec, err := track(create(ctx, db, project, store.ObjectDecision, spec.state, map[string]any{
			"statement": spec.statement, "instead_of": "La alternativa descartada.",
			"would_choose_if": "Cambia el caudal medido.", "assumption": i == 5,
			"origin": spec.origin, "human_marked_critical": spec.marked,
		}))
		if err != nil {
			return out, err
		}
		decIDs = append(decIDs, dec.ID)
		out.DecisionLabels = append(out.DecisionLabels, dec.Label)
		for _, secIdx := range spec.usedIn {
			if err := addEdge(ctx, db, project, store.EdgeUsedIn, dec.ID, secIDs[secIdx]); err != nil {
				return out, err
			}
		}
	}
	// Critical: dec-1 (marked by the human) and dec-3 (used in two
	// sections, one of them a conclusion).
	out.CriticalLabels = []string{out.DecisionLabels[0], out.DecisionLabels[2]}

	if _, err := track(create(ctx, db, project, store.ObjectCard, "open", map[string]any{
		"decision_id": decIDs[0], "chosen_option": "", "reason": "",
	})); err != nil {
		return out, err
	}

	err = db.Write(ctx, project, func(tx *store.Tx) error {
		if _, err := tx.AppendEvent(store.EventInput{
			Type: store.EventHumanMessage, Actor: "human",
			Data: map[string]any{"text": AnsweredMessage, "response": AnsweredResponse},
		}); err != nil {
			return err
		}
		_, err := tx.AppendEvent(store.EventInput{
			Type: store.EventHumanMessage, Actor: "human",
			Data: map[string]any{"text": UnansweredMessage},
		})
		return err
	})
	if err != nil {
		return out, err
	}
	_ = critIDs
	return out, nil
}
