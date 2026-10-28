//go:build meta

// Micro-test 0.8 (backlog): what Meta's strict mode admits of the
// principal's fixed-output schema, how often Spark's answers comply
// with the full schema validated in Go, what each call costs, and
// recorded real responses for the fake provider (T-11).
//
// It runs only with -tags=meta and META_API_KEY set:
//
//	make microtest
//
// No Meta call fails this test: every result is logged as it arrives
// and noted in the report, which is written even on failure. The
// report lands in docs/casos-prueba/micro-prueba-0.8/informe.md and
// the recordings in internal/provider/testdata/microtest-*.json.
package microtest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dlclark/regexp2"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/LaurenteEber/tupaqnin/internal/provider"
)

const (
	schemaPath  = "../../../docs/diseno/bloque-3/salida-principal.schema.json"
	testdataDir = "../testdata"
	reportPath  = "../../../docs/casos-prueba/micro-prueba-0.8/informe.md"
)

// ecmaRegexp adapts regexp2 to the validator: JSON Schema patterns
// are ECMA-262 (the principal schema uses a negative lookahead,
// ^(?!new-)…), which Go's RE2 cannot parse.
type ecmaRegexp regexp2.Regexp

func (re *ecmaRegexp) MatchString(s string) bool {
	matched, err := (*regexp2.Regexp)(re).MatchString(s)
	return err == nil && matched
}

func (re *ecmaRegexp) String() string { return (*regexp2.Regexp)(re).String() }

func ecmaEngine(s string) (jsonschema.Regexp, error) {
	re, err := regexp2.Compile(s, regexp2.ECMAScript)
	if err != nil {
		return nil, err
	}
	re.MatchTimeout = time.Second
	return (*ecmaRegexp)(re), nil
}

// userItem builds one input message in the shape Meta's Responses
// docs use: a message item with an input_text content block.
func userItem(text string) json.RawMessage {
	b, _ := json.Marshal(map[string]any{
		"type": "message", "role": "user",
		"content": []map[string]string{{"type": "input_text", "text": text}},
	})
	return b
}

func compileSchema(t *testing.T, raw []byte) *jsonschema.Schema {
	t.Helper()
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("parse principal schema: %v", err)
	}
	c := jsonschema.NewCompiler()
	c.UseRegexpEngine(ecmaEngine)
	const loc = "https://tupaqnin.test/salida-principal.schema.json"
	if err := c.AddResource(loc, doc); err != nil {
		t.Fatalf("add schema resource: %v", err)
	}
	sch, err := c.Compile(loc)
	if err != nil {
		t.Fatalf("compile principal schema: %v", err)
	}
	return sch
}

func validateInstance(sch *jsonschema.Schema, inst any) error { return sch.Validate(inst) }

func decodeInstance(text string) (any, error) {
	return jsonschema.UnmarshalJSON(strings.NewReader(text))
}

// stripOptionalNulls removes, recursively, the null values of
// properties that were optional in the full schema, so an output
// produced against the transport schema (where optionals are
// nullable) can be validated against the full schema (B-2).
func stripOptionalNulls(inst any, schema map[string]any, root map[string]any) {
	if ref, ok := schema["$ref"].(string); ok {
		if target, ok := resolveRef(ref, root); ok {
			stripOptionalNulls(inst, target, root)
		}
		return
	}
	obj, ok := inst.(map[string]any)
	if !ok {
		return
	}
	props, _ := schema["properties"].(map[string]any)
	required := make(map[string]bool)
	if req, ok := schema["required"].([]any); ok {
		for _, r := range req {
			if name, ok := r.(string); ok {
				required[name] = true
			}
		}
	}
	for name, value := range obj {
		propSchema, _ := props[name].(map[string]any)
		if value == nil && !required[name] && propSchema != nil {
			delete(obj, name)
			continue
		}
		if propSchema != nil {
			stripOptionalNullsInValue(value, propSchema, root)
		}
	}
}

func stripOptionalNullsInValue(inst any, schema map[string]any, root map[string]any) {
	if branches, ok := schema["anyOf"].([]any); ok {
		for _, branch := range branches {
			if b, ok := branch.(map[string]any); ok {
				stripOptionalNulls(inst, b, root)
			}
		}
		return
	}
	if arr, ok := inst.([]any); ok {
		if items, ok := schema["items"].(map[string]any); ok {
			for _, item := range arr {
				stripOptionalNullsInValue(item, items, root)
			}
		}
		return
	}
	stripOptionalNulls(inst, schema, root)
}

