package server_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Backlog-r2 2.6 through the routes (P6, 5b): the report carries the
// open audience questions of its section; «sí» leaves one addressed and
// «no» declined with audience_question_declined; an answered one is no
// longer in the report and cannot be answered again.
func TestK1_ReportCarriesTheOpenAudienceQuestionsAndTheirAnswersRoute(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := newProject(t, h, nil)
	base := "/api/projects/" + projectID
	seedSignSection(t, db, projectID, "Comparamos solo estudios con el mismo filtro.", false)
	err := db.Write(context.Background(), projectID, func(tx *store.Tx) error {
		sec, err := tx.GetObject(context.Background(), projectID, "sec-1")
		if err != nil {
			return err
		}
		judge := store.EventInput{Type: store.EventStateChanged, Actor: "agent", AgentRole: "judge"}
		for _, q := range []map[string]any{
			{"question": "¿Por qué el mismo filtro?", "section_id": sec.ID},
			{"question": "El agua de pozo.", "section_id": sec.ID, "not_covered": true},
		} {
			aq, err := tx.CreateObject(store.ObjectAudienceQuestion, "open", q, judge)
			if err != nil {
				return err
			}
			if _, err := tx.AddEdge(store.EdgeAbout, aq.ID, sec.ID, "", nil, store.EventInput{Type: store.EventEdge, Actor: "agent"}); err != nil {
				return err
			}
		}
		_, err = tx.AppendEvent(store.EventInput{Type: store.EventStopReport, Actor: "agent", AgentRole: "principal",
			Data: cycle.StopReport{StopReason: cycle.StopSectionReady, Section: "sec-1", ProposeSignoff: true}})
		return err
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	view := decode[server.StopReportView](t, do(t, h, http.MethodGet, base+"/stop-report", nil))
	if len(view.AudienceQuestions) != 2 || view.AudienceQuestions[0].Question != "¿Por qué el mismo filtro?" ||
		view.AudienceQuestions[0].NotCovered || !view.AudienceQuestions[1].NotCovered || view.DefenseFailed {
		t.Fatalf("report audience questions: %+v (defense failed %v)", view.AudienceQuestions, view.DefenseFailed)
	}

	answer := func(label, a string) int {
		t.Helper()
		return do(t, h, http.MethodPost, base+"/audience-questions/"+label+"/answer", map[string]string{"answer": a}).Code
	}
	if code := answer("aq-1", "maybe"); code != http.StatusBadRequest {
		t.Fatalf("answer «maybe»: %d", code)
	}
	if code := answer("aq-9", "yes"); code != http.StatusNotFound {
		t.Fatalf("answer on aq-9: %d", code)
	}
	rec := do(t, h, http.MethodPost, base+"/audience-questions/aq-1/answer", map[string]string{"answer": "yes"})
	if got := decode[server.AnswerAudienceQuestionResponse](t, rec); rec.Code != http.StatusOK || got.State != "addressed" {
		t.Fatalf("answer yes: %d %+v", rec.Code, got)
	}
	if code := answer("aq-2", "no"); code != http.StatusOK {
		t.Fatalf("answer no: %d", code)
	}
	if code := answer("aq-1", "no"); code != http.StatusConflict {
		t.Fatalf("second answer: %d", code)
	}
	if n := len(eventsOf(t, db, projectID, store.EventAudienceQuestionDeclined)); n != 1 {
		t.Fatalf("audience_question_declined events: %d, want 1", n)
	}
	if n := len(eventsOf(t, db, projectID, store.EventHumanMessage)); n != 0 {
		t.Fatalf("answering opened a turn: %d human messages", n)
	}
	view = decode[server.StopReportView](t, do(t, h, http.MethodGet, base+"/stop-report", nil))
	if len(view.AudienceQuestions) != 0 {
		t.Fatalf("answered questions still in the report: %+v", view.AudienceQuestions)
	}
}
