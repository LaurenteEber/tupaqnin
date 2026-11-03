package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
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

// StopReportPending is one of the agent's open pendings in the report
// (P-111); the page names it by its kind, its cause and its passage.
type StopReportPending struct {
	Section SectionRef `json:"section"`
	Kind    string     `json:"kind"`
	Cause   string     `json:"cause,omitempty"`
	Passage string     `json:"passage,omitempty"`
}

// StopReportDecided is one decision of the stretch (B-103): statement and «porque»
// in literal, the sections by title, and who decided (human or agent).
type StopReportDecided struct {
	Statement string       `json:"statement"`
	InsteadOf string       `json:"instead_of"`
	Reason    string       `json:"reason"`
	Sections  []SectionRef `json:"sections"`
	Who       string       `json:"who"`
}

// StopReportAgentChange is what one step changed on its own in one section
// (B-103): its place in the stretch, its one line, and its «Sale / Entra».
type StopReportAgentChange struct {
	Section SectionRef          `json:"section"`
	Step    int                 `json:"step"`
	Summary string              `json:"summary"`
	Changes []graph.JudgeChange `json:"changes"`
}

// StopReportView is P6 [2]: the last stop report and whether you opened
// it (backlog-r2 2.5). First the answer; Section is the section it
// evaluates, nil when none, and ProposeSignoff whether the app proposes
// signing it.
type StopReportView struct {
	Seen       bool   `json:"seen"`
	CreatedAt  string `json:"created_at"`
	StopReason string `json:"stop_reason"`
	// Cap says which of the demo's caps stopped the chain (B-89).
	Cap string `json:"cap,omitempty"`
	// Cause says why the mode finished, with stop_reason mode_finished (B-83).
	Cause           string               `json:"cause,omitempty"`
	Section         *SectionRef          `json:"section"`
	ProposeSignoff  bool                 `json:"propose_signoff"`
	Answer          string               `json:"answer"`
	Done            []string             `json:"done"`
	Next            string               `json:"next"`
	Decisions       []StopReportDecision `json:"decisions"`
	Verified        []StopReportCitation `json:"verified"`
	NotVerified     []StopReportCitation `json:"not_verified"`
	Open            []StopReportOpenItem `json:"open"`
	AgentPendings   []StopReportPending  `json:"agent_pendings"`
	PrincipalFailed bool                 `json:"principal_failed"`
	// Unapplied are the titles of the sections your decisions have not reached,
	// read now, not when the report was written (B-100).
	Unapplied []string `json:"unapplied"`
	// AudienceQuestions are the open audience questions of the report's
	// section (backlog-r2 2.6, P6 «preparación de la defensa»), each
	// answered «sí» or «no»; DefenseFailed says the judge failed its
	// shape twice at this stop (fichas §7: visible, blocking nothing).
	AudienceQuestions []graph.AudienceQuestion `json:"audience_questions"`
	DefenseFailed     bool                     `json:"defense_failed"`
	// JudgeNotVerified are the sections the judge could not verify
	// (backlog-r3 3.6, fichas §7: visible, blocking nothing).
	JudgeNotVerified []SectionRef `json:"judge_not_verified"`
	// Detected is what the app detected since this report, your edits
	// included (B-48 b): the short return shows it under the report.
	Detected []DetectedView `json:"detected"`
	// Decided and AgentChanges catch you up on the stretch before signing (B-103).
	Decided      []StopReportDecided     `json:"decided"`
	AgentChanges []StopReportAgentChange `json:"agent_changes"`
	// OpensConversation: the report is unseen and there is no return
	// line, so the conversation opens with it on top (short return,
	// backlog-r2 2.7). Go decides it, with the frame's clock.
	OpensConversation bool `json:"opens_conversation"`
}

