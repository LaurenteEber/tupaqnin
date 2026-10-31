package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// H-3's actions of slice 2 (backlog-r2 2.8): change the audience or a
// criterion after the request is sent, and download the document.

// CriterionChanger writes a criterion's new version and runs its probe
// (B-46, P-72). *cycle.Runner satisfies it; without it, the gate writes
// the version and queues the probe for the next trigger.
type CriterionChanger interface {
	ChangeCriterion(ctx context.Context, projectID, label, text string) (cycle.CriterionChange, error)
}

var errNoRequest = errors.New("server: no request")

// PUT /api/projects/{projectID}/request/audience with {audience}: the
// audience, literal, with audience_changed (old and new value, K-13).
// It changes no section's state: only the next defense preparation
// reads it (fichas §2, Tu turno). The same audience writes nothing.
// [EL DISEÑO NO LO DICE]: the route works before submit too (it only
// sets the audience); PUT /request is the one refused after submit.
func (s *Server) putRequestAudience(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	var req PutAudienceRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	ctx := r.Context()
	err := s.db.Write(ctx, projectID, func(tx *store.Tx) error {
		requests, err := tx.ListObjects(ctx, projectID, store.ObjectRequest)
		if err != nil {
			return err
		}
		if len(requests) == 0 {
			return errNoRequest
		}
		obj := requests[0]
		b := bodyMap(obj)
		from := str(b, "audience")
		if from == req.Audience {
			return nil
		}
		b["audience"] = req.Audience
		_, err = tx.UpdateObject(obj.ID, obj.CurrentVersion, obj.State, b,
			store.EventInput{Type: store.EventAudienceChanged, Actor: "human", ObjectID: obj.ID,
				Data: map[string]any{"label": obj.Label, "from": from, "to": req.Audience}})
		return err
	})
	switch {
	case errors.Is(err, errNoRequest):
		writeError(w, http.StatusNotFound, "request_not_found")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	view, _ := s.requestView(r, projectID)
	writeJSON(w, http.StatusOK, view)
}

// PUT /api/projects/{projectID}/criteria/{label} with {text}: the
// criterion's new version, yours only (estados §6), and B-46's probe
// queued in the checker (2.3, P-72).
func (s *Server) putCriterion(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	var req PutCriterionRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	change := cycle.NewGate(s.db).ChangeCriterion
	if c, ok := s.starter.(CriterionChanger); ok {
		change = c.ChangeCriterion
	}
	got, err := change(r.Context(), projectID, r.PathValue("label"), req.Text)
	switch {
	case errors.Is(err, cycle.ErrCriterionNotFound):
		writeError(w, http.StatusNotFound, "criterion_not_found")
	case errors.Is(err, cycle.ErrCriterionTextRequired):
		writeError(w, http.StatusBadRequest, "criterion_text_required")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal")
	default:
		writeJSON(w, http.StatusOK, CriterionView{Label: got.Label, Version: got.Version, Text: got.Text})
	}
}

// ExportNotSigned is B-47's mark per language: the catalog's
// export.not_signed (web/src/i18n, T-1), copied here because Go writes
// the file; a test keeps both equal.
var ExportNotSigned = map[string]string{"en": "Not signed", "es": "Sin firmar"}

// GET /api/projects/{projectID}/export.md[?lang=es]: the document as a
// Markdown download (backlog-r2 2.8, K-10 b, B-47), built from the graph
// in Go (graph.ExportMarkdown); the file name comes from project.name.
// [EL DISEÑO NO LO DICE]: the mark's language is the page's (?lang, as
// the page's own switch; English by default, H-1); the file name keeps
// the name with path separators and control characters replaced, and
// «document» when the project has no name.
func (s *Server) getExport(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	mark := ExportNotSigned["en"]
	if r.URL.Query().Get("lang") == "es" {
		mark = ExportNotSigned["es"]
	}
	md, err := graph.ExportMarkdown(r.Context(), s.db, projectID, mark)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	name, err := s.db.ProjectName(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition",
		mime.FormatMediaType("attachment", map[string]string{"filename": exportFilename(name)}))
	_, _ = io.WriteString(w, md)
}

// exportFilename is project.name as a safe file name with .md.
func exportFilename(name string) string {
	clean := strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || strings.ContainsRune(`/\:*?"<>|`, r) {
			return '-'
		}
		return r
	}, strings.TrimSpace(name))
	clean = strings.Trim(clean, ". -")
	if clean == "" {
		clean = "document"
	}
	return clean + ".md"
}
