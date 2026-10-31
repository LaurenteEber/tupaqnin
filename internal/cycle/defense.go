package cycle

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// The judge's defense instructions (a draft Eber reviews) and its fixed
// output, a copy of salida-juez-defensa.schema.json (backlog-r2 2.6).
//
//go:embed prompts/judge-defense.en.md
var judgeDefenseInstructions string

//go:embed schema/judge_defense_output.schema.json
var judgeDefenseSchema []byte

// The answers to an audience question (estados §9): «sí» and «no».
const (
	AnswerYes = "yes"
	AnswerNo  = "no"
)

// DefenseFailed is the result of the judge's output_retry after its
// second failure (fichas §7): it stays visible in the report and blocks
// nothing.
// [EL DISEÑO NO LO DICE]: the result's name; not agent_pending, which
// would block the signoff (B-15), and the defense never blocks.
const DefenseFailed = "defense_failed"

var (
	ErrAudienceQuestionNotFound = errors.New("cycle: audience question not found")
	ErrAudienceQuestionNotOpen  = errors.New("cycle: audience question is not open")
	ErrInvalidAudienceAnswer    = errors.New("cycle: the answer is neither yes nor no")
)

// DefenseQuestion is one likely question of the audience (fichas §7).
type DefenseQuestion struct {
	Question string `json:"question"`
	Section  string `json:"section"`
}

// DefenseNotCovered is one thing the work does not cover (fichas §7).
type DefenseNotCovered struct {
	Statement string `json:"statement"`
	Section   string `json:"section"`
}

// DefenseOutput is the judge's fixed output at the stop.
type DefenseOutput struct {
	AudienceQuestions []DefenseQuestion   `json:"audience_questions"`
	NotCovered        []DefenseNotCovered `json:"not_covered"`
}

var (
	defenseSchemaOnce sync.Once
	defenseSchema     *jsonschema.Schema
	defenseSendable   json.RawMessage
	defenseSchemaErr  error
)

func judgeDefenseSchemas() (*jsonschema.Schema, json.RawMessage, error) {
	defenseSchemaOnce.Do(func() {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(judgeDefenseSchema))
		if err != nil {
			defenseSchemaErr = fmt.Errorf("cycle: parse judge defense schema: %w", err)
			return
		}
		c := jsonschema.NewCompiler()
		const loc = "https://tupaqnin.invalid/judge_defense_output.schema.json"
		if err := c.AddResource(loc, doc); err != nil {
			defenseSchemaErr = fmt.Errorf("cycle: add judge defense schema: %w", err)
			return
		}
		if defenseSchema, err = c.Compile(loc); err != nil {
			defenseSchemaErr = fmt.Errorf("cycle: compile judge defense schema: %w", err)
			return
		}
		defenseSendable, defenseSchemaErr = provider.TransportSchema(judgeDefenseSchema)
	})
	return defenseSchema, defenseSendable, defenseSchemaErr
}

// validateDefense checks the judge's output against its fixed shape: the
// schema, and that every entry names the section in question (rule 4).
// [EL DISEÑO NO LO DICE]: an entry on another section is an error of
// shape (retried), since the judge only received this one.
func validateDefense(raw, section string) ([]string, DefenseOutput, error) {
	var out DefenseOutput
	sch, _, err := judgeDefenseSchemas()
	if err != nil {
		return nil, out, err
	}
	inst, err := jsonschema.UnmarshalJSON(strings.NewReader(raw))
	if err != nil {
		return []string{"/: output is not valid JSON"}, out, nil
	}
	if err := sch.Validate(inst); err != nil {
		var verr *jsonschema.ValidationError
		if !errors.As(err, &verr) {
			return nil, out, fmt.Errorf("cycle: validate judge defense: %w", err)
		}
		return flattenErrors(verr), out, nil
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, out, fmt.Errorf("cycle: parse judge defense: %w", err)
	}
	var errs []string
	for i, q := range out.AudienceQuestions {
		if q.Section != section {
			errs = append(errs, fmt.Sprintf("/audience_questions/%d/section: %s is not the section in question, %s", i, q.Section, section))
		}
	}
	for i, n := range out.NotCovered {
		if n.Section != section {
			errs = append(errs, fmt.Sprintf("/not_covered/%d/section: %s is not the section in question, %s", i, n.Section, section))
		}
	}
	return errs, out, nil
}

