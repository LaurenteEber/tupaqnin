package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/LaurenteEber/tupaqnin/internal/checks"
	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// OpenQuestionOption is one option of a «por aclarar» (P4 [2]): its
// contrast, consequence and risk. Keep marks the app's «mantener» of a
// decision of yours or of the premise (estados §2 and §6).
type OpenQuestionOption struct {
	Option      string `json:"option"`
	Contrast    string `json:"contrast"`
	Consequence string `json:"consequence"`
	Risk        string `json:"risk"`
	Keep        bool   `json:"keep"`
	// Other marks the app's «la otra» of an objection (B-81), and RiskReasons
	// its risk, the decision's criticality reasons the page translates.
	Other       bool     `json:"other,omitempty"`
	RiskReasons []string `json:"risk_reasons,omitempty"`
	// Code is an option the app wrote, which the page translates: a
	// scope «por aclarar»'s (backlog-r3 3.4: complete_review ·
	// declare_limit).
	Code string `json:"code,omitempty"`
}

// OpenQuestionView is an open «por aclarar» as P4 [2] reads it.
type OpenQuestionView struct {
	Label             string               `json:"label"`
	State             string               `json:"state"`
	Question          string               `json:"question"`
	Options           []OpenQuestionOption `json:"options"`
	Sections          []SectionRef         `json:"sections"`
	Decisions         []string             `json:"decisions"`
	Variants          []string             `json:"variants"`
	UndeclaredByAgent bool                 `json:"undeclared_by_agent"`
	NewPassage        string               `json:"new_passage,omitempty"`
	ClashingPassage   string               `json:"clashing_passage,omitempty"`
	// ObjectionReason: born from an objection, Question is the decision and this
	// is your reason (P-115).
	ObjectionReason string `json:"objection_reason,omitempty"`
	// OffersOwn: Go offers «Escribe la tuya», on an objection's question (B-107).
	OffersOwn bool `json:"offers_own"`
}

type AnswerOpenQuestionResponse struct {
	OpenQuestion string `json:"open_question"`
	NewDecision  string `json:"new_decision,omitempty"`
	// DefensePreparing: your answer left a section ready and its defense
	// is being prepared in the background (P-92).
	DefensePreparing bool `json:"defense_preparing,omitempty"`
	// TurnID: your answer started the one step that applies it to the text (B-100).
	TurnID string `json:"turn_id,omitempty"`
}

// AnswerOpenQuestionRequest answers a «por aclarar» (backlog-r2 2.4):
// option is the index in its options, from 0; null chooses none. OwnStatement is
// «Escribe la tuya» (B-107), instead of an option.
type AnswerOpenQuestionRequest struct {
	Option       *int    `json:"option"`
	OwnStatement *string `json:"own_statement,omitempty"`
	Reason       string  `json:"reason"`
}

// OpenQuestionAnswerer answers a «por aclarar» and, when the premise
// changed, probes the current text against it (B-46), then prepares
// the defense of the section the answer left ready (P-86). *cycle.Runner
// satisfies it; without it, the gate answers alone.
type OpenQuestionAnswerer interface {
	AnswerOpenQuestion(ctx context.Context, projectID, label string, option *int, reason string) (cycle.OpenQuestionAnswer, error)
}

// OwnOptionAnswerer is «Escribe la tuya» with what any answer starts (B-107);
// *cycle.Runner satisfies it; without it, the gate answers alone.
type OwnOptionAnswerer interface {
	AnswerObjectionWithYours(ctx context.Context, projectID, label, statement, reason string) (cycle.OpenQuestionAnswer, error)
}