func resolveRef(ref string, root map[string]any) (map[string]any, bool) {
	const prefix = "#/$defs/"
	if !strings.HasPrefix(ref, prefix) {
		return nil, false
	}
	defs, _ := root["$defs"].(map[string]any)
	target, ok := defs[strings.TrimPrefix(ref, prefix)].(map[string]any)
	return target, ok
}

// callRecord is one finished (or failed) Meta call.
type callRecord struct {
	resp      provider.Response
	deltas    []string
	meanDelay time.Duration
	err       error
	valid     bool
	validErr  string
}

// safeCall makes one call and never fails the test: an error is
// recorded and logged, and the run continues.
func safeCall(t *testing.T, m *provider.Meta, req provider.Request) callRecord {
	t.Helper()
	var rec callRecord
	var last time.Time
	var totalGap time.Duration
	resp, err := m.Respond(context.Background(), req, func(d provider.Delta) {
		now := time.Now()
		if !last.IsZero() {
			totalGap += now.Sub(last)
		}
		last = now
		rec.deltas = append(rec.deltas, d.Text)
	})
	if err != nil {
		rec.err = err
		t.Logf("call %s: ERROR %v", req.TurnID, err)
		return rec
	}
	rec.resp = resp
	if len(rec.deltas) > 1 {
		rec.meanDelay = totalGap / time.Duration(len(rec.deltas)-1)
	}
	t.Logf("call %s: ok, %d deltas, in %d / out %d tokens, cost %.6f USD",
		req.TurnID, len(rec.deltas), resp.Usage.InputTokens, resp.Usage.OutputTokens, resp.Usage.CostUSD)
	return rec
}

func errDetail(err error) string {
	if perr, ok := err.(*provider.ProviderError); ok {
		return fmt.Sprintf("código `%s`, tipo `%s`, param `%s`", perr.Code, perr.Type, perr.Param)
	}
	return fmt.Sprintf("%T", err)
}

// keywordProbes: one minimal strict-shaped schema per keyword of the
// principal schema (defaults note for micro-test 0.8). A probe is
// admitted when its strict call succeeds.
var keywordProbes = []struct {
	keyword string
	schema  string
	answer  string
}{
	{"if/then", `{"type":"object","additionalProperties":false,"properties":{"operation":{"type":"string","enum":["replace","insert_after"]},"anchor":{"type":"string"}},"required":["operation","anchor"],"if":{"properties":{"operation":{"const":"replace"}}},"then":{"properties":{"anchor":{"minLength":1}}}}`, `{"operation":"insert_after","anchor":""}`},
	{"minLength", `{"type":"object","additionalProperties":false,"properties":{"text":{"type":"string","minLength":1}},"required":["text"]}`, `{"text":"x"}`},
	{"pattern", `{"type":"object","additionalProperties":false,"properties":{"label":{"type":"string","pattern":"^[a-z]+-[0-9]+$"}},"required":["label"]}`, `{"label":"dec-1"}`},
	{"anyOf", `{"type":"object","additionalProperties":false,"properties":{"supersedes":{"anyOf":[{"type":"string"},{"type":"null"}]}},"required":["supersedes"]}`, `{"supersedes":null}`},
	{"const", `{"type":"object","additionalProperties":false,"properties":{"operation":{"const":"replace"}},"required":["operation"]}`, `{"operation":"replace"}`},
	{"propiedades opcionales", `{"type":"object","additionalProperties":false,"properties":{"a":{"type":"string"},"b":{"type":"string"}},"required":["a"]}`, `{"a":"x"}`},
}

// dropKeys returns a copy of the schema (decoded) without the named
// keys at any level, for the full-schema variants.
func dropKeys(node any, keys map[string]bool) any {
	switch n := node.(type) {
	case map[string]any:
		out := make(map[string]any, len(n))
		for k, v := range n {
			if keys[k] {
				continue
			}
			out[k] = dropKeys(v, keys)
		}
		return out
	case []any:
		out := make([]any, len(n))
		for i, item := range n {
			out[i] = dropKeys(item, keys)
		}
		return out
	default:
		return node
	}
}

func marshalSchema(t *testing.T, doc any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal variant schema: %v", err)
	}
	return b
}

func saveFixture(t *testing.T, name string, rec callRecord) {
	t.Helper()
	fx := provider.Fixture{Name: name, Deltas: rec.deltas, Items: rec.resp.RawItems,
		Usage: rec.resp.Usage, DeltaDelay: rec.meanDelay}
	b, err := json.MarshalIndent(fx, "", "  ")
	if err != nil {
		t.Logf("fixture %s: marshal: %v", name, err)
		return
	}
	path := filepath.Join(testdataDir, name+".json")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Logf("fixture %s: write: %v", name, err)
		return
	}
	t.Logf("fixture saved: %s", path)
}