// CallJudgeDefense makes the judge's one call for a section's defense
// (backlog-r2 2.6, fichas §7): Spark 1.3 high, a clean context with its
// reduced brief, the fixed shape and one retry. Each output that fails
// the shape, or a failed call, appends output_retry of turn kind
// judge_defense: result retry after the first, DefenseFailed after the
// second; ok is false then. No transaction is open while the judge
// answers (specification §2). The micro-test of the judge records with
// it.
func CallJudgeDefense(ctx context.Context, prov provider.Provider, db *store.DB, projectID, turnID, section string) (DefenseOutput, bool, error) {
	_, schema, err := judgeDefenseSchemas()
	if err != nil {
		return DefenseOutput{}, false, err
	}
	brief, err := graph.DefenseBrief(ctx, db, projectID, section)
	if err != nil {
		return DefenseOutput{}, false, err
	}
	input := []json.RawMessage{userItem(brief)}
	for attempt := 1; attempt <= 2; attempt++ {
		resp, err := prov.Respond(ctx, provider.Request{
			ProjectID: projectID, TurnID: turnID,
			Role: provider.RoleJudge, TurnKind: provider.TurnJudgeDefense,
			Model: provider.JudgeDefenseModel, Effort: provider.JudgeDefenseEffort,
			Instructions: judgeDefenseInstructions, Input: input, Schema: schema, SchemaStrict: true,
			DefenseSection: section,
		}, nil)
		var errs []string
		var out DefenseOutput
		if err != nil {
			if ctx.Err() != nil {
				return DefenseOutput{}, false, ctx.Err()
			}
			// The provider's error body is never kept (T-10).
			errs = []string{"provider_error"}
		} else if errs, out, err = validateDefense(resp.Text, section); err != nil {
			return DefenseOutput{}, false, err
		}
		if len(errs) == 0 {
			return out, true, nil
		}
		data := map[string]any{"turn_kind": string(provider.TurnJudgeDefense), "attempt": attempt,
			"errors": errs, "result": "retry", "section": section}
		if attempt == 2 {
			data["result"] = DefenseFailed
		}
		err = db.Write(ctx, projectID, func(tx *store.Tx) error {
			_, err := tx.AppendEvent(store.EventInput{Type: store.EventOutputRetry, Actor: "agent",
				AgentRole:      string(provider.RoleJudge),
				ModelAndEffort: provider.JudgeDefenseModel + " " + provider.JudgeDefenseEffort,
				TurnID:         turnID, Data: data})
			return err
		})
		if err != nil {
			return DefenseOutput{}, false, err
		}
		if resp.RawItems != nil {
			input = append(append(input, resp.RawItems...), userItem(RetryMessage(errs)))
		}
	}
	return DefenseOutput{}, false, nil
}

// prepareDefense is the defense at the stop (backlog-r2 2.6): when the
// report proposes signing a section that B-38 or its role ask for, one
// judge call, and each question and each «no cubre» becomes an open
// audience question with its about edge, in one transaction. The judge
// failing twice leaves its output_retry and nothing else.
// P-76 (fichas §2 «one round»): a stop prepares it again only when the
// section's text has another fingerprint than at the last preparation;
// otherwise no judge call, and its open questions keep showing.
// [EL DISEÑO NO LO DICE]: the judge's entries keep its literal text, a
// «no cubre» marked not_covered in the body.
func (r *Runner) prepareDefense(ctx context.Context, projectID, turnID, section string) error {
	needed, err := graph.DefenseNeeded(ctx, r.db, projectID, section)
	if err != nil || !needed {
		return err
	}
	if same, err := sameTextAsLastDefense(ctx, r.db, projectID, section); err != nil || same {
		return err
	}
	out, ok, err := CallJudgeDefense(ctx, r.prov, r.db, projectID, turnID, section)
	if err != nil || !ok {
		return err
	}
	return r.db.Write(ctx, projectID, func(tx *store.Tx) error {
		sec, err := tx.GetObject(ctx, projectID, section)
		if err != nil {
			return err
		}
		judge := store.EventInput{Type: store.EventStateChanged, Actor: "agent", AgentRole: string(provider.RoleJudge),
			ModelAndEffort: provider.JudgeDefenseModel + " " + provider.JudgeDefenseEffort, TurnID: turnID}
		create := func(text string, notCovered bool) error {
			body := map[string]any{"question": text, "section_id": sec.ID}
			if notCovered {
				body["not_covered"] = true
			}
			ev := judge
			ev.Data = map[string]any{"to": "open", "section": sec.Label}
			aq, err := tx.CreateObject(store.ObjectAudienceQuestion, "open", body, ev)
			if err != nil {
				return err
			}
			edge := judge
			edge.Type = store.EventEdge
			_, err = tx.AddEdge(store.EdgeAbout, aq.ID, sec.ID, "", nil, edge)
			return err
		}
		for _, q := range out.AudienceQuestions {
			if err := create(q.Question, false); err != nil {
				return err
			}
		}
		for _, n := range out.NotCovered {
			if err := create(n.Statement, true); err != nil {
				return err
			}
		}
		return nil
	})
}

