package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/LaurenteEber/tupaqnin/internal/demo/case5"
	"github.com/LaurenteEber/tupaqnin/internal/fingerprint"
	"github.com/LaurenteEber/tupaqnin/internal/pdfextract"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Shapes for backlog-r1 1.1 (P1 minimum and the left half of P2).
type CreateProjectRequest struct {
	Name     string `json:"name"`
	Template string `json:"template"` // empty · case-5
}

type ProjectView struct {
	ProjectID string `json:"project_id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

type RequestView struct {
	Text        string       `json:"text"`
	Audience    string       `json:"audience"`
	Criteria    []string     `json:"criteria"`
	Premise     string       `json:"premise"`
	Exploratory bool         `json:"exploratory"`
	Sources     []SourceView `json:"sources"`
}

type PutRequestRequest struct {
	Text        string   `json:"text"`
	Audience    string   `json:"audience"`
	Criteria    []string `json:"criteria"`
	Premise     string   `json:"premise"`
	Exploratory bool     `json:"exploratory"`
}

type SourceView struct {
	Label       string `json:"label"`
	Version     int    `json:"version"`
	Kind        string `json:"kind"`
	Filename    string `json:"filename"`
	MediaType   string `json:"media_type"`
	Fingerprint string `json:"fingerprint"`
}

type sourceBody struct {
	Kind        string `json:"kind"`
	Filename    string `json:"filename"`
	MediaType   string `json:"media_type"`
	Text        string `json:"text"`
	Fingerprint string `json:"fingerprint"`
}

func bodyMap(obj store.Object) map[string]any {
	var b map[string]any
	_ = json.Unmarshal(obj.Body, &b)
	if b == nil {
		b = map[string]any{}
	}
	return b
}

func str(b map[string]any, key string) string {
	s, _ := b[key].(string)
	return s
}

// saveRequest writes the request, its criteria and its premise in
// one transaction (specification table 1.1). Everything the human
// wrote is kept literal (fichas §2). It returns a catalog code on
// failure ("") on success.
// [EL DISEÑO NO LO DICE] re-saving with fewer criteria than before:
// criteria are updated by position and new ones are created; a
// removed criterion is not deleted (history only grows), so it
// stays current. The minimum provisional choice.
func (s *Server) saveRequest(r *http.Request, projectID string, in PutRequestRequest) string {
	err := s.db.Write(r.Context(), projectID, func(tx *store.Tx) error {
		ev := store.EventInput{Type: store.EventStateChanged, Actor: "human"}
		requests, err := s.objectsInTx(r, projectID, store.ObjectRequest)
		if err != nil {
			return err
		}
		var reqObj store.Object
		if len(requests) == 0 {
			reqObj, err = tx.CreateObject(store.ObjectRequest, "",
				map[string]any{"text": in.Text, "audience": in.Audience}, ev)
			if err != nil {
				return err
			}
		} else {
			reqObj = requests[0]
			updated, uerr := tx.UpdateObject(reqObj.ID, reqObj.CurrentVersion, "",
				map[string]any{"text": in.Text, "audience": in.Audience}, ev)
			if errors.Is(uerr, store.ErrNoChange) {
				uerr = nil
			}
			if uerr != nil {
				return uerr
			}
			if updated.ID != "" {
				reqObj = updated
			}
		}

		criteria, err := s.objectsInTx(r, projectID, store.ObjectCriterion)
		if err != nil {
			return err
		}
		mine := criteria[:0]
		for _, c := range criteria {
			if str(bodyMap(c), "request_id") == reqObj.ID {
				mine = append(mine, c)
			}
		}
		for i, text := range in.Criteria {
			body := map[string]any{"text": text, "request_id": reqObj.ID}
			if i < len(mine) {
				_, uerr := tx.UpdateObject(mine[i].ID, mine[i].CurrentVersion, "current", body, ev)
				if errors.Is(uerr, store.ErrNoChange) {
					uerr = nil
				}
				if uerr != nil {
					return uerr
				}
			} else if _, err := tx.CreateObject(store.ObjectCriterion, "current", body, ev); err != nil {
				return err
			}
		}

		premises, err := s.objectsInTx(r, projectID, store.ObjectPremise)
		if err != nil {
			return err
		}
		pbody := map[string]any{"text": in.Premise, "request_id": reqObj.ID, "exploratory": in.Exploratory}
		var prem *store.Object
		for i := range premises {
			if str(bodyMap(premises[i]), "request_id") == reqObj.ID {
				prem = &premises[i]
				break
			}
		}
		if prem == nil {
			_, err = tx.CreateObject(store.ObjectPremise, "current", pbody, ev)
			return err
		}
		_, uerr := tx.UpdateObject(prem.ID, prem.CurrentVersion, "current", pbody, ev)
		if errors.Is(uerr, store.ErrNoChange) {
			uerr = nil
		}
		return uerr
	})
	if err != nil {
		return "internal"
	}
	return ""
}

// objectsInTx lists objects through the reader pool. It is only
// called with no write of this request committed yet in other flows,
// and inside saveRequest for rows this same transaction has not
// touched except the request itself (read-your-own criteria set is
// stable within one PUT).
func (s *Server) objectsInTx(r *http.Request, projectID string, t store.ObjectType) ([]store.Object, error) {
	return s.db.ListObjects(r.Context(), projectID, t)
}

func (s *Server) requestView(r *http.Request, projectID string) (RequestView, bool) {
	view := RequestView{Criteria: []string{}, Sources: []SourceView{}}
	requests, err := s.db.ListObjects(r.Context(), projectID, store.ObjectRequest)
	if err != nil {
		return view, false
	}
	if len(requests) > 0 {
		b := bodyMap(requests[0])
		view.Text, view.Audience = str(b, "text"), str(b, "audience")
		criteria, _ := s.db.ListObjects(r.Context(), projectID, store.ObjectCriterion)
		for _, c := range criteria {
			if str(bodyMap(c), "request_id") == requests[0].ID {
				view.Criteria = append(view.Criteria, str(bodyMap(c), "text"))
			}
		}
		premises, _ := s.db.ListObjects(r.Context(), projectID, store.ObjectPremise)
		for _, p := range premises {
			b := bodyMap(p)
			if str(b, "request_id") == requests[0].ID {
				view.Premise, _ = b["text"].(string)
				view.Exploratory, _ = b["exploratory"].(bool)
			}
		}
	}
	sources, _ := s.db.ListObjects(r.Context(), projectID, store.ObjectSource)
	for _, src := range sources {
		var b sourceBody
		_ = json.Unmarshal(src.Body, &b)
		view.Sources = append(view.Sources, SourceView{Label: src.Label, Version: src.CurrentVersion,
			Kind: b.Kind, Filename: b.Filename, MediaType: b.MediaType, Fingerprint: b.Fingerprint})
	}
	return view, true
}

// POST /api/projects with {name, template}
func (s *Server) createProjectV2(w http.ResponseWriter, r *http.Request) {
	var req CreateProjectRequest
	if r.Body != nil {
		_ = json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req)
	}
	switch req.Template {
	case "":
		id, err := s.db.CreateProject(r.Context(), req.Name)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
		writeJSON(w, http.StatusCreated, CreateProjectResponse{ProjectID: id})
	case "case-5":
		id, code := s.createFromTemplate(r)
		if code != "" {
			writeError(w, http.StatusInternalServerError, code)
			return
		}
		writeJSON(w, http.StatusCreated, CreateProjectResponse{ProjectID: id})
	default:
		writeError(w, http.StatusBadRequest, "template_unknown")
	}
}

func (s *Server) createFromTemplate(r *http.Request) (string, string) {
	tpl, files, err := case5.Load()
	if err != nil {
		return "", "internal"
	}
	id, err := s.db.CreateProject(r.Context(), tpl.Name)
	if err != nil {
		return "", "internal"
	}
	if code := s.saveRequest(r, id, PutRequestRequest{Text: tpl.Text, Audience: tpl.Audience,
		Criteria: tpl.Criteria, Premise: tpl.Premise, Exploratory: tpl.Exploratory}); code != "" {
		return "", code
	}
	for _, src := range tpl.Sources {
		if _, code := s.addSource(r, id, src.Kind, src.Filename, files[src.Filename]); code != "" {
			return "", code
		}
	}
	return id, ""
}

// GET /api/projects → the project list (P1 minimum, no owner filter
// until the delivery slice: pantallas.md P1).
func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := s.db.ListProjects(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	views := make([]ProjectView, 0, len(projects))
	for _, p := range projects {
		views = append(views, ProjectView{ProjectID: p.ID, Name: p.Name, CreatedAt: p.CreatedAt})
	}
	writeJSON(w, http.StatusOK, views)
}

// GET /api/projects/{projectID}/request
func (s *Server) getRequest(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	view, ok := s.requestView(r, projectID)
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// PUT /api/projects/{projectID}/request with
// {text, audience, criteria[], premise, exploratory}
func (s *Server) putRequest(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	var req PutRequestRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if code := s.saveRequest(r, projectID, req); code != "" {
		writeError(w, http.StatusInternalServerError, code)
		return
	}
	view, _ := s.requestView(r, projectID)
	writeJSON(w, http.StatusOK, view)
}

const maxSourceBytes = 5 << 20 // specification §6: source ≤ 5 MB

// addSource stores one source. A source identical to a current one
// (same fingerprint) creates no new version (§4). It returns the
// view and a catalog code ("") on success.
func (s *Server) addSource(r *http.Request, projectID, kind, filename string, data []byte) (SourceView, string) {
	if kind != "file" && kind != "calculation" {
		return SourceView{}, "source_invalid_kind"
	}
	if len(data) > maxSourceBytes {
		return SourceView{}, "source_too_large"
	}
	var text, mediaType string
	switch strings.ToLower(path.Ext(filename)) {
	case ".md":
		text, mediaType = string(data), "text/markdown"
	case ".txt":
		text, mediaType = string(data), "text/plain"
	case ".csv":
		text, mediaType = string(data), "text/csv"
	case ".pdf":
		mediaType = "application/pdf"
		extracted, err := pdfextract.Extract(r.Context(), data)
		if err != nil {
			return SourceView{}, pdfextract.Code(err)
		}
		text = extracted
	default:
		return SourceView{}, "source_unsupported_type"
	}
	fp := fingerprint.Source(text)

	existing, err := s.db.ListObjects(r.Context(), projectID, store.ObjectSource)
	if err != nil {
		return SourceView{}, "internal"
	}
	for _, src := range existing {
		var b sourceBody
		_ = json.Unmarshal(src.Body, &b)
		if b.Fingerprint == fp {
			return SourceView{Label: src.Label, Version: src.CurrentVersion, Kind: b.Kind,
				Filename: b.Filename, MediaType: b.MediaType, Fingerprint: b.Fingerprint}, ""
		}
	}

	body := sourceBody{Kind: kind, Filename: filename, MediaType: mediaType, Text: text, Fingerprint: fp}
	var created store.Object
	err = s.db.Write(r.Context(), projectID, func(tx *store.Tx) error {
		var err error
		created, err = tx.CreateObject(store.ObjectSource, "", body,
			store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		return err
	})
	if err != nil {
		return SourceView{}, "internal"
	}
	return SourceView{Label: created.Label, Version: created.CurrentVersion, Kind: kind,
		Filename: filename, MediaType: mediaType, Fingerprint: fp}, ""
}

// POST /api/projects/{projectID}/sources (multipart: file, kind)
func (s *Server) uploadSource(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxSourceBytes+(1<<20))
	if err := r.ParseMultipartForm(maxSourceBytes); err != nil {
		writeError(w, http.StatusBadRequest, "source_too_large")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "source_missing_file")
		return
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, maxSourceBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "source_too_large")
		return
	}
	view, code := s.addSource(r, projectID, r.FormValue("kind"), header.Filename, data)
	if code != "" {
		status := http.StatusBadRequest
		if code == "internal" {
			status = http.StatusInternalServerError
		}
		if code == "source_too_large" {
			status = http.StatusRequestEntityTooLarge
		}
		writeError(w, status, code)
		return
	}
	writeJSON(w, http.StatusCreated, view)
}

// GET /api/projects/{projectID}/sources/{label}/download: the source
// text served as a download (B3-22 row 7).
func (s *Server) downloadSource(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	obj, err := s.db.GetObject(r.Context(), projectID, r.PathValue("label"))
	if err != nil || obj.Type != store.ObjectSource {
		writeError(w, http.StatusNotFound, "source_not_found")
		return
	}
	var b sourceBody
	_ = json.Unmarshal(obj.Body, &b)
	w.Header().Set("Content-Type", b.MediaType)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+b.Filename+"\"")
	_, _ = io.WriteString(w, b.Text)
}

// POST /api/projects/{projectID}/request/submit → {turn_id}: it
// records the human message and opens the understanding turn
// (1.2 will give that turn its own type; in 1.1 it runs through
// the same starter as a message with the request text).
func (s *Server) submitRequest(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if !s.requireProject(w, r, projectID) {
		return
	}
	view, _ := s.requestView(r, projectID)
	if strings.TrimSpace(view.Text) == "" {
		writeError(w, http.StatusBadRequest, "request_missing")
		return
	}
	turnID, err := s.starter.StartTurn(projectID, view.Text)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusAccepted, PostMessageResponse{TurnID: turnID})
}
