package server

import (
	"context"
	"net/http"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// SessionView is GET /api/session (E.6): who you are for the home, and whether
// a new project would open, computed by the rules POST /api/projects applies.
type SessionView struct {
	Account   string `json:"account"` // username, or empty without an account
	LinkLabel string `json:"link_label"`
	CanCreate bool   `json:"can_create"`
	// CannotCreate is empty, or account_required · account_limit · link_full ·
	// link_budget · day_cap · disk_full.
	CannotCreate string `json:"cannot_create"`
}

// ProjectLine is one row of «Tus proyectos» (P1): the list's fields, then what
// the home adds, all computed in Go (AGENTS rule 5).
type ProjectLine struct {
	ProjectView  `tstype:",extends"`
	LastActivity string `json:"last_activity"`
	Signed       int    `json:"signed"`
	Sections     int    `json:"sections"`
	// NoAccount replaces the row's expires_in_days (Eber, 2-nov): with a 24 h
	// server expiry days mean nothing, the home says the project is lost when
	// the browser closes.
	// [EL DISEÑO NO LO DICE]: the field's name and shape.
	NoAccount bool `json:"no_account"`
}

// getSession is GET /api/session. Without a session it answers 401 no_session
// and says nothing else: the web shows «Necesitas un link de invitación» and
// «Entrar» on that answer.
// [EL DISEÑO NO LO DICE]: 401 and not a 200 with an empty session, the same code
// every other route gives a visitor with no session.
func (s *Server) getSession(w http.ResponseWriter, r *http.Request) {
	sess, ok, err := s.sessionOf(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	if !ok {
		writeError(w, http.StatusUnauthorized, "no_session")
		return
	}
	s.touchSession(w, r, sess)
	account, label, err := s.db.SessionNames(r.Context(), sess)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	refusal, err := s.createRefusal(r.Context(), sess)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, SessionView{Account: account, LinkLabel: label, CanCreate: refusal == "", CannotCreate: refusal})
}

// createRefusal is the code POST /api/projects would answer now, or "", asked in
// the same order: the caps, then the rules of the link and the account. It writes
// nothing.
func (s *Server) createRefusal(ctx context.Context, sess store.Session) (string, error) {
	blocked, err := s.createBlocked(ctx, sess)
	if err != nil || blocked != "" {
		return blocked, err
	}
	err = s.db.CreateRefused(ctx, sess)
	if code, known := createRefusalCode(err); known {
		return code, nil
	}
	return "", err
}

// listProjectLines is GET /api/projects: only your projects (B3-18, P1).
func (s *Server) listProjectLines(w http.ResponseWriter, r *http.Request) {
	sess, _ := r.Context().Value(sessionKey{}).(store.Session)
	projects, err := s.db.ListOwnedProjects(r.Context(), sess)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	lines := make([]ProjectLine, 0, len(projects))
	for _, p := range projects {
		signed, sections, err := s.signedSections(r.Context(), p.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
		lines = append(lines, ProjectLine{
			ProjectView:  ProjectView{ProjectID: p.ID, Name: p.Name, CreatedAt: p.CreatedAt},
			LastActivity: p.LastActivity,
			Signed:       signed,
			Sections:     sections,
			NoAccount:    p.NoAccount,
		})
	}
	writeJSON(w, http.StatusOK, lines)
}

// signedSections counts a project's sections and those signed, by the one
// function for a section's state. One changed since its signoff is not signed.
func (s *Server) signedSections(ctx context.Context, projectID string) (signed, sections int, err error) {
	objects, err := graph.OrderedSections(ctx, s.db, projectID)
	if err != nil {
		return 0, 0, err
	}
	for _, obj := range objects {
		status, _, err := graph.SectionStatus(ctx, s.db, projectID, obj.Label)
		if err != nil {
			return 0, 0, err
		}
		if status == graph.StatusSigned {
			signed++
		}
	}
	return signed, len(objects), nil
}
