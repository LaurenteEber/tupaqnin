package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Shapes for P4 (backlog-r1 1.4): a card reads its decision, why it is
// critical and the sections that use it (pantallas.md, P4).
type CardView struct {
	Label         string   `json:"label"`
	State         string   `json:"state"`
	Decision      string   `json:"decision"`
	Statement     string   `json:"statement"`
	InsteadOf     string   `json:"instead_of"`
	WouldChooseIf string   `json:"would_choose_if"`
	Reasons       []string     `json:"reasons"`
	Sections      []SectionRef `json:"sections"`
}

// SectionRef names a section on screen by its title (P-36, B-18); the
// label is only for the page to navigate, never shown.
type SectionRef struct {
	Label string `json:"label"`
	Title string `json:"title"`
}

// TodoView is what is yours to do (estados §1); in slice 1, the open
// cards.
type TodoView struct {
	Cards []CardView `json:"cards"`
}

type AnswerCardRequest struct {
	Option string `json:"option"` // keep · other
	Reason string `json:"reason"`
}

type AnswerCardResponse struct {
	Card        string `json:"card"`
	Decision    string `json:"decision"`
	NewDecision string `json:"new_decision,omitempty"`
}

func (s *Server) cardView(r *http.Request, projectID string, card store.Object) (CardView, error) {
	ctx := r.Context()
	view := CardView{Label: card.Label, State: card.State, Reasons: []string{}, Sections: []SectionRef{}}
	dec, err := s.db.GetObject(ctx, projectID, str(bodyMap(card), "decision_id"))
	if err != nil {
		return view, err
	}
	b := bodyMap(dec)
	view.Decision, view.Statement = dec.Label, str(b, "statement")
	view.InsteadOf, view.WouldChooseIf = str(b, "instead_of"), str(b, "would_choose_if")
	// Criticality is computed when reading (T-5).
	_, reasons, err := graph.Critical(ctx, s.db, projectID, dec.Label)
	if err != nil {
		return view, err
	}
	for _, reason := range reasons {
		view.Reasons = append(view.Reasons, string(reason))
	}
	edges, err := s.db.EdgesFrom(ctx, projectID, dec.ID)
	if err != nil {
		return view, err
	}
	for _, e := range edges {
		if e.Type != store.EdgeUsedIn {
			continue
		}
		if sec, err := s.db.GetObject(ctx, projectID, e.ToID); err == nil {
			view.Sections = append(view.Sections, SectionRef{Label: sec.Label, Title: str(bodyMap(sec), "title")})
		}
	}
	return view, nil
}

// GET /api/projects/{projectID}/cards/{label}
func (s *Server) getCard(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	card, err := s.db.GetObject(r.Context(), projectID, r.PathValue("label"))
	if err != nil || card.Type != store.ObjectCard {
		writeError(w, http.StatusNotFound, "card_not_found")
		return
	}
	view, err := s.cardView(r, projectID, card)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// GET /api/projects/{projectID}/todo: the open cards.
func (s *Server) getTodo(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	cards, err := s.db.ListObjects(r.Context(), projectID, store.ObjectCard)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	todo := TodoView{Cards: []CardView{}}
	for _, card := range cards {
		if card.State != "open" {
			continue
		}
		view, err := s.cardView(r, projectID, card)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
		todo.Cards = append(todo.Cards, view)
	}
	writeJSON(w, http.StatusOK, todo)
}

// POST /api/projects/{projectID}/cards/{label}/answer with
// {option: keep · other, reason}.
// [EL DISEÑO NO LO DICE]: the status codes; 422 invalid_reason (the
// answer was recorded and the card stays open), 409 for a card that is
// not open or whose decision changed.
func (s *Server) answerCard(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	var req AnswerCardRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	answer, err := cycle.NewGate(s.db).AnswerCard(r.Context(), projectID, r.PathValue("label"), req.Option, req.Reason)
	switch {
	case errors.Is(err, cycle.ErrCardNotFound):
		writeError(w, http.StatusNotFound, "card_not_found")
	case errors.Is(err, cycle.ErrInvalidOption):
		writeError(w, http.StatusBadRequest, "invalid_option")
	case errors.Is(err, cycle.ErrInvalidReason):
		writeError(w, http.StatusUnprocessableEntity, "invalid_reason")
	case errors.Is(err, cycle.ErrCardNotOpen):
		writeError(w, http.StatusConflict, "card_not_open")
	case errors.Is(err, cycle.ErrCardObsolete):
		writeError(w, http.StatusConflict, "card_obsolete")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal")
	default:
		writeJSON(w, http.StatusOK, AnswerCardResponse{Card: answer.Card, Decision: answer.Decision,
			NewDecision: answer.NewDecision})
	}
}