func (s *Server) openQuestionView(r *http.Request, projectID string, q store.Object) (OpenQuestionView, error) {
	ctx := r.Context()
	b := bodyMap(q)
	undeclared, _ := b["undeclared_by_agent"].(bool)
	view := OpenQuestionView{Label: q.Label, State: q.State, Question: str(b, "question"),
		Options: []OpenQuestionOption{}, Sections: []SectionRef{}, Decisions: []string{}, Variants: []string{},
		UndeclaredByAgent: undeclared, NewPassage: str(b, "new_passage"), ClashingPassage: str(b, "clashing_passage"),
		ObjectionReason: str(b, "objection_reason"), OffersOwn: str(b, "origin") == checks.OriginObjection}
	options, _ := b["options"].([]any)
	for _, o := range options {
		m, _ := o.(map[string]any)
		keep, _ := m["keep"].(bool)
		other, _ := m["other"].(bool)
		var risks []string
		raw, _ := m["risk_reasons"].([]any)
		for _, r := range raw {
			if s, ok := r.(string); ok {
				risks = append(risks, s)
			}
		}
		view.Options = append(view.Options, OpenQuestionOption{Option: str(m, "option"), Contrast: str(m, "contrast"),
			Consequence: str(m, "consequence"), Risk: str(m, "risk"), Keep: keep, Code: str(m, "code"), Other: other, RiskReasons: risks})
	}
	variants, _ := b["variants"].([]any)
	for _, v := range variants {
		if s, ok := v.(string); ok {
			view.Variants = append(view.Variants, s)
		}
	}
	// What it blocks, by its current blocks edges: sections by their
	// title, decisions by their statement (never a label on screen).
	edges, err := s.db.EdgesFrom(ctx, projectID, q.ID)
	if err != nil {
		return view, err
	}
	for _, e := range edges {
		if e.Type != store.EdgeBlocks {
			continue
		}
		obj, err := s.db.GetObject(ctx, projectID, e.ToID)
		if err != nil {
			return view, err
		}
		switch obj.Type {
		case store.ObjectSection:
			view.Sections = append(view.Sections, SectionRef{Label: obj.Label, Title: str(bodyMap(obj), "title")})
		case store.ObjectDecision:
			view.Decisions = append(view.Decisions, str(bodyMap(obj), "statement"))
		}
	}
	return view, nil
}

// POST /api/projects/{projectID}/open-questions/{label}/answer with
// {option, reason} (backlog-r2 2.4).
// [EL DISEÑO NO LO DICE]: the status codes, as the card's: 422
// invalid_reason (answer_without_reason recorded, still open), 422
// option_required (no option chosen, still open), 400 invalid_option
// (an index outside the list), 409 for one that is not open. B-107: 422
// empty_statement, 400 own_option_not_offered, and 400 invalid_option with both an
// option and your own.
func (s *Server) answerOpenQuestion(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	var req AnswerOpenQuestionRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if req.OwnStatement != nil && req.Option != nil {
		writeError(w, http.StatusBadRequest, "invalid_option")
		return
	}
	gate := cycle.NewGate(s.db)
	answer := gate.AnswerOpenQuestion
	if a, ok := s.starter.(OpenQuestionAnswerer); ok {
		answer = a.AnswerOpenQuestion
	}
	answerOwn := gate.AnswerObjectionWithYours
	if a, ok := s.starter.(OwnOptionAnswerer); ok {
		answerOwn = a.AnswerObjectionWithYours
	}
	var got cycle.OpenQuestionAnswer
	var err error
	if req.OwnStatement != nil {
		got, err = answerOwn(r.Context(), projectID, r.PathValue("label"), *req.OwnStatement, req.Reason)
	} else {
		got, err = answer(r.Context(), projectID, r.PathValue("label"), req.Option, req.Reason)
	}
	switch {
	case errors.Is(err, cycle.ErrOwnOptionNotOffered):
		writeError(w, http.StatusBadRequest, "own_option_not_offered")
	case errors.Is(err, cycle.ErrEmptyStatement):
		writeError(w, http.StatusUnprocessableEntity, "empty_statement")
	case errors.Is(err, cycle.ErrOpenQuestionNotFound):
		writeError(w, http.StatusNotFound, "open_question_not_found")
	case errors.Is(err, cycle.ErrInvalidOption):
		writeError(w, http.StatusBadRequest, "invalid_option")
	case errors.Is(err, cycle.ErrInvalidReason):
		writeError(w, http.StatusUnprocessableEntity, "invalid_reason")
	case errors.Is(err, cycle.ErrNoOption):
		writeError(w, http.StatusUnprocessableEntity, "option_required")
	case errors.Is(err, cycle.ErrOpenQuestionNotOpen):
		writeError(w, http.StatusConflict, "open_question_not_open")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal")
	default:
		writeJSON(w, http.StatusOK, AnswerOpenQuestionResponse{OpenQuestion: got.OpenQuestion, NewDecision: got.NewDecision,
			DefensePreparing: got.DefensePreparing, TurnID: got.TurnID})
	}
}