// TestSchemaCompilesWithECMARegex is the regression check for the
// first run: the label pattern needs ECMA-262. No key, no calls.
func TestSchemaCompilesWithECMARegex(t *testing.T) {
	raw, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read principal schema: %v", err)
	}
	sch := compileSchema(t, raw)
	good := `{"triage_line":"x","closing_risk":"none","note":"n","used_decisions":["dec-1"]}`
	inst, err := decodeInstance(good)
	if err != nil {
		t.Fatalf("decode good instance: %v", err)
	}
	if err := validateInstance(sch, inst); err != nil {
		t.Fatalf("a conforming output must validate: %v", err)
	}
	bad := `{"triage_line":"x","closing_risk":"bogus","note":"n"}`
	badInst, err := decodeInstance(bad)
	if err != nil {
		t.Fatalf("decode bad instance: %v", err)
	}
	if err := validateInstance(sch, badInst); err == nil {
		t.Fatal("a closing_risk outside the closed list must not validate")
	}

	// The transport schema must also compile, and an output with
	// nulls in the originally-optional properties must validate
	// against the full schema once those nulls are removed.
	transport, err := provider.TransportSchema(raw)
	if err != nil {
		t.Fatalf("TransportSchema: %v", err)
	}
	compileSchema(t, transport)
	var fullDoc map[string]any
	if err := json.Unmarshal(raw, &fullDoc); err != nil {
		t.Fatalf("decode full schema: %v", err)
	}
	withNulls := `{"triage_line":"x","closing_risk":"none","note":"n","used_decisions":null,"new_text":null}`
	inst2, err := decodeInstance(withNulls)
	if err != nil {
		t.Fatalf("decode nulls instance: %v", err)
	}
	if obj, ok := inst2.(map[string]any); ok {
		stripOptionalNulls(obj, fullDoc, fullDoc)
	}
	if err := validateInstance(sch, inst2); err != nil {
		t.Fatalf("output with optional nulls removed must validate: %v", err)
	}
	if obj, ok := inst2.(map[string]any); ok {
		if _, present := obj["used_decisions"]; present {
			t.Fatalf("optional null must be removed before validating: %v", obj)
		}
	}
}

const toyInstructions = "You are the principal of tupaqnin. For this toy step, return only the fixed output JSON: " +
	"a triage_line, closing_risk \"none\", one new_text with section_id \"new-1\", base_version 0, anchor \"\", " +
	"operation \"insert_after\" and a short text, proposed_roles for new-1 with role \"other\", and a note. " +
	"Fill every other field with null or an empty array."

const toyInput = "Toy request: write the first section of a short guide about watering plants."