// sectionsNotReady are the labels of the project's sections that could
// not be signed now (graph.SectionClear), read before your answer so
// defendInBackground knows which ones your answer left ready (P-86).
func (r *Runner) sectionsNotReady(ctx context.Context, projectID string) (map[string]bool, error) {
	sections, err := graph.OrderedSections(ctx, r.db, projectID)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, sec := range sections {
		ok, err := graph.SectionClear(ctx, r.db, projectID, sec.Label)
		if err != nil {
			return nil, err
		}
		if !ok {
			out[sec.Label] = true
		}
	}
	return out, nil
}

// afterYourAnswer runs what your answer to a card or a «por aclarar»
// leaves to do, once the answer is saved: the queued probes when you
// changed the premise (P-72), in the answer's request and in the turn
// slot, as before; then the defense of each section your answer left
// ready (P-86, backlog-r2 2.6, fichas §8.2), with the conditions and the
// P-76 rule of prepareDefense, in the background (P-90: the answer
// returns at once; the audience questions show when the conversation
// updates). A probe's failure is returned, as before; before is nil
// when the sections could not be read before the answer, and then no
// defense runs.
func (r *Runner) afterYourAnswer(ctx context.Context, projectID string, before map[string]bool, probes bool) error {
	if probes && r.tryAcquireTurn(projectID) {
		turnID, err := newTurnID()
		if err == nil {
			err = r.runQueuedProbes(ctx, projectID, turnID)
		}
		r.releaseTurn(projectID)
		if err != nil {
			return err
		}
	}
	r.defendInBackground(projectID, before)
	return nil
}

// defendInBackground prepares, apart from the answer's request, the
// defense of each section among before that your answer left ready
// (P-90). It keeps the turn slot rule: with a turn of the project
// running, the defense waits for it in the runner and runs when the
// slot is released (P-91). A failure blocks nothing and is logged by code only.
// [EL DISEÑO NO LO DICE]: the defense waits in memory, not in the
// history (P-90, 2.5 proportionality): a restart in between drops it,
// and the next stop that proposes signing the section prepares it.
// After Close it does nothing; Close waits for one in flight.
func (r *Runner) defendInBackground(projectID string, before map[string]bool) {
	if len(before) == 0 {
		return
	}
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.answers.Add(1)
	r.mu.Unlock()
	go func() {
		ready, err := readyAmong(r.ctx, r.db, projectID, before)
		if err != nil {
			r.logAfterAnswer(r.ctx, "ready_sections_error")
			r.answers.Done()
			return
		}
		r.queueDefenses(projectID, ready)
		r.runDefensesInBackground(projectID)
	}()
}

// runDefensesInBackground takes the project's turn slot and prepares
// its queued defenses; with the slot held, the holder's release
// launches them (P-91). The caller counted it in answers.
func (r *Runner) runDefensesInBackground(projectID string) {
	defer r.answers.Done()
	if !r.tryAcquireTurn(projectID) {
		return // releaseTurn launches them when the slot frees
	}
	defer r.releaseTurn(projectID)
	r.runQueuedDefenses(r.ctx, projectID)
}

// logAfterAnswer logs a failure after your saved answer by its code
// only (P-90, T-10): never the error's text.
func (r *Runner) logAfterAnswer(ctx context.Context, code string) {
	if ctx.Err() == nil && r.Logger != nil {
		r.Logger.Error("after your answer failed", "code", code)
	}
}

// notReadyBeforeYourAnswer is sectionsNotReady for an answer: its
// failure never fails the answer (P-90); it is logged by code and no
// defense follows.
func (r *Runner) notReadyBeforeYourAnswer(ctx context.Context, projectID string) map[string]bool {
	before, err := r.sectionsNotReady(ctx, projectID)
	if err != nil {
		r.logAfterAnswer(ctx, "ready_sections_error")
		return nil
	}
	return before
}

// readyAmong are the labels, among those given, of the sections that
// could be signed now, in the project's order.
func readyAmong(ctx context.Context, db *store.DB, projectID string, labels map[string]bool) ([]string, error) {
	sections, err := graph.OrderedSections(ctx, db, projectID)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, sec := range sections {
		if !labels[sec.Label] {
			continue
		}
		ok, err := graph.SectionClear(ctx, db, projectID, sec.Label)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, sec.Label)
		}
	}
	return out, nil
}

// queueDefenses keeps the sections whose defense waits for the turn
// slot (P-86), each once.
func (r *Runner) queueDefenses(projectID string, labels []string) {
	if len(labels) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.defenses == nil {
		r.defenses = map[string][]string{}
	}
	for _, label := range labels {
		if !slices.Contains(r.defenses[projectID], label) {
			r.defenses[projectID] = append(r.defenses[projectID], label)
		}
	}
}