// DetectedView is a graph.Detected with its section named by title.
type DetectedView struct {
	Kind    string      `json:"kind"`
	Actor   string      `json:"actor"`
	Section *SectionRef `json:"section,omitempty"`
	Text    string      `json:"text,omitempty"`
	To      string      `json:"to,omitempty"`
	Count   int         `json:"count"`
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
	view := StopReportView{Seen: report.Seen, CreatedAt: report.CreatedAt, StopReason: string(report.StopReason), Cause: report.Cause, Cap: report.Cap,
		ProposeSignoff: report.ProposeSignoff, Answer: report.Answer, Done: report.Done, Next: report.Next,
		Decisions: []StopReportDecision{}, Verified: []StopReportCitation{}, NotVerified: []StopReportCitation{},
		Open: []StopReportOpenItem{}, AgentPendings: []StopReportPending{}, PrincipalFailed: report.PrincipalFailed,
		AudienceQuestions: []graph.AudienceQuestion{}}
	if view.Done == nil {
		view.Done = []string{}
	}
	unapplied, err := graph.UnappliedDecisions(ctx, s.db, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	view.Unapplied = []string{}
	for _, u := range unapplied {
		if !slices.Contains(view.Unapplied, u.Title) {
			view.Unapplied = append(view.Unapplied, u.Title)
		}
	}
	if report.Section != "" {
		ref := s.sectionRef(ctx, projectID, report.Section)
		view.Section = &ref
		qs, err := graph.OpenAudienceQuestions(ctx, s.db, projectID, report.Section)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
		if qs != nil {
			view.AudienceQuestions = qs
		}
		if view.DefenseFailed, err = cycle.DefenseFailedSince(ctx, s.db, projectID, report.Seq); err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
	}
	view.JudgeNotVerified = []SectionRef{}
	// A chain without work can verify after the report and writes no other
	// report (P-75 b): the page shows only what still could not verify.
	stillNot, err := cycle.JudgeNotVerified(ctx, s.db, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	for _, label := range report.JudgeNotVerified {
		if slices.Contains(stillNot, label) {
			view.JudgeNotVerified = append(view.JudgeNotVerified, s.sectionRef(ctx, projectID, label))
		}
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
	for _, p := range report.AgentPendings {
		view.AgentPendings = append(view.AgentPendings, StopReportPending{Section: s.sectionRef(ctx, projectID, p.Section),
			Kind: p.Kind, Cause: p.Cause, Passage: p.Passage})
	}
	view.Decided = []StopReportDecided{}
	for _, d := range report.Decided {
		dv := StopReportDecided{Statement: d.Statement, InsteadOf: d.InsteadOf, Reason: d.Reason, Who: d.Who, Sections: []SectionRef{}}
		for _, sec := range d.Sections {
			dv.Sections = append(dv.Sections, s.sectionRef(ctx, projectID, sec))
		}
		view.Decided = append(view.Decided, dv)
	}
	view.AgentChanges = []StopReportAgentChange{}
	for _, c := range report.AgentChanges {
		view.AgentChanges = append(view.AgentChanges, StopReportAgentChange{Section: s.sectionRef(ctx, projectID, c.Section),
			Step: c.Step, Summary: c.Summary, Changes: c.Changes})
	}
	// The short return (B-48 b): what was detected since this report,
	// and whether the conversation opens with it (unseen, no return line).
	detected, err := graph.DetectedSinceStop(ctx, s.db, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	view.Detected = []DetectedView{}
	for _, d := range detected {
		dv := DetectedView{Kind: d.Kind, Actor: d.Actor, Text: d.Text, To: d.To, Count: d.Count}
		if d.Section != "" {
			ref := s.sectionRef(ctx, projectID, d.Section)
			dv.Section = &ref
		}
		view.Detected = append(view.Detected, dv)
	}
	events, err := s.db.EventsSince(ctx, projectID, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	view.OpensConversation = !report.Seen && !graph.ReturnDue(events, s.now(), s.returnAfter)
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
	writeTurnStarted(w, turnID)
}

// CorrectionAsker is «Pedir corrección al agente» (B-103); the runner does it.
type CorrectionAsker interface {
	RequestCorrection(ctx context.Context, projectID, label, kind, about, message string) (string, error)
}

// CorrectionRequest names what to correct, as «Firmar» listed it, and the message
// that stands for it in the conversation if it has to wait, in the page's language.
type CorrectionRequest struct {
	Kind    string `json:"kind"`
	About   string `json:"about"`
	Message string `json:"message"`
}

// POST /api/projects/{projectID}/sections/{label}/correction {kind, about, message}
// → 202 {turn_id, queued}: one step of the agent on that pending, or your message
// queued with a turn running (B-103). Go checks it is open.
// [EL DISEÑO NO LO DICE]: 400 empty_message, 404 not_found, 409 pending_closed when
// it is no longer open, 409 pending_yours for a card or a «por aclarar».
func (s *Server) postCorrection(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	asker, ok := s.starter.(CorrectionAsker)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	var req CorrectionRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		writeError(w, http.StatusBadRequest, "empty_message")
		return
	}
	turnID, err := asker.RequestCorrection(r.Context(), projectID, r.PathValue("label"), req.Kind, req.About, req.Message)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found")
	case errors.Is(err, cycle.ErrPendingNotOpen):
		writeError(w, http.StatusConflict, "pending_closed")
	case errors.Is(err, cycle.ErrPendingYours):
		writeError(w, http.StatusConflict, "pending_yours")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal")
	default:
		writeTurnStarted(w, turnID)
	}
}