func TestMicroTestPrincipalSchema(t *testing.T) {
	apiKey := os.Getenv("META_API_KEY")
	if apiKey == "" {
		t.Fatal("META_API_KEY is required: run `make microtest`, which reads it from the keychain")
	}
	meta := provider.NewMeta(apiKey, nil, nil)

	schemaRaw, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read principal schema: %v", err)
	}
	sch := compileSchema(t, schemaRaw)
	var fullDoc map[string]any
	if err := json.Unmarshal(schemaRaw, &fullDoc); err != nil {
		t.Fatalf("decode full schema: %v", err)
	}

	var report strings.Builder
	fmt.Fprintf(&report, "# Micro-prueba 0.8 · informe\n\n")
	fmt.Fprintf(&report, "Fecha: %s · modelo del principal: `%s`, esfuerzo `%s`.\n\n",
		time.Now().UTC().Format("2006-01-02"), provider.PrincipalModel, provider.PrincipalEffort)
	// The report is written even if everything below fails.
	defer func() {
		if err := os.MkdirAll(filepath.Dir(reportPath), 0o755); err != nil {
			t.Logf("report dir: %v", err)
			return
		}
		if err := os.WriteFile(reportPath, []byte(report.String()), 0o644); err != nil {
			t.Logf("write report: %v", err)
			return
		}
		t.Logf("report written: %s", reportPath)
	}()

	// 1. Keyword probes, each strict and on its own.
	report.WriteString("## Palabras clave del esquema en modo estricto\n\n")
	report.WriteString("| Palabra clave | ¿La admite? | Detalle |\n|---|---|---|\n")
	for _, probe := range keywordProbes {
		rec := safeCall(t, meta, provider.Request{
			ProjectID: "microtest", TurnID: "probe-" + probe.keyword,
			Role: provider.RolePrincipal, Model: provider.PrincipalModel, Effort: provider.EffortLow,
			Instructions: "Return exactly this JSON and nothing else: " + probe.answer,
			Input:        []json.RawMessage{userItem("Return the JSON you were given.")},
			Schema:       json.RawMessage(probe.schema), SchemaStrict: true,
		})
		if rec.err == nil {
			fmt.Fprintf(&report, "| `%s` | sí | la llamada terminó bien |\n", probe.keyword)
			continue
		}
		verdict := "sin conclusión"
		if perr, ok := rec.err.(*provider.ProviderError); ok &&
			(strings.Contains(perr.Param, "schema") || strings.Contains(perr.Param, "text") || strings.Contains(perr.Param, "format")) {
			verdict = "no"
		}
		fmt.Fprintf(&report, "| `%s` | %s | error: %s |\n", probe.keyword, verdict, errDetail(rec.err))
	}
	report.WriteString("\nLas que no se admitan se omiten del esquema enviado y la app las valida en Go (B-2).\n\n")

	// 2. The full schema, in variants, each on its own (not strict:
	// these test whether Meta accepts the keyword at all).
	report.WriteString("## El esquema completo, por variantes (sin strict)\n\n")
	report.WriteString("| Variante | ¿La acepta? | Detalle |\n|---|---|---|\n")
	variants := []struct {
		name string
		keys map[string]bool
	}{
		{"tal cual", nil},
		{"sin $schema ni $id", map[string]bool{"$schema": true, "$id": true}},
		{"sin if/then", map[string]bool{"if": true, "then": true, "else": true}},
		{"sin pattern", map[string]bool{"pattern": true}},
		{"sin $schema/$id, sin if/then y sin pattern", map[string]bool{
			"$schema": true, "$id": true, "if": true, "then": true, "else": true, "pattern": true}},
	}
	for _, variant := range variants {
		sent := schemaRaw
		if variant.keys != nil {
			var doc any
			if err := json.Unmarshal(schemaRaw, &doc); err != nil {
				t.Fatalf("decode schema for variant: %v", err)
			}
			sent = marshalSchema(t, dropKeys(doc, variant.keys))
		}
		rec := safeCall(t, meta, provider.Request{
			ProjectID: "microtest", TurnID: "variant-" + variant.name,
			Role: provider.RolePrincipal, Model: provider.PrincipalModel, Effort: provider.EffortLow,
			Instructions: toyInstructions,
			Input:        []json.RawMessage{userItem(toyInput)},
			Schema:       sent, // SchemaStrict false: accepted as-is
		})
		if rec.err == nil {
			fmt.Fprintf(&report, "| %s | sí | la llamada terminó bien |\n", variant.name)
		} else {
			fmt.Fprintf(&report, "| %s | no | error: %s |\n", variant.name, errDetail(rec.err))
		}
	}
	report.WriteString("\n")

	// 2b. The full schema as-is under strict, on its own. Expected to
	// be rejected (the subset requires every property in required);
	// it is a finding, not a failure.
	report.WriteString("## El esquema completo tal cual, en modo estricto\n\n")
	strictRec := safeCall(t, meta, provider.Request{
		ProjectID: "microtest", TurnID: "full-schema-strict",
		Role: provider.RolePrincipal, Model: provider.PrincipalModel, Effort: provider.EffortLow,
		Instructions: toyInstructions,
		Input:        []json.RawMessage{userItem(toyInput)},
		Schema:       schemaRaw, SchemaStrict: true,
	})
	if strictRec.err == nil {
		report.WriteString("El esquema completo **sí** fue admitido en modo estricto.\n\n")
	} else {
		fmt.Fprintf(&report, "Rechazado en modo estricto: %s. El esquema de transporte (`provider.TransportSchema`) "+
			"pone cada propiedad en `required`, vuelve anulables las opcionales y quita las palabras clave no admitidas; "+
			"la app valida en Go contra el esquema completo (B-2).\n\n", errDetail(strictRec.err))
	}

	// 3. Ten calls with the transport schema, strict, each validated
	// in Go against the full schema after removing the nulls of the
	// properties that were optional.
	transport, err := provider.TransportSchema(schemaRaw)
	if err != nil {
		t.Fatalf("TransportSchema: %v", err)
	}
	report.WriteString("## Diez llamadas de Spark con el esquema de transporte (strict)\n\n")
	report.WriteString("| # | Resultado | ¿Cumple el esquema completo en Go? | Tokens entrada | Tokens salida | Costo USD |\n")
	report.WriteString("|---|---|---|---|---|---|\n")
	var records []callRecord
	validCount := 0
	var totalCost float64
	for i := 1; i <= 10; i++ {
		rec := safeCall(t, meta, provider.Request{
			ProjectID: "microtest", TurnID: fmt.Sprintf("toy-%d", i),
			Role: provider.RolePrincipal, Model: provider.PrincipalModel, Effort: provider.PrincipalEffort,
			Instructions: toyInstructions,
			Input:        []json.RawMessage{userItem(toyInput)},
			Schema:       transport, SchemaStrict: true,
		})
		if rec.err != nil {
			fmt.Fprintf(&report, "| %d | error: %s | — | — | — | — |\n", i, errDetail(rec.err))
			records = append(records, rec)
			continue
		}
		inst, derr := decodeInstance(rec.resp.Text)
		if derr != nil {
			rec.validErr = "la salida no es JSON: " + derr.Error()
		} else {
			if obj, ok := inst.(map[string]any); ok {
				stripOptionalNulls(obj, fullDoc, fullDoc)
			}
			if verr := validateInstance(sch, inst); verr != nil {
				rec.validErr = verr.Error()
			} else {
				rec.valid = true
				validCount++
			}
		}
		records = append(records, rec)
		totalCost += rec.resp.Usage.CostUSD
		fmt.Fprintf(&report, "| %d | bien | %s | %d | %d | %.6f |\n", i,
			map[bool]string{true: "sí", false: "no: " + rec.validErr}[rec.valid],
			rec.resp.Usage.InputTokens, rec.resp.Usage.OutputTokens, rec.resp.Usage.CostUSD)
	}
	fmt.Fprintf(&report, "\nCumplen: %d de 10 · costo total de las que terminaron: %.6f USD.\n\n", validCount, totalCost)

	// 4. Recordings: one valid, one invalid, one with streaming.
	report.WriteString("## Grabaciones guardadas (internal/provider/testdata/)\n\n")
	savedValid := false
	bestStreaming := -1
	for i, rec := range records {
		if rec.err != nil {
			continue
		}
		if rec.valid && !savedValid {
			saveFixture(t, "microtest-valid", rec)
			savedValid = true
		}
		if bestStreaming < 0 || len(rec.deltas) > len(records[bestStreaming].deltas) {
			bestStreaming = i
		}
	}
	fmt.Fprintf(&report, "- `microtest-valid.json`: %s\n",
		map[bool]string{true: "la primera respuesta que cumplió", false: "ninguna de las diez cumplió; no se guardó"}[savedValid])
	if bestStreaming >= 0 {
		saveFixture(t, "microtest-streaming", records[bestStreaming])
		report.WriteString("- `microtest-streaming.json`: la llamada con más deltas, con su ritmo medio entre deltas.\n")
	} else {
		report.WriteString("- `microtest-streaming.json`: ninguna llamada terminó; no se guardó.\n")
	}
	invalid := safeCall(t, meta, provider.Request{
		ProjectID: "microtest", TurnID: "invalid-plain-text",
		Role: provider.RolePrincipal, Model: provider.PrincipalModel, Effort: provider.EffortLow,
		Instructions: "Reply with one short plain sentence. Do not return JSON.",
		Input:        []json.RawMessage{userItem("Say hello to a new reader.")},
	})
	if invalid.err != nil {
		fmt.Fprintf(&report, "- `microtest-invalid.json`: la llamada en texto plano falló (%s); no se guardó.\n", errDetail(invalid.err))
	} else {
		inst, derr := decodeInstance(invalid.resp.Text)
		invalid.valid = derr == nil && validateInstance(sch, inst) == nil
		saveFixture(t, "microtest-invalid", invalid)
		fmt.Fprintf(&report, "- `microtest-invalid.json`: respuesta en texto plano; cumple el esquema: %s.\n",
			map[bool]string{true: "sí — no sirve como inválida", false: "no"}[invalid.valid])
	}
	report.WriteString("\n")

	// 5. Context windows (Meta's model docs, read for 0.5).
	report.WriteString("## Ventana de contexto por modelo\n\n")
	report.WriteString("| Modelo | Ventana (tokens) | Fuente |\n|---|---|---|\n")
	for _, model := range []string{provider.ModelSpark13, provider.ModelWonder10} {
		fmt.Fprintf(&report, "| `%s` | %d | https://dev.meta.ai/docs/models |\n", model, provider.ContextWindow(model))
	}
	report.WriteString("\nLa ventana alimenta el tope del arranque (≤ 15 %) y el umbral (B2-2, B2-3).\n")
	t.Logf("micro-test finished: %d/10 valid with the transport schema", validCount)
}
