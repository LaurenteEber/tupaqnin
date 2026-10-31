package cycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/LaurenteEber/tupaqnin/internal/checks"
	"github.com/LaurenteEber/tupaqnin/internal/evidence"
	"github.com/LaurenteEber/tupaqnin/internal/fingerprint"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Verification of findings (backlog-r3 3.3; flujos §2: «la app verifica
// por hallazgo, no por retorno»; espec. §3; B-59, B-61). Each finding is
// checked on its own: whether its source exists and whether its passage
// is in it. A finding that fails never affects the others of the same
// reader. Whether the passage supports a claim is the checker's (A-2),
// never this.

// A finding's verification (glossary: `verification`) and its reason.
const (
	VerificationEntered    = "entered"
	VerificationUnverified = "unverified"
	VerificationDiscarded  = "discarded"

	ReasonPassageNotFound = "passage_not_found"
	ReasonSourceNotFound  = "source_not_found"
	ReasonInaccessible    = "inaccessible"
)

// FindingCheck is the verification of one finding, kept in the round's
// body. Part is the reader's part, or 0 for a finding of the app itself:
// a reference the round asked for, which the app looks up without a
// reader (B-59 d); Finding is the finding's number in its reader's
// findings, or the reference's number in the round's references.
// SourceID is the project source it was checked against or entered as
// (an internal id: labels only reach a model or the page).
type FindingCheck struct {
	Part         int    `json:"part"`
	Finding      int    `json:"finding"`
	Verification string `json:"verification"`
	Reason       string `json:"reason,omitempty"`
	SourceID     string `json:"source_id,omitempty"`
	Access       string `json:"access,omitempty"`
}

// readerFinding is one finding of salida-lector, as the reader wrote it.
type readerFinding struct {
	Kind      string  `json:"kind"`
	Passage   *string `json:"passage"`
	SourceID  *string `json:"source_id"`
	Hit       *string `json:"hit"`
	Reference *struct {
		Title string  `json:"title"`
		DOI   *string `json:"doi"`
	} `json:"reference"`
}

// webSource is an external source to create or reuse (B-61).
type webSource struct {
	URL      string
	DOI      string
	Title    string
	Authors  []string
	Year     int
	Access   string
	Text     string
	checkIdx []int // the checks that point at it
}

// externalRecord is what the app knows of an external source: from the
// round's hit registry or from Lookup. URL and DOI come from an API,
// never from a model (B-58).
type externalRecord struct {
	Service  string
	Title    string
	Authors  []string
	Year     int
	DOI      string
	URL      string
	OpenText bool
	Abstract string
}

// fetched is one download of a URL an API gave: its text, or that the
// page does not exist (404, 410, a domain that does not resolve).
type fetched struct {
	text     string
	notFound bool
}

// verifier holds one VerifyFindings run: the round's downloads (shared
// with read_hit) and the lookups it made, so each URL and reference is
// asked once.
type verifier struct {
	ev      Evidence
	rr      *roundRun
	lookups map[evidence.Reference]lookupResult
	web     map[string]*webSource // by DOI or URL
	order   []string
}

type lookupResult struct {
	rec evidence.Record
	err error
}

// VerifyFindings checks every finding of a round's readers once they
// end (backlog-r3 3.3) and keeps in the round's body each finding's
// verification and reason. It downloads outside any transaction; then,
// in one write conditioned on the round's version read, it creates or
// reuses (same DOI or same URL) a source of kind web for what enters
// from outside, or is «no verificado» (B-61), with source_new_version
// when its text changed. The DOI of your material is never checked
// (B-59): a finding on a project source is checked against its text
// only. FetchText only gets a URL from the round's hit registry or from
// Lookup, never one a model wrote (B-58).
func (r *Runner) VerifyFindings(ctx context.Context, projectID, roundLabel string) error {
	round, err := r.db.GetObject(ctx, projectID, roundLabel)
	if errors.Is(err, store.ErrNotFound) || (err == nil && round.Type != store.ObjectRound) {
		return ErrNotRound
	}
	if err != nil {
		return err
	}
	var body RoundBody
	if err := json.Unmarshal(round.Body, &body); err != nil {
		return fmt.Errorf("cycle: round body: %w", err)
	}
	labeler, err := graph.NewLabeler(ctx, r.db, projectID)
	if err != nil {
		return err
	}
	// The round's downloads: those its readers made with read_hit when
	// the round is still running here (runRound calls this), so a page
	// read_hit got is not downloaded again and a URL that failed keeps
	// its retry budget; a fresh registry otherwise.
	r.mu.Lock()
	rr := r.rounds[round.ID]
	r.mu.Unlock()
	if rr == nil {
		rr = newRoundRun(0)
	}
	v := &verifier{ev: r.evidenceClient(), rr: rr,
		lookups: map[evidence.Reference]lookupResult{}, web: map[string]*webSource{}}

	// The app's own findings (the round's references, B-59 d) were
	// checked when the round started; they stay as they are.
	checksOut := []FindingCheck{}
	for _, c := range body.Findings {
		if c.Part == 0 {
			checksOut = append(checksOut, c)
		}
	}
	hits := map[string]HitRecord{}
	for _, h := range body.Hits {
		hits[h.Label] = h
	}
	for _, rd := range body.Readers {
		if rd.Status != ReaderReturned {
			continue
		}
		var out struct {
			Findings []readerFinding `json:"findings"`
		}
		if err := json.Unmarshal(rd.Output, &out); err != nil {
			continue
		}
		for i, f := range out.Findings {
			c, ext, ok := v.check(ctx, r, projectID, labeler, hits, f)
			if !ok {
				continue
			}
			c.Part, c.Finding = rd.Part, i+1
			checksOut = append(checksOut, c)
			if ext != nil {
				v.addWeb(ext, c, len(checksOut)-1)
			}
		}
	}

	return r.db.Write(ctx, projectID, func(tx *store.Tx) error {
		sources, err := tx.ListObjects(ctx, projectID, store.ObjectSource)
		if err != nil {
			return err
		}
		for _, key := range v.order {
			ws := v.web[key]
			id, err := writeWebSource(ctx, tx, sources, ws, body.TurnID)
			if err != nil {
				return err
			}
			for _, idx := range ws.checkIdx {
				checksOut[idx].SourceID = id
			}
			if src, err := tx.Object(id); err == nil {
				sources = appendOrReplace(sources, src)
			}
		}
		body.Findings = checksOut
		counts := map[string]int{}
		for _, c := range checksOut {
			counts[c.Verification]++
		}
		_, err = tx.UpdateObject(round.ID, round.CurrentVersion, round.State, body, store.EventInput{
			Type: store.EventStateChanged, Actor: "app", TurnID: body.TurnID,
			Data: map[string]any{"to": round.State, "findings_verified": len(checksOut),
				VerificationEntered: counts[VerificationEntered], VerificationUnverified: counts[VerificationUnverified],
				VerificationDiscarded: counts[VerificationDiscarded]},
		})
		if errors.Is(err, store.ErrNoChange) {
			return nil
		}
		return err
	})
}

// check verifies one finding. It returns false for a finding with
// nothing to check (a gap without passage nor source). The record is
// set when the finding is about an external source that enters or is
// «no verificado» (B-61): it becomes a web source.
// [EL DISEÑO NO LO DICE]: a finding that is not a gap and has no
// passage is discarded with passage_not_found; one with a passage and
// no source, with source_not_found; a hit-n the round never numbered,
// with source_not_found.
func (v *verifier) check(ctx context.Context, r *Runner, projectID string, labeler *graph.Labeler,
	hits map[string]HitRecord, f readerFinding) (FindingCheck, *externalRecord, bool) {
	passage := ""
	if f.Passage != nil {
		passage = strings.TrimSpace(*f.Passage)
	}
	named := f.SourceID != nil || f.Hit != nil || f.Reference != nil
	if passage == "" {
		if f.Kind == "gap" || !named {
			return FindingCheck{}, nil, false
		}
		return FindingCheck{Verification: VerificationDiscarded, Reason: ReasonPassageNotFound}, nil, true
	}
	switch {
	case f.SourceID != nil:
		return v.checkProjectSource(ctx, r, projectID, labeler, *f.SourceID, passage), nil, true
	case f.Hit != nil:
		h, ok := hits[*f.Hit]
		if !ok {
			return FindingCheck{Verification: VerificationDiscarded, Reason: ReasonSourceNotFound}, nil, true
		}
		rec := externalRecord{Service: h.Service, Title: h.Title, Authors: h.Authors, Year: h.Year, DOI: h.DOI,
			URL: h.URL, OpenText: h.OpenText, Abstract: h.Abstract}
		return v.checkExternal(ctx, rec, passage)
	case f.Reference != nil:
		// B-58: only the DOI and the title are used to look a written
		// reference up; nothing it says is ever downloaded.
		ref := evidence.Reference{Title: f.Reference.Title}
		if f.Reference.DOI != nil {
			ref.DOI = *f.Reference.DOI
		}
		res, seen := v.lookups[ref]
		if !seen {
			res.rec, res.err = v.ev.Lookup(ctx, ref)
			v.lookups[ref] = res
		}
		switch {
		case errors.Is(res.err, evidence.ErrNotFound):
			return FindingCheck{Verification: VerificationDiscarded, Reason: ReasonSourceNotFound}, nil, true
		case res.err != nil:
			// [EL DISEÑO NO LO DICE]: the registry did not answer, so
			// whether the source exists is unknown: «no verificado», and
			// no source is stored (it has no identity the app checked).
			return FindingCheck{Verification: VerificationUnverified, Reason: ReasonInaccessible}, nil, true
		}
		rec := externalRecord{Service: res.rec.Service, Title: res.rec.Title, Authors: res.rec.Authors,
			Year: res.rec.Year, DOI: res.rec.DOI, URL: res.rec.URL, OpenText: res.rec.OpenText, Abstract: res.rec.Abstract}
		return v.checkExternal(ctx, rec, passage)
	}
	return FindingCheck{Verification: VerificationDiscarded, Reason: ReasonSourceNotFound}, nil, true
}

// checkProjectSource is B-59 (a): the passage, normalized, is in the
// source's text → entered; if not → discarded. The DOI of your material
// is never checked. A web source stored without text (access none) is
// «no verificado».
func (v *verifier) checkProjectSource(ctx context.Context, r *Runner, projectID string, labeler *graph.Labeler,
	label, passage string) FindingCheck {
	id, err := labeler.ToID(label)
	if err != nil {
		return FindingCheck{Verification: VerificationDiscarded, Reason: ReasonSourceNotFound}
	}
	src, err := r.db.GetObject(ctx, projectID, id)
	if err != nil || src.Type != store.ObjectSource {
		return FindingCheck{Verification: VerificationDiscarded, Reason: ReasonSourceNotFound}
	}
	b := bodyOf(src)
	access := ""
	if bodyStr(b, "kind") == graph.SourceKindWeb {
		access = bodyStr(b, "access")
		if access == graph.AccessNone {
			return FindingCheck{Verification: VerificationUnverified, Reason: ReasonInaccessible, SourceID: src.ID,
				Access: access}
		}
	}
	if passageIn(bodyStr(b, "text"), passage) {
		return FindingCheck{Verification: VerificationEntered, SourceID: src.ID, Access: access}
	}
	return FindingCheck{Verification: VerificationDiscarded, Reason: ReasonPassageNotFound, SourceID: src.ID}
}

// checkExternal is B-59 (b) and (c) for a source whose record exists:
// the passage in the open text → entered with full_text; in the abstract
// → entered with abstract; the full text accessible and the passage not
// in it → discarded; only the abstract (a paywall) or nothing → «no
// verificado». A URL without DOI (c) that answers 404 or 410, or whose
// domain does not resolve, is «fuente inexistente».
func (v *verifier) checkExternal(ctx context.Context, rec externalRecord, passage string) (FindingCheck, *externalRecord, bool) {
	full := ""
	if rec.URL != "" && (rec.OpenText || rec.DOI == "") {
		page := v.fetch(ctx, rec.URL)
		if page.notFound && rec.DOI == "" {
			return FindingCheck{Verification: VerificationDiscarded, Reason: ReasonSourceNotFound}, nil, true
		}
		full = page.text
	}
	switch {
	case full != "" && passageIn(full, passage):
		return FindingCheck{Verification: VerificationEntered, Access: graph.AccessFullText}, withText(rec, full), true
	case rec.Abstract != "" && passageIn(rec.Abstract, passage):
		return FindingCheck{Verification: VerificationEntered, Access: graph.AccessAbstract}, withText(rec, ""), true
	case full != "":
		return FindingCheck{Verification: VerificationDiscarded, Reason: ReasonPassageNotFound}, nil, true
	}
	return FindingCheck{Verification: VerificationUnverified, Reason: ReasonInaccessible, Access: graph.AccessNone},
		withText(rec, ""), true
}

// withText is the record with the text the source would store: the
// downloaded full text when there is one, else its abstract (addWeb
// reads it from Abstract).
func withText(rec externalRecord, full string) *externalRecord {
	out := rec
	if full != "" {
		out.Abstract = full
	}
	return &out
}

// fetch downloads a URL an API gave through the round's downloads: at
// most one retry after a failure that may pass, counting what read_hit
// already tried (backlog-r3 3.3, «No hacer»: never more than one).
func (v *verifier) fetch(ctx context.Context, url string) fetched {
	return v.rr.download(url).fetch(ctx, v.ev, url, maxDownloadAttempts)
}

// addWeb plans the web source a finding enters as or is «no verificado»
// as (B-61), one per DOI or URL in the run; the best access wins
// (full_text, then abstract, then none).
func (v *verifier) addWeb(rec *externalRecord, c FindingCheck, idx int) {
	key := webKey(rec.DOI, rec.URL)
	if key == "" {
		return
	}
	text := ""
	if c.Access != graph.AccessNone {
		text = rec.Abstract
	}
	ws, ok := v.web[key]
	if !ok {
		ws = &webSource{URL: rec.URL, DOI: rec.DOI, Title: rec.Title, Authors: rec.Authors, Year: rec.Year,
			Access: c.Access, Text: text}
		v.web[key] = ws
		v.order = append(v.order, key)
	} else if accessRank(c.Access) > accessRank(ws.Access) {
		ws.Access, ws.Text = c.Access, text
	}
	ws.checkIdx = append(ws.checkIdx, idx)
}

func webKey(doi, url string) string {
	if doi != "" {
		return "doi:" + strings.ToLower(doi)
	}
	if url != "" {
		return "url:" + url
	}
	return ""
}

func accessRank(a string) int {
	switch a {
	case graph.AccessFullText:
		return 2
	case graph.AccessAbstract:
		return 1
	}
	return 0
}

// writeWebSource creates the web source, or reuses the one with the
// same DOI or the same URL (B-61). A reused source gets a new version
// (source_new_version, with the warning to the sections that use it,
// A-1) only when its text changed; a source the app holds with more
// access is never downgraded. It returns the source's id.
// [EL DISEÑO NO LO DICE]: the stored text is the extracted text, as for
// an uploaded PDF (B-9), with media_type text/plain; a reused source
// whose new check has less access keeps what it had.
func writeWebSource(ctx context.Context, tx *store.Tx, sources []store.Object, ws *webSource, turnID string) (string, error) {
	authors := ws.Authors
	if authors == nil {
		authors = []string{}
	}
	body := map[string]any{"kind": graph.SourceKindWeb, "url": ws.URL, "media_type": "text/plain",
		"text": ws.Text, "doi": ws.DOI, "title": ws.Title, "authors": authors, "year": ws.Year,
		"access": ws.Access}
	if ws.Text != "" {
		body["fingerprint"] = fingerprint.Source(ws.Text)
	} else {
		body["fingerprint"] = ""
	}
	for _, src := range sources {
		b := bodyOf(src)
		if bodyStr(b, "kind") != graph.SourceKindWeb {
			continue
		}
		sameDOI := ws.DOI != "" && strings.EqualFold(bodyStr(b, "doi"), ws.DOI)
		sameURL := ws.URL != "" && bodyStr(b, "url") == ws.URL
		if !sameDOI && !sameURL {
			continue
		}
		if accessRank(ws.Access) < accessRank(bodyStr(b, "access")) || bodyStr(b, "fingerprint") == body["fingerprint"] {
			return src.ID, nil
		}
		if _, err := tx.UpdateObject(src.ID, src.CurrentVersion, src.State, body, store.EventInput{
			Type: store.EventSourceNewVersion, Actor: "app", TurnID: turnID,
			Data: map[string]any{"label": src.Label, "url": ws.URL, "doi": ws.DOI},
		}); err != nil && !errors.Is(err, store.ErrNoChange) {
			return "", err
		}
		if _, err := checks.Warn(ctx, tx, "app", turnID, []string{src.ID}, nil); err != nil {
			return "", err
		}
		return src.ID, nil
	}
	created, err := tx.CreateObject(store.ObjectSource, "", body, store.EventInput{
		Type: store.EventStateChanged, Actor: "app", TurnID: turnID,
		Data: map[string]any{"kind": graph.SourceKindWeb, "access": ws.Access},
	})
	if err != nil {
		return "", err
	}
	return created.ID, nil
}

func appendOrReplace(list []store.Object, obj store.Object) []store.Object {
	for i, o := range list {
		if o.ID == obj.ID {
			list[i] = obj
			return list
		}
	}
	return append(list, obj)
}

// passageIn reports whether passage is literal in text, both normalized
// as B-59 (a) says: NFC, spaces collapsed, quotes and dashes unified,
// letter case ignored ([inicial]).
func passageIn(text, passage string) bool {
	p := verifyForm(passage)
	return p != "" && strings.Contains(verifyForm(text), p)
}

var unifyQuotesAndDashes = strings.NewReplacer(
	"‘", "'", "’", "'", "‚", "'", "‛", "'", "′", "'", "´", "'", "`", "'",
	"“", "\"", "”", "\"", "„", "\"", "‟", "\"", "″", "\"", "«", "\"", "»", "\"",
	"‐", "-", "‑", "-", "‒", "-", "–", "-", "—", "-", "―", "-", "−", "-",
	"﹘", "-", "﹣", "-", "－", "-",
)

func verifyForm(s string) string {
	return strings.ToLower(unifyQuotesAndDashes.Replace(graph.CompareString(s)))
}

// lookupRoundReferences is B-59 (d): the references the round asks for
// (B-69) the app looks up with Lookup, without a reader, before the
// readers start. One that does not exist is the app's finding,
// discarded with source_not_found; one that exists enters the round's
// results as a hit-n (part 0) for the readers.
// [EL DISEÑO NO LO DICE]: a reference the registry could not answer is
// «no verificado» (inaccessible), not discarded.
func (r *Runner) lookupRoundReferences(ctx context.Context, refs []namedReference, rr *roundRun) []FindingCheck {
	out := []FindingCheck{}
	ev := r.evidenceClient()
	for i, ref := range refs {
		q := evidence.Reference{}
		if ref.DOI != nil {
			q.DOI = *ref.DOI
		}
		if ref.Title != nil {
			q.Title = *ref.Title
		}
		rec, err := ev.Lookup(ctx, q)
		switch {
		case errors.Is(err, evidence.ErrNotFound):
			out = append(out, FindingCheck{Finding: i + 1, Verification: VerificationDiscarded, Reason: ReasonSourceNotFound})
		case err != nil:
			out = append(out, FindingCheck{Finding: i + 1, Verification: VerificationUnverified, Reason: ReasonInaccessible})
		default:
			rr.register(0, []evidence.Hit{{Service: rec.Service, ID: rec.ID, Title: rec.Title, Authors: rec.Authors,
				Year: rec.Year, DOI: rec.DOI, URL: rec.URL, OpenText: rec.OpenText, Abstract: rec.Abstract}})
		}
	}
	return out
}
