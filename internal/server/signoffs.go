package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
)

// SignoffStarter starts the signoff's turns (B-13, B-14): Wonder's
// rounds and «Preguntar al agente». *cycle.Runner satisfies it; the
// routes that start a turn exist only when the starter does.
type SignoffStarter interface {
	SignoffExplanation(projectID, signLabel, text string) (string, error)
	SignoffAnswers(projectID, signLabel, originAnswer, caseAnswer string) (string, error)
	SignoffAsk(projectID, signLabel, text string) (string, error)
}

// SignoffBlocked is the answer to asking to sign when the signoff cannot
// open or sign: the error code and, for pendings, the list (7a, flujos
// §3 «la app te muestra qué falta»).
// [EL DISEÑO NO LO DICE]: the status; 409 with error "pendings".
type SignoffBlocked struct {
	Error       string          `json:"error"`
	Pendings    []graph.Pending `json:"pendings,omitempty"`
	OpenChanges []string        `json:"open_changes,omitempty"`
}

type SignoffTextRequest struct {
	Text string `json:"text"`
}

type SignoffAnswersRequest struct {
	OriginAnswer string `json:"origin_answer"`
	CaseAnswer   string `json:"case_answer"`
}

// maxSignoffBody caps every signoff request body (specification §6).
const maxSignoffBody = 1 << 20

func decodeSignoff(r *http.Request, v any) error {
	return json.NewDecoder(io.LimitReader(r.Body, maxSignoffBody)).Decode(v)
}

// signoffError maps the signoff's errors to their status and code.
func signoffError(w http.ResponseWriter, err error) {
	var pend *cycle.PendingsError
	switch {
	case errors.As(err, &pend):
		writeJSON(w, http.StatusConflict, SignoffBlocked{Error: "pendings", Pendings: pend.Pendings})
	case errors.Is(err, cycle.ErrSectionNotFound):
		writeError(w, http.StatusNotFound, "section_not_found")
	case errors.Is(err, cycle.ErrSignoffNotFound):
		writeError(w, http.StatusNotFound, "signoff_not_found")
	case errors.Is(err, cycle.ErrSectionSigned):
		writeError(w, http.StatusConflict, "already_signed")
	case errors.Is(err, cycle.ErrSignoffNotInProgress):
		writeError(w, http.StatusConflict, "signoff_not_in_progress")
	case errors.Is(err, cycle.ErrExplanationGiven):
		writeError(w, http.StatusConflict, "explanation_given")
	case errors.Is(err, cycle.ErrNoRoundYet):
		writeError(w, http.StatusConflict, "no_round_yet")
	case errors.Is(err, cycle.ErrBriefSignoff):
		writeError(w, http.StatusConflict, "brief_signoff")
	case errors.Is(err, cycle.ErrSignoffDoesNotCoincide):
		writeError(w, http.StatusConflict, "does_not_coincide")
	case errors.Is(err, cycle.ErrSignoffOutdated):
		writeError(w, http.StatusConflict, "signoff_outdated")
	case errors.Is(err, cycle.ErrTurnRunning):
		writeError(w, http.StatusConflict, "turn_running")
	case errors.Is(err, cycle.ErrEmptyAnswer):
		writeError(w, http.StatusBadRequest, "empty_answer")
	case errors.Is(err, cycle.ErrCaseAnswerRequired):
		writeError(w, http.StatusUnprocessableEntity, "case_answer_required")
	default:
		writeError(w, http.StatusInternalServerError, "internal")
	}
}

func (s *Server) writeSignoff(w http.ResponseWriter, r *http.Request, projectID, label string, status int) {
	v, err := graph.SignoffDetail(r.Context(), s.db, projectID, label)
	if err != nil {
		signoffError(w, cycle.ErrSignoffNotFound)
		return
	}
	writeJSON(w, status, v)
}

// POST /api/projects/{projectID}/sections/{label}/signoff → 201 the
// signoff in progress (sign-n); 409 with the pendings and open changes
// when they keep it from opening (7a).
func (s *Server) requestSignoff(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	open, err := cycle.NewGate(s.db).RequestSignoff(r.Context(), projectID, r.PathValue("label"))
	if err != nil {
		signoffError(w, err)
		return
	}
	if !open.Opened {
		writeJSON(w, http.StatusConflict, SignoffBlocked{Error: "pendings", Pendings: open.Pendings, OpenChanges: open.OpenChanges})
		return
	}
	s.writeSignoff(w, r, projectID, open.Label, http.StatusCreated)
}

// GET /api/projects/{projectID}/signoffs/{label}
func (s *Server) getSignoff(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	s.writeSignoff(w, r, projectID, r.PathValue("label"), http.StatusOK)
}

// POST /api/projects/{projectID}/signoffs/{label}/sign
func (s *Server) signSignoff(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	label := r.PathValue("label")
	if err := cycle.NewGate(s.db).Sign(r.Context(), projectID, label); err != nil {
		signoffError(w, err)
		return
	}
	s.writeSignoff(w, r, projectID, label, http.StatusOK)
}

// POST /api/projects/{projectID}/signoffs/{label}/leave (7g)
func (s *Server) leaveSignoff(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	label := r.PathValue("label")
	if err := cycle.NewGate(s.db).LeaveSignoff(r.Context(), projectID, label); err != nil {
		signoffError(w, err)
		return
	}
	s.writeSignoff(w, r, projectID, label, http.StatusOK)
}

// signoffTurn answers a route that starts a turn: 202 {turn_id}, or 200
// with an empty turn_id when no model is called (the brief signoff,
// A-4).
func signoffTurn(w http.ResponseWriter, turnID string, err error) {
	if err != nil {
		signoffError(w, err)
		return
	}
	status := http.StatusAccepted
	if turnID == "" {
		status = http.StatusOK
	}
	writeJSON(w, status, PostMessageResponse{TurnID: turnID})
}

// POST /api/projects/{projectID}/signoffs/{label}/explanation {text}
// → {turn_id} (7b, B-13 (1)).
func (s *Server) postSignoffExplanation(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	var req SignoffTextRequest
	if err := decodeSignoff(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	turnID, err := s.signoffs.SignoffExplanation(projectID, r.PathValue("label"), req.Text)
	signoffTurn(w, turnID, err)
}

// POST /api/projects/{projectID}/signoffs/{label}/answers
// {origin_answer, case_answer} → {turn_id} (7e, B-13 (2)).
func (s *Server) postSignoffAnswers(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	var req SignoffAnswersRequest
	if err := decodeSignoff(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	turnID, err := s.signoffs.SignoffAnswers(projectID, r.PathValue("label"), req.OriginAnswer, req.CaseAnswer)
	signoffTurn(w, turnID, err)
}

// POST /api/projects/{projectID}/signoffs/{label}/ask {text} →
// {turn_id}: «Preguntar al agente», an answer without new text (7d,
// B-14).
func (s *Server) postSignoffAsk(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	var req SignoffTextRequest
	if err := decodeSignoff(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	turnID, err := s.signoffs.SignoffAsk(projectID, r.PathValue("label"), req.Text)
	signoffTurn(w, turnID, err)
}