// runQueuedDefenses prepares the defense of each queued section still
// ready, with a turn id of its own, so the conversation shows its
// questions apart from any stop report (P-86). The caller holds the
// project's turn slot.
func (r *Runner) runQueuedDefenses(ctx context.Context, projectID string) {
	r.mu.Lock()
	labels := r.defenses[projectID]
	delete(r.defenses, projectID)
	r.mu.Unlock()
	if len(labels) == 0 {
		return
	}
	turnID, err := newTurnID()
	for _, label := range labels {
		if err != nil {
			break
		}
		var ok bool
		if ok, err = graph.SectionClear(ctx, r.db, projectID, label); err != nil || !ok {
			continue
		}
		err = r.prepareDefense(ctx, projectID, turnID, label)
	}
	if err != nil {
		r.logAfterAnswer(ctx, "defense_error")
	}
}

// sameTextAsLastDefense reports whether the section's text has the
// fingerprint it had at its last defense preparation (P-76), derived
// from the history, which only grows: the last preparation is the last
// audience question the judge opened about the section, and the text it
// was prepared on is the section's version just before that event.
// [EL DISEÑO NO LO DICE]: a preparation is one that opened questions; a
// judge that failed twice prepared nothing, so the next stop tries again.
func sameTextAsLastDefense(ctx context.Context, db *store.DB, projectID, label string) (bool, error) {
	sec, err := db.GetObject(ctx, projectID, label)
	if err != nil {
		return false, err
	}
	events, err := db.EventsSince(ctx, projectID, 0)
	if err != nil {
		return false, err
	}
	version, prepared := 0, 0
	for _, ev := range events {
		if ev.ObjectID == sec.ID && ev.NewVersion > 0 {
			version = ev.NewVersion
			continue
		}
		if ev.Type != store.EventStateChanged || ev.AgentRole != string(provider.RoleJudge) {
			continue
		}
		var data map[string]any
		_ = json.Unmarshal(ev.Data, &data)
		if data["to"] == "open" && data["section"] == sec.Label {
			prepared = version
		}
	}
	if prepared == 0 {
		return false, nil
	}
	return sameSectionText(ctx, db, projectID, sec, prepared)
}

// AnswerAudienceQuestion answers an open audience question (estados §9,
// backlog-r2 2.6): «yes» leaves it addressed, an instruction of yours
// that enters the principal's next turn without opening one; «no»
// leaves it declined with audience_question_declined («no abordado,
// decidido por ti»). No «porque» is asked. It never touches the signoff.
// [EL DISEÑO NO LO DICE]: an answered question cannot be answered again.
func (g *Gate) AnswerAudienceQuestion(ctx context.Context, projectID, label, answer string) error {
	if answer != AnswerYes && answer != AnswerNo {
		return ErrInvalidAudienceAnswer
	}
	return g.db.Write(ctx, projectID, func(tx *store.Tx) error {
		aq, err := tx.GetObject(ctx, projectID, label)
		if errors.Is(err, store.ErrNotFound) || (err == nil && aq.Type != store.ObjectAudienceQuestion) {
			return ErrAudienceQuestionNotFound
		}
		if err != nil {
			return err
		}
		if aq.State != "open" {
			return ErrAudienceQuestionNotOpen
		}
		var body map[string]any
		if err := json.Unmarshal(aq.Body, &body); err != nil {
			return fmt.Errorf("cycle: parse audience question: %w", err)
		}
		state := "addressed"
		ev := store.EventInput{Type: store.EventStateChanged, Actor: "human",
			Data: map[string]any{"label": aq.Label, "from": "open", "to": "addressed"}}
		if answer == AnswerNo {
			state = "declined"
			ev = store.EventInput{Type: store.EventAudienceQuestionDeclined, Actor: "human",
				Data: map[string]any{"label": aq.Label, "from": "open", "to": "declined",
					"text": "not addressed, decided by you"}}
		}
		// Conditioned on the version just read (AGENTS rule 6).
		_, err = tx.UpdateObject(aq.ID, aq.CurrentVersion, state, body, ev)
		return err
	})
}

// DefenseFailedSince reports whether the judge failed its shape twice
// after the stop report of seq (fichas §7: the report shows it).
func DefenseFailedSince(ctx context.Context, q store.Querier, projectID string, seq int) (bool, error) {
	events, err := q.EventsSince(ctx, projectID, seq)
	if err != nil {
		return false, err
	}
	for _, ev := range events {
		if ev.Type != store.EventOutputRetry {
			continue
		}
		var data map[string]any
		_ = json.Unmarshal(ev.Data, &data)
		if data["turn_kind"] == string(provider.TurnJudgeDefense) && data["result"] == DefenseFailed {
			return true, nil
		}
	}
	return false, nil
}
