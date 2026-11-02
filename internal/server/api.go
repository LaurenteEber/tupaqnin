package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/LaurenteEber/tupaqnin/internal/checks"
	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/fingerprint"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// TurnStarter starts a turn in the background and returns its id at
// once (B-1). *cycle.Runner satisfies it.
type TurnStarter interface {
	StartTurn(projectID, text string) (string, error)
	StartUnderstandingTurn(projectID string) (string, error)
}

// EditNotifier is told each save of your edit that changed a section,
// so the batch of your edits goes to the checker 60 s after the last
// one (B-36). *cycle.Runner satisfies it.
type EditNotifier interface {
	EditSaved(projectID string)
}

// API request and response shapes (backlog 0.10). TypeScript types
// are generated from these with tygo (web/src/api/types.ts); the API
// only transports what Go computes (rule 5 of AGENTS.md).
type CreateProjectResponse struct {
	ProjectID string `json:"project_id"`
}

type SectionResponse struct {
	Label   string `json:"label"`
	Version int    `json:"version"`
	// Proposal is the proposal your save displaced (B-73 d), if any.
	Proposal string `json:"proposal,omitempty"`
}

type SectionView struct {
	Label   string `json:"label"`
	Title   string `json:"title"`
	Focus   string `json:"focus"`
	Version int    `json:"version"`
	// State is the section's status from graph.SectionStatus, the one
	// function every view uses (ready included, computed).
	State    string `json:"state"`
	Pendings int    `json:"pendings"`
	Text     string `json:"text"`
	// Unverified are the paragraphs (blocks, from 1) without a verified
	// origin: the page marks them «sin verificar» (R-1).
	Unverified []int `json:"unverified"`
	// Origins says beside each paragraph where it comes from (backlog-r3
	// 3.5), as graph.Provenances computes it.
	Origins []ParagraphOrigin `json:"origins"`
	// Paragraphs are the editor's paragraphs, one per block of the
	// history (P-78): paragraph n is block n, a list item one each.
	Paragraphs []graph.Paragraph `json:"paragraphs"`
}

// ParagraphOrigin is one paragraph's origin: its block, from 1, and
// material · external · model_knowledge · unverified.
type ParagraphOrigin struct {
	Block  int    `json:"block"`
	Origin string `json:"origin"`
}

type PutSectionRequest struct {
	TabID       string `json:"tab_id"`
	BaseVersion int    `json:"base_version"`
	Text        string `json:"text"`
}

type PostMessageRequest struct {
	Text string `json:"text"`
}

