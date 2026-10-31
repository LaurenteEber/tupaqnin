package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
)

// AcceptClaimRequest accepts a paragraph without a source as general
// knowledge: its literal passage and your «porque» (R-1, backlog-r2 2.11).
type AcceptClaimRequest struct {
	Passage string `json:"passage"`
	Reason  string `json:"reason"`
}

// EvidenceRequiredView is whether the document asks for evidence, with
// the «Entendí»'s line on why and whether you confirmed it (B-51).
type EvidenceRequiredView struct {
	Value bool   `json:"value"`
	Why   string `json:"why"`
	State string `json:"state"`
}

// PutEvidenceRequest is «Corregir» on «¿pide evidencia?» (B-51).
type PutEvidenceRequest struct {
	Required *bool `json:"required"`
}

// POST /api/projects/{projectID}/sections/{label}/claims/accept with
// {passage, reason} → 204; claim_accepted in the history (R-1).
// [EL DISEÑO NO LO DICE]: the status codes, as closing a change: 404 for
// no such section, 422 for a «porque» that does not count, 409 for a
// passage that is not a paragraph of the current text.
func (s *Server) acceptClaim(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	var req AcceptClaimRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	err := cycle.NewGate(s.db).AcceptClaim(r.Context(), projectID, r.PathValue("label"), req.Passage, req.Reason)
	switch {
	case errors.Is(err, cycle.ErrSectionNotFound):
		writeError(w, http.StatusNotFound, "section_not_found")
	case errors.Is(err, cycle.ErrInvalidReason):
		writeError(w, http.StatusUnprocessableEntity, "invalid_reason")
	case errors.Is(err, cycle.ErrPassageNotInSection):
		writeError(w, http.StatusConflict, "passage_not_in_section")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// PUT /api/projects/{projectID}/request/evidence with {required} → 200
// with the stored value, confirmed (B-51).
func (s *Server) putRequestEvidence(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	var req PutEvidenceRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil || req.Required == nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	got, err := cycle.ConfirmEvidenceRequired(r.Context(), s.db, projectID, *req.Required)
	switch {
	case errors.Is(err, cycle.ErrNoRequest):
		writeError(w, http.StatusNotFound, "request_not_found")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal")
	default:
		writeJSON(w, http.StatusOK, EvidenceRequiredView(got))
	}
}
