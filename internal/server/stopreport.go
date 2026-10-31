package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// StopReportDecision is one decision the report shows (B-39): its
// statement and «instead of» (the app's), the principal's gist (D6) and
// the sections that use it.
type StopReportDecision struct {
	Label     string       `json:"label"`
	Statement string       `json:"statement"`
	InsteadOf string       `json:"instead_of"`
	Gist      string       `json:"gist"`
	Sections  []SectionRef `json:"sections"`
}

// StopReportCitation is a verified or not verified claim (B-41).
type StopReportCitation struct {
	Section SectionRef `json:"section"`
	Claim   string     `json:"claim"`
}

// StopReportOpenItem is an open card or «por aclarar» (B-41).
type StopReportOpenItem struct {
	Kind  string `json:"kind"`
	Label string `json:"label"`
	Text  string `json:"text"`
}

// StopReportView is P6 [2]: the last stop report and whether you opened
// it (backlog-r2 2.5). First the answer; Section is the section it
// evaluates, nil when none, and ProposeSignoff whether the app proposes
// signing it.
type StopReportView struct {
	Seen            bool                 `json:"seen"`
	CreatedAt       string               `json:"created_at"`
	StopReason      string               `json:"stop_reason"`
	Section         *SectionRef          `json:"section"`
	ProposeSignoff  bool                 `json:"propose_signoff"`
	Answer          string               `json:"answer"`
	Done            []string             `json:"done"`
	Next            string               `json:"next"`
	Decisions       []StopReportDecision `json:"decisions"`
	Verified        []StopReportCitation `json:"verified"`
	NotVerified     []StopReportCitation `json:"not_verified"`
	Open            []StopReportOpenItem `json:"open"`
	PrincipalFailed bool                 `json:"principal_failed"`
}

// sectionRef names a section by its current title; a label that is not
// a section any more keeps an empty title.
func (s *Server) sectionRef(ctx context.Context, projectID, label string) SectionRef {
	ref := SectionRef{Label: label}
	if obj, err := s.db.GetObject(ctx, projectID, label); err == nil && obj.Type == store.ObjectSection {
		ref.Title = str(bodyMap(obj), "title")
	}
	return ref
}

// GET /api/projects/{projectID}/stop-report: the last stop report and
// whether you opened it (P6). Reading it records nothing: folded does
// not count as seen (B-40).
// [EL DISEÑO NO LO DICE]: 404 no_stop_report when there is none.
func (s *Server) getStopReport(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	ctx := r.Context()
	report, ok, err := cycle.LatestStopReport(ctx, s.db, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "no_stop_report")
		return
	}
	view := StopReportView{Seen: report.Seen, CreatedAt: report.CreatedAt, StopReason: string(report.StopReason),
		ProposeSignoff: report.ProposeSignoff, Answer: report.Answer, Done: report.Done, Next: report.Next,
		Decisions: []StopReportDecision{}, Verified: []StopReportCitation{}, NotVerified: []StopReportCitation{},
		Open: []StopReportOpenItem{}, PrincipalFailed: report.PrincipalFailed}
	if view.Done == nil {
		view.Done = []string{}
	}
	if report.Section != "" {
		ref := s.sectionRef(ctx, projectID, report.Section)
		view.Section = &ref
	}
	gists := map[string]string{}
	for _, u := range report.Unseen {
		gists[u.Decision] = u.Gist
	}
	for _, d := range report.Decisions {
		dv := StopReportDecision{Label: d.Label, Statement: d.Statement, InsteadOf: d.InsteadOf,
			Gist: gists[d.Label], Sections: []SectionRef{}}
		for _, sec := range d.Sections {
			dv.Sections = append(dv.Sections, s.sectionRef(ctx, projectID, sec))
		}
		view.Decisions = append(view.Decisions, dv)
	}
	for _, c := range report.Verified {
		view.Verified = append(view.Verified, StopReportCitation{Section: s.sectionRef(ctx, projectID, c.Section), Claim: c.Claim})
	}
	for _, c := range report.NotVerified {
		view.NotVerified = append(view.NotVerified, StopReportCitation{Section: s.sectionRef(ctx, projectID, c.Section), Claim: c.Claim})
	}
	for _, o := range report.Open {
		view.Open = append(view.Open, StopReportOpenItem{Kind: o.Kind, Label: o.Label, Text: o.Text})
	}
	writeJSON(w, http.StatusOK, view)
}

// POST /api/projects/{projectID}/stop-report/seen: you opened the report
// in the panel (B-40): stop_report_seen and decision_seen (report) for
// each decision it shows.
// [EL DISEÑO NO LO DICE]: 204 on success, 404 no_stop_report.
func (s *Server) postStopReportSeen(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	err := cycle.NewGate(s.db).MarkStopReportSeen(r.Context(), projectID)
	switch {
	case errors.Is(err, cycle.ErrNoStopReport):
		writeError(w, http.StatusNotFound, "no_stop_report")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// ContinueStopReportRequest is «Sigue» or «Abordémoslo»: the button's
// text, sent as a message of yours (backlog-r2 2.5).
type ContinueStopReportRequest struct {
	Text string `json:"text"`
}

// POST /api/projects/{projectID}/stop-report/continue {text} → {turn_id}:
// «Sigue» and «Abordémoslo» are messages of yours (human_message) with
// the button's text; they open another turn, as POST /messages does.
// [EL DISEÑO NO LO DICE]: the same codes as POST /messages (202, 400
// empty_message, 409 turn_running); it does not require a report.
func (s *Server) postStopReportContinue(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	var req ContinueStopReportRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		writeError(w, http.StatusBadRequest, "empty_message")
		return
	}
	turnID, err := s.starter.StartTurn(projectID, req.Text)
	if errors.Is(err, cycle.ErrTurnRunning) {
		writeError(w, http.StatusConflict, "turn_running")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusAccepted, PostMessageResponse{TurnID: turnID})
}