type PostMessageResponse struct {
	TurnID string `json:"turn_id"`
	// Queued: the mode's running chain takes the message at its next step
	// (B-83), and TurnID is empty.
	Queued bool `json:"queued,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

func (s *Server) requireProject(w http.ResponseWriter, r *http.Request, projectID string) bool {
	exists, err := s.db.ProjectExists(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return false
	}
	if !exists {
		writeError(w, http.StatusNotFound, "project_not_found")
		return false
	}
	return true
}

// CreateSectionRequest is «+ Sección» (K-17): a title (required), the
// «aquí se concluye» mark (role "conclusion", or empty: the principal
// proposes it) and «qué debe abordar».
type CreateSectionRequest struct {
	TabID string `json:"tab_id"`
	Title string `json:"title"`
	Role  string `json:"role"`
	Focus string `json:"focus"`
}

// POST /api/projects/{projectID}/sections with {title, role, focus} →
// {label, version}: a new empty section in draft, last (K-17). It
// replaces the 0.10 route.
func (s *Server) createSection(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	var req CreateSectionRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if !s.editors.claim(projectID, req.TabID) {
		writeError(w, http.StatusConflict, "read_only_tab")
		return
	}
	obj, err := cycle.CreateSection(r.Context(), s.db, projectID, req.Title, req.Role, req.Focus)
	if err != nil {
		writeSectionError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, SectionResponse{Label: obj.Label, Version: obj.CurrentVersion})
}

// SectionDetailsRequest changes the title or «qué debe abordar» of a
// section (P-49); a missing field stays as it is.
type SectionDetailsRequest struct {
	TabID string  `json:"tab_id"`
	Title *string `json:"title"`
	Focus *string `json:"focus"`
	// Plan: the change is a correction from the «Entendí»'s card, so it
	// also answers the plan (R-6, cycle.CorrectPlanDetails).
	Plan bool `json:"plan,omitempty"`
}

// PUT /api/projects/{projectID}/sections/{label}/details with
// {title?, focus?} → 204. The state does not change (K-17).
func (s *Server) putSectionDetails(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	var req SectionDetailsRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if !s.editors.claim(projectID, req.TabID) {
		writeError(w, http.StatusConflict, "read_only_tab")
		return
	}
	change := cycle.ChangeSectionDetails
	if req.Plan {
		change = cycle.CorrectPlanDetails
	}
	if err := change(r.Context(), s.db, projectID, r.PathValue("label"), req.Title, req.Focus); err != nil {
		writeSectionError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SectionOrderRequest names every section once, in the new order.
type SectionOrderRequest struct {
	TabID  string   `json:"tab_id"`
	Labels []string `json:"labels"`
}

// PUT /api/projects/{projectID}/sections/order with {labels[]} → 204
// (P-49: «subir» and «bajar» in the frame skeleton).
func (s *Server) putSectionOrder(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	var req SectionOrderRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if !s.editors.claim(projectID, req.TabID) {
		writeError(w, http.StatusConflict, "read_only_tab")
		return
	}
	if err := cycle.ReorderSections(r.Context(), s.db, projectID, req.Labels); err != nil {
		writeSectionError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeSectionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, cycle.ErrTitleRequired), errors.Is(err, cycle.ErrFocusTooLong),
		errors.Is(err, cycle.ErrInvalidRole), errors.Is(err, cycle.ErrInvalidOrder):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, cycle.ErrNotASection):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, store.ErrVersionConflict):
		writeError(w, http.StatusConflict, "version_conflict")
	default:
		writeError(w, http.StatusInternalServerError, "internal")
	}
}

func sectionText(obj store.Object) string {
	var b map[string]any
	_ = json.Unmarshal(obj.Body, &b)
	text, _ := b["text"].(string)
	return text
}

// GET /api/projects/{projectID}/sections
func (s *Server) listSections(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	objects, err := graph.OrderedSections(r.Context(), s.db, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	views := make([]SectionView, 0, len(objects))
	for _, obj := range objects {
		status, pendings, err := graph.SectionStatus(r.Context(), s.db, projectID, obj.Label)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
		provenance, err := graph.Provenances(r.Context(), s.db, projectID, obj.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
		unverified, origins := []int{}, []ParagraphOrigin{}
		for _, p := range provenance {
			if !p.Verified {
				unverified = append(unverified, p.Block)
			}
			origins = append(origins, ParagraphOrigin{Block: p.Block, Origin: p.Origin})
		}
		views = append(views, SectionView{Label: obj.Label, Title: str(bodyMap(obj), "title"), Focus: str(bodyMap(obj), "focus"),
			Version: obj.CurrentVersion, State: string(status), Pendings: len(pendings), Text: sectionText(obj),
			Unverified: unverified, Origins: origins, Paragraphs: graph.Paragraphs(sectionText(obj))})
	}
	writeJSON(w, http.StatusOK, views)
}

// PUT /api/projects/{projectID}/sections/{label} with
// {base_version, text} → 200 (new version; over a text of the agent's, it
// goes to a proposal, B-73 d), 204 (no change, B-3) or 409 (a base version
// with your own edit on top, P-84).
func (s *Server) putSection(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	label := r.PathValue("label")
	if !s.requireProject(w, r, projectID) {
		return
	}
	var req PutSectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	current, err := s.db.GetObject(r.Context(), projectID, label)
	if err != nil || current.Type != store.ObjectSection {
		writeError(w, http.StatusNotFound, "section_not_found")
		return
	}
	// Only one tab edits (specification §2): a save from another tab is
	// refused.
	if !s.editors.claim(projectID, req.TabID) {
		writeError(w, http.StatusConflict, "read_only_tab")
		return
	}
	// P-84 (T-4: the base is the text): a base version older than the
	// current one whose stored text is the current text (the plan
	// confirmed, a title, focus, position or role changed meanwhile) is
	// accepted against the current version; a changed text keeps the 409.
	currentText := str(bodyMap(current), "text")
	// The text your tab read: versions only grow, so it is read outside the
	// write's transaction.
	readText, readOK, err := s.textAtVersion(r, projectID, current.ID, req.BaseVersion)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	// B-73 d: over a text the agent wrote since, your save rules and its text
	// goes to a proposal; anything of yours on top keeps the 409 (P-84).
	displaces := func(q store.Querier, text string) (bool, error) {
		if !readOK || readText == text {
			return false, nil
		}
		on, err := cycle.AgentOnTop(r.Context(), q, projectID, current.ID, req.BaseVersion)
		if err != nil {
			return false, err
		}
		if !on {
			return false, store.ErrVersionConflict
		}
		return true, nil
	}
	if req.BaseVersion != current.CurrentVersion {
		if _, err := displaces(s.db, currentText); errors.Is(err, store.ErrVersionConflict) || !readOK {
			writeError(w, http.StatusConflict, "version_conflict")
			return
		} else if err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
	}
	// The section fingerprint replaces B-3's comparison (specification
	// §4): the same content, whatever its formatting, is no change.
	if fingerprint.Section(req.Text) == fingerprint.Section(currentText) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	beforeSectionWrite()
	var updated store.Object
	var displacedProposal string
	unchanged, signoffCancelled := false, false
	err = s.db.Write(r.Context(), projectID, func(tx *store.Tx) error {
		// P-84: re-read in the transaction; a version written since the
		// read above that kept the text keeps your edit, which keeps its
		// details and is conditioned on the version re-read here.
		obj, err := tx.Object(current.ID)
		if err != nil {
			return err
		}
		agentText := str(bodyMap(obj), "text")
		displaced, err := displaces(tx, agentText)
		if err != nil {
			return err
		}
		// [EL DISEÑO NO LO DICE]: a save of the very text your tab read, over
		// the agent's, is not an edit (the client does not send it; specification
		// §2): nothing changes and nothing is displaced.
		if displaced && fingerprint.Section(req.Text) == fingerprint.Section(readText) {
			unchanged = true
			return nil
		}
		body := bodyMap(obj)
		body["text"] = req.Text
		// Your edit on a signed section moves it to changed_since_signoff
		// (estados §1: your edit rules).
		state := obj.State
		if state == "signed" {
			state = "changed_since_signoff"
		}
		ev := store.EventInput{Type: store.EventEdit, Actor: "human"}
		if displaced {
			// Undoing it goes back to what your tab read, not to the agent's
			// text, which stays in its proposal (B-79).
			ev.Data = map[string]any{graph.EditReadVersion: req.BaseVersion}
		}
		updated, err = tx.UpdateObject(obj.ID, obj.CurrentVersion, state, body, ev)
		if err != nil {
			return err
		}
		// The app's rules 1 to 5 run on each save that changes the
		// fingerprint (B-36), in the same transaction (backlog-r2 2.1). When
		// it displaces the agent's text, your edit is over what your tab read.
		edited := currentText
		if displaced {
			edited = readText
			err := cycle.TrackCriticality(r.Context(), tx, "", func() error {
				var err error
				displacedProposal, err = cycle.DisplaceAgentText(r.Context(), tx, updated, agentText,
					cycle.Displacement{BaseVersion: req.BaseVersion})
				return err
			})
			if err != nil {
				return err
			}
		}
		if _, err := cycle.CheckYourEdit(r.Context(), tx, current.ID, edited); err != nil {
			return err
		}
		// Rule 6, on your edits only and without blocking (B-80).
		if err := checks.AlertSuspiciousEdit(r.Context(), tx, checks.SuspectEdit{
			Section: updated, BaseText: edited, WasSigned: obj.State == "signed", Now: s.now()}); err != nil {
			return err
		}
		// Your edit of a section in signoff cancels the signoff in
		// progress (K-16, B-14: signoff_cancelled, edited). P-90 (as
		// P-89): read in this transaction, so a signoff that started
		// after the read above is cancelled too.
		signing, inSignoff, err := graph.ActiveSignoff(r.Context(), tx, projectID, current.ID)
		if err != nil || !inSignoff {
			return err
		}
		signoffCancelled = true
		return cycle.CancelSignoff(tx, signing, graph.CancelEdited, "human", map[string]any{"section": current.Label})
	})
	switch {
	case errors.Is(err, store.ErrVersionConflict):
		writeError(w, http.StatusConflict, "version_conflict")
	case errors.Is(err, store.ErrNoChange), err == nil && unchanged:
		w.WriteHeader(http.StatusNoContent)
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal")
	default:
		// [EL DISEÑO NO LO DICE]: «sin cambios» counts from a save that
		// changed the fingerprint, as the rules (B-36).
		if n, ok := s.starter.(EditNotifier); ok {
			n.EditSaved(projectID)
		}
		if signoffCancelled {
			s.signoffClosed(r.Context(), projectID)
		}
		writeJSON(w, http.StatusOK, SectionResponse{Label: updated.Label, Version: updated.CurrentVersion, Proposal: displacedProposal})
	}
}

// beforeSectionWrite runs between putSection's reads and its write's
// transaction; tests use it to start a signoff in between (P-90).
var beforeSectionWrite = func() {}

// textAtVersion is the text of a stored version of a section (P-84), and
// whether that version exists. Versions only grow, so it is read outside the
// write's transaction.
func (s *Server) textAtVersion(r *http.Request, projectID, objectID string, version int) (string, bool, error) {
	versions, err := s.db.Versions(r.Context(), projectID, objectID)
	if err != nil {
		return "", false, err
	}
	for _, v := range versions {
		if v.Version != version {
			continue
		}
		var b map[string]any
		_ = json.Unmarshal(v.Body, &b)
		return str(b, "text"), true, nil
	}
	return "", false, nil
}

// POST /api/projects/{projectID}/messages with {text} → {turn_id}
// at once; the turn runs in the background (B-1).
func (s *Server) postMessage(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	var req PostMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		writeError(w, http.StatusBadRequest, "empty_message")
		return
	}
	turnID, err := s.starter.StartTurn(projectID, req.Text)
	// [EL DISEÑO NO LO DICE] One turn per project: a message sent
	// while a turn runs is refused with 409 turn_running.
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
