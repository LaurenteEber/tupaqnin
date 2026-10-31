package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// AnswerAudienceQuestionRequest answers an audience question (estados
// §9): answer is "yes" («sí») or "no».
type AnswerAudienceQuestionRequest struct {
	Answer string `json:"answer"`
}

// AnswerAudienceQuestionResponse is the question's label and new state.
type AnswerAudienceQuestionResponse struct {
	AudienceQuestion string `json:"audience_question"`
	State            string `json:"state"`
}

// POST /api/projects/{projectID}/audience-questions/{label}/answer
// {answer: yes|no} (backlog-r2 2.6): «sí» → addressed, your instruction
// in the principal's next turn, without opening one; «no» → declined
// with audience_question_declined.
// [EL DISEÑO NO LO DICE]: 200 with the new state; 400 invalid_answer;
// 404 audience_question_not_found; 409 audience_question_not_open.
func (s *Server) answerAudienceQuestion(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	var req AnswerAudienceQuestionRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	label := r.PathValue("label")
	err := cycle.NewGate(s.db).AnswerAudienceQuestion(r.Context(), projectID, label, req.Answer)
	switch {
	case errors.Is(err, cycle.ErrInvalidAudienceAnswer):
		writeError(w, http.StatusBadRequest, "invalid_answer")
		return
	case errors.Is(err, cycle.ErrAudienceQuestionNotFound):
		writeError(w, http.StatusNotFound, "audience_question_not_found")
		return
	case errors.Is(err, cycle.ErrAudienceQuestionNotOpen):
		writeError(w, http.StatusConflict, "audience_question_not_open")
		return
	case errors.Is(err, store.ErrVersionConflict):
		writeError(w, http.StatusConflict, "version_conflict")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	aq, err := s.db.GetObject(r.Context(), projectID, label)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, AnswerAudienceQuestionResponse{AudienceQuestion: aq.Label, State: aq.State})
}
