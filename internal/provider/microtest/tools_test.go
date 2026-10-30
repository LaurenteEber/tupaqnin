//go:build meta

// Micro-test of tool calls (backlog-r1 1.9, proposal of 30-oct R-3):
// whether Meta's Responses API admits function tools in strict mode
// together with the principal's strict fixed output, whether Spark
// calls them to read the human's sources before citing, whether the
// round trip with store: false (reasoning items and
// function_call_output sent back) works, and what it costs. If it
// fails, B-22 applies instead of the tools.
//
// It runs only with -tags=meta and META_API_KEY set:
//
//	make microtest-tools
//
// No Meta call fails this test: every result is logged and noted in
// the report, which is written even on failure. The report lands in
// docs/casos-prueba/micro-prueba-herramientas/informe.md and the calls
// of the first complete run in internal/provider/testdata/tools/.
package microtest

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/provider"
)

const (
	toolsReportPath = "../../../docs/casos-prueba/micro-prueba-herramientas/informe.md"
	toolsFixtureDir = "../testdata/tools"
	toolsRuns       = 5
	// maxToolRounds caps the calls of one run: the first call and up to
	// three answers with tool outputs.
	maxToolRounds = 4
)

// The two tools of R-3 over the human's material. Their argument
// shape here is the micro-test's: piece 1.9 fixes the real one.
var toolDefs = []map[string]any{
	{
		"type": "function", "name": "read_source", "strict": true,
		"description": "Read one of the human's sources. span is a paragraph range like \"3-5\", or null for the whole source.",
		"parameters": map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []string{"source", "span"},
			"properties": map[string]any{
				"source": map[string]any{"type": "string", "description": "The source label, for example src-1."},
				"span":   map[string]any{"type": []string{"string", "null"}},
			},
		},
	},
	{
		"type": "function", "name": "search_sources", "strict": true,
		"description": "Search the literal text in all of the human's sources. Returns the matching paragraphs with their source and number.",
		"parameters": map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []string{"text"},
			"properties": map[string]any{
				"text": map[string]any{"type": "string"},
			},
		},
	},
}

// toolSources is the toy material: two sources, the figure only in the
// body of the first, never in its first line.
var toolSources = map[string][]string{
	"src-1": {
		"Muestreo de agua de grifo en 40 hogares de Lima (2025).",
		"Se tomaron dos muestras por hogar, en la mañana y en la noche.",
		"Las partículas se contaron con espectroscopía infrarroja.",
		"El agua embotellada tuvo 1,5 veces más partículas por litro que la de grifo.",
		"La diferencia se mantuvo en los dos horarios.",
	},
	"src-2": {
		"Revisión de métodos de conteo de microplásticos (2024).",
		"Los métodos ópticos subestiman las partículas menores de 20 µm.",
		"Ningún estudio revisado compara agua embotellada con agua de grifo.",
	},
}

const toolsInstructions = `You write one section of a document with a human.
The human's sources are listed in the index by their first line only.
You have two tools over them: read_source and search_sources.
Before writing a figure or a citation, read the passage that holds it with a tool;
a citation's passage must be copied literally from what you read.
When you have read enough, return the fixed JSON output you were given.`

func toolsTask() string {
	var b strings.Builder
	b.WriteString("Index of the human's sources:\n")
	for _, label := range []string{"src-1", "src-2"} {
		fmt.Fprintf(&b, "- %s · %d paragraphs · first line: %s\n", label, len(toolSources[label]), toolSources[label][0])
	}
	b.WriteString("\nSection sec-1 («Exposición») is empty. Write it: how much more particles bottled water has " +
		"than tap water, with the figure and its citation from the sources. Use the anchor \"\" with insert_after.")
	return b.String()
}

// runTool answers one function call from the toy material.
func runTool(name, arguments string) string {
	var args struct {
		Source string  `json:"source"`
		Span   *string `json:"span"`
		Text   string  `json:"text"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "error: arguments are not JSON"
	}
	switch name {
	case "read_source":
		paras, ok := toolSources[args.Source]
		if !ok {
			return "error: no source " + args.Source
		}
		from, to := 1, len(paras)
		if args.Span != nil {
			_, _ = fmt.Sscanf(*args.Span, "%d-%d", &from, &to)
		}
		var b strings.Builder
		for i := from; i <= to && i <= len(paras); i++ {
			if i >= 1 {
				fmt.Fprintf(&b, "[%d] %s\n", i, paras[i-1])
			}
		}
		return b.String()
	case "search_sources":
		var b strings.Builder
		for _, label := range []string{"src-1", "src-2"} {
			for i, p := range toolSources[label] {
				if args.Text != "" && strings.Contains(strings.ToLower(p), strings.ToLower(args.Text)) {
					fmt.Fprintf(&b, "%s [%d] %s\n", label, i+1, p)
				}
			}
		}
		if b.Len() == 0 {
			return "no matches"
		}
		return b.String()
	}
	return "error: unknown tool " + name
}

// toolCall is one finished call of the micro-test.
type toolCall struct {
	status    int
	errCode   string
	errParam  string
	output    []json.RawMessage
	text      string
	deltas    []string
	argDeltas int
	inTokens  int
	outTokens int
	elapsed   time.Duration
	meanDelay time.Duration
}

// callWithTools sends one streamed Responses call with tools; tools or
// format may be nil. It never fails the test.
func callWithTools(apiKey string, tools []map[string]any, format json.RawMessage, input []json.RawMessage) toolCall {
	body := map[string]any{
		"model": provider.PrincipalModel, "instructions": toolsInstructions,
		"stream": true, "store": false, "include": []string{"reasoning.encrypted_content"},
		"reasoning": map[string]string{"effort": provider.PrincipalEffort},
		"input":     input,
	}
	if tools != nil {
		body["tools"] = tools
		body["tool_choice"] = "auto"
	}
	if format != nil {
		body["text"] = map[string]any{"format": map[string]any{
			"type": "json_schema", "name": "principal_output", "strict": true, "schema": format}}
	}
	payload, _ := json.Marshal(body)
	var call toolCall
	start := time.Now()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		provider.DefaultBaseURL+"/responses", bytes.NewReader(payload))
	if err != nil {
		call.errCode = "build_request"
		return call
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
	if err != nil {
		call.errCode = "transport"
		return call
	}
	defer func() { _ = resp.Body.Close() }()
	call.status = resp.StatusCode
	if resp.StatusCode != http.StatusOK {
		// Only the error's code and param are kept, never the raw body (T-10).
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		var parsed struct {
			Error struct {
				Code  *string `json:"code"`
				Param *string `json:"param"`
			} `json:"error"`
		}
		_ = json.Unmarshal(raw, &parsed)
		call.errCode = fmt.Sprintf("http_%d", resp.StatusCode)
		if parsed.Error.Code != nil {
			call.errCode += " " + *parsed.Error.Code
		}
		if parsed.Error.Param != nil {
			call.errParam = *parsed.Error.Param
		}
		call.elapsed = time.Since(start)
		return call
	}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64<<10), 10<<20)
	var last time.Time
	var gap time.Duration
	var done []json.RawMessage
	for scanner.Scan() {
		data := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "data:"))
		if !strings.HasPrefix(scanner.Text(), "data:") || data == "" || data == "[DONE]" {
			continue
		}
		var ev struct {
			Type     string          `json:"type"`
			Delta    string          `json:"delta"`
			Item     json.RawMessage `json:"item"`
			Response *struct {
				Output []json.RawMessage `json:"output"`
				Usage  struct {
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
				Error *struct {
					Code string `json:"code"`
				} `json:"error"`
			} `json:"response"`
		}
		if json.Unmarshal([]byte(data), &ev) != nil {
			continue
		}
		switch ev.Type {
		case "response.output_text.delta":
			now := time.Now()
			if !last.IsZero() {
				gap += now.Sub(last)
			}
			last = now
			call.deltas = append(call.deltas, ev.Delta)
		case "response.function_call_arguments.delta":
			call.argDeltas++
		case "response.output_item.done":
			done = append(done, ev.Item)
		case "response.failed":
			call.errCode = "response_failed"
			if ev.Response != nil && ev.Response.Error != nil {
				call.errCode += " " + ev.Response.Error.Code
			}
		case "response.completed":
			if ev.Response != nil {
				call.output = ev.Response.Output
				call.inTokens = ev.Response.Usage.InputTokens
				call.outTokens = ev.Response.Usage.OutputTokens
			}
		}
	}
	if len(call.output) == 0 {
		call.output = done
	}
	if call.output == nil && call.errCode == "" {
		call.errCode = "stream_ended"
	}
	for _, item := range call.output {
		var msg struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		}
		if json.Unmarshal(item, &msg) == nil && msg.Type == "message" {
			for _, c := range msg.Content {
				if c.Type == "output_text" {
					call.text += c.Text
				}
			}
		}
	}
	if len(call.deltas) > 1 {
		call.meanDelay = gap / time.Duration(len(call.deltas)-1)
	}
	call.elapsed = time.Since(start)
	return call
}

func (c toolCall) failed() bool { return c.errCode != "" }

func (c toolCall) cost() float64 {
	return provider.CostUSD(provider.PrincipalModel, c.inTokens, c.outTokens)
}

func (c toolCall) errText() string {
	if c.errParam != "" {
		return fmt.Sprintf("`%s` (param `%s`)", c.errCode, c.errParam)
	}
	return fmt.Sprintf("`%s`", c.errCode)
}

// functionCall is one function_call item of an output.
type functionCall struct {
	Type      string `json:"type"`
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

func functionCalls(output []json.RawMessage) []functionCall {
	var out []functionCall
	for _, item := range output {
		var fc functionCall
		if json.Unmarshal(item, &fc) == nil && fc.Type == "function_call" {
			out = append(out, fc)
		}
	}
	return out
}

func toolOutputItem(callID, output string) json.RawMessage {
	b, _ := json.Marshal(map[string]string{"type": "function_call_output", "call_id": callID, "output": output})
	return b
}

// argsValid reports whether a call's arguments decode and carry only
// and exactly the tool's required properties.
func argsValid(fc functionCall) bool {
	var args map[string]any
	if json.Unmarshal([]byte(fc.Arguments), &args) != nil {
		return false
	}
	for _, def := range toolDefs {
		if def["name"] != fc.Name {
			continue
		}
		required := def["parameters"].(map[string]any)["required"].([]string)
		if len(args) != len(required) {
			return false
		}
		for _, name := range required {
			if _, ok := args[name]; !ok {
				return false
			}
		}
		return true
	}
	return false
}

// toolRun is one task carried until a final output or the cap.
type toolRun struct {
	calls      []toolCall
	toolCalls  []functionCall
	badArgs    int
	final      string
	finalValid bool
	finalErr   string
	figureRead bool // the figure 1,5 reached the model through a tool
	cited      bool // a citation's passage is literal in src-1
}

func saveToolFixture(t *testing.T, name string, c toolCall) {
	t.Helper()
	fx := provider.Fixture{Name: name, Deltas: c.deltas, Items: c.output,
		Usage:      provider.Usage{InputTokens: c.inTokens, OutputTokens: c.outTokens, CostUSD: c.cost()},
		DeltaDelay: c.meanDelay}
	b, err := json.MarshalIndent(fx, "", "  ")
	if err != nil {
		t.Logf("fixture %s: marshal: %v", name, err)
		return
	}
	if err := os.MkdirAll(toolsFixtureDir, 0o755); err != nil {
		t.Logf("fixture %s: mkdir: %v", name, err)
		return
	}
	if err := os.WriteFile(filepath.Join(toolsFixtureDir, name+".json"), b, 0o644); err != nil {
		t.Logf("fixture %s: write: %v", name, err)
	}
}

func TestMicroTestTools(t *testing.T) {
	apiKey := os.Getenv("META_API_KEY")
	if apiKey == "" {
		t.Skip("META_API_KEY not set")
	}

	var report strings.Builder
	defer func() {
		if err := os.MkdirAll(filepath.Dir(toolsReportPath), 0o755); err != nil {
			t.Errorf("report dir: %v", err)
			return
		}
		if err := os.WriteFile(toolsReportPath, []byte(report.String()), 0o644); err != nil {
			t.Errorf("write report: %v", err)
			return
		}
		t.Logf("report written: %s", toolsReportPath)
	}()

	fmt.Fprintf(&report, "# Micro-prueba de herramientas (pieza 1.9, R-3)\n\nCorrida: %s · modelo `%s` · esfuerzo `%s` · `store: false`, streaming.\n\n",
		time.Now().UTC().Format("2006-01-02 15:04 UTC"), provider.PrincipalModel, provider.PrincipalEffort)

	schemaRaw, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read principal schema: %v", err)
	}
	var fullDoc map[string]any
	if err := json.Unmarshal(schemaRaw, &fullDoc); err != nil {
		t.Fatalf("decode principal schema: %v", err)
	}
	sch := compileSchema(t, schemaRaw)
	transport, err := provider.TransportSchema(schemaRaw)
	if err != nil {
		t.Fatalf("TransportSchema: %v", err)
	}
	task := []json.RawMessage{userItem(toolsTask())}

	// 1. Is the request admitted? Tools alone, then tools with the
	// strict fixed output (what the principal would send).
	report.WriteString("## 1. ¿Meta admite la petición?\n\n| Prueba | Resultado | Llamadas a herramientas | Segundos |\n|---|---|---|---|\n")
	probes := []struct {
		name   string
		format json.RawMessage
	}{
		{"herramientas `strict`, sin salida fija", nil},
		{"herramientas `strict` + salida fija `strict` (esquema de transporte del principal)", transport},
	}
	admitted := true
	for _, p := range probes {
		c := callWithTools(apiKey, toolDefs, p.format, task)
		result := "admitida"
		if c.failed() {
			result = "rechazada: " + c.errText()
			if p.format != nil {
				admitted = false
			}
		}
		t.Logf("probe %q: %s, %d tool calls", p.name, result, len(functionCalls(c.output)))
		fmt.Fprintf(&report, "| %s | %s | %d | %.1f |\n", p.name, result, len(functionCalls(c.output)), c.elapsed.Seconds())
	}
	report.WriteString("\n")
	if !admitted {
		report.WriteString("**Resultado: falla.** Meta no admite herramientas junto con la salida fija estricta. " +
			"Según la 1.9, se aplica B-22 en lugar de las herramientas.\n")
		return
	}

	// 2. Complete runs: call, answer each tool call, until the fixed
	// output arrives or the cap.
	fmt.Fprintf(&report, "## 2. %d corridas completas (hasta %d llamadas cada una)\n\n", toolsRuns, maxToolRounds)
	report.WriteString("| # | Llamadas al modelo | Herramientas usadas | Argumentos inválidos | ¿Salida final? | ¿Cumple el esquema completo? | ¿Leyó la cifra con una herramienta? | ¿Cita literal de src-1? | Segundos | Costo USD |\n")
	report.WriteString("|---|---|---|---|---|---|---|---|---|---|\n")
	var runs []toolRun
	saved := false
	for i := 1; i <= toolsRuns; i++ {
		run := toolRun{}
		input := append([]json.RawMessage{}, task...)
		for round := 1; round <= maxToolRounds; round++ {
			c := callWithTools(apiKey, toolDefs, transport, input)
			run.calls = append(run.calls, c)
			if c.failed() {
				run.finalErr = "error " + c.errText()
				break
			}
			calls := functionCalls(c.output)
			if len(calls) == 0 {
				run.final = c.text
				break
			}
			// store: false: send back everything the model returned
			// (reasoning with its encrypted content and the calls),
			// then one output per call.
			input = append(input, c.output...)
			for _, fc := range calls {
				run.toolCalls = append(run.toolCalls, fc)
				if !argsValid(fc) {
					run.badArgs++
				}
				out := runTool(fc.Name, fc.Arguments)
				if strings.Contains(out, "1,5 veces") {
					run.figureRead = true
				}
				input = append(input, toolOutputItem(fc.CallID, out))
			}
		}
		if run.final != "" {
			inst, derr := decodeInstance(run.final)
			switch {
			case derr != nil:
				run.finalErr = "no es JSON"
			default:
				if obj, ok := inst.(map[string]any); ok {
					stripOptionalNulls(obj, fullDoc, fullDoc)
				}
				if verr := validateInstance(sch, inst); verr != nil {
					run.finalErr = verr.Error()
				} else {
					run.finalValid = true
				}
			}
			var out struct {
				Citations []struct {
					Passage string `json:"passage"`
				} `json:"citations"`
			}
			if json.Unmarshal([]byte(run.final), &out) == nil {
				body := strings.Join(toolSources["src-1"], "\n")
				for _, c := range out.Citations {
					if c.Passage != "" && strings.Contains(body, c.Passage) {
						run.cited = true
					}
				}
			}
		}
		runs = append(runs, run)

		var secs, cost float64
		for _, c := range run.calls {
			secs += c.elapsed.Seconds()
			cost += c.cost()
		}
		names := map[string]int{}
		for _, fc := range run.toolCalls {
			names[fc.Name]++
		}
		used := "ninguna"
		if len(names) > 0 {
			var parts []string
			for _, n := range []string{"read_source", "search_sources"} {
				if names[n] > 0 {
					parts = append(parts, fmt.Sprintf("%s ×%d", n, names[n]))
				}
			}
			used = strings.Join(parts, ", ")
		}
		finalCell := map[bool]string{true: "sí", false: "no"}[run.final != ""]
		if run.final == "" && run.finalErr != "" {
			finalCell = "no: " + run.finalErr
		}
		validCell := "—"
		if run.final != "" {
			validCell = map[bool]string{true: "sí", false: "no: " + run.finalErr}[run.finalValid]
		}
		fmt.Fprintf(&report, "| %d | %d | %s | %d | %s | %s | %s | %s | %.1f | %.6f |\n", i, len(run.calls), used, run.badArgs,
			finalCell, validCell, map[bool]string{true: "sí", false: "no"}[run.figureRead],
			map[bool]string{true: "sí", false: "no"}[run.cited], secs, cost)
		t.Logf("run %d: %d calls, %d tool calls, final=%v valid=%v figureRead=%v cited=%v",
			i, len(run.calls), len(run.toolCalls), run.final != "", run.finalValid, run.figureRead, run.cited)

		// The first run that used a tool and ended valid is recorded,
		// one fixture per call, for the fake provider (1.9).
		if !saved && len(run.toolCalls) > 0 && run.finalValid {
			for n, c := range run.calls {
				saveToolFixture(t, fmt.Sprintf("tools-call-%d", n+1), c)
			}
			saved = true
		}
	}

	// 3. Verdict.
	var withTools, valid, read, cited, streamed int
	for _, r := range runs {
		if len(r.toolCalls) > 0 {
			withTools++
		}
		if r.finalValid {
			valid++
		}
		if r.figureRead {
			read++
		}
		if r.cited {
			cited++
		}
		for _, c := range r.calls {
			if c.argDeltas > 0 {
				streamed++
				break
			}
		}
	}
	report.WriteString("\n## 3. Resultado\n\n")
	fmt.Fprintf(&report, "- Corridas que usaron herramientas: %d de %d.\n", withTools, len(runs))
	fmt.Fprintf(&report, "- Corridas con salida final que cumple el esquema completo: %d de %d.\n", valid, len(runs))
	fmt.Fprintf(&report, "- Corridas que leyeron la cifra con una herramienta: %d de %d · con la cita literal de src-1: %d de %d.\n",
		read, len(runs), cited, len(runs))
	fmt.Fprintf(&report, "- Corridas con argumentos de herramienta en streaming (`response.function_call_arguments.delta`): %d de %d.\n", streamed, len(runs))
	fmt.Fprintf(&report, "- Grabaciones en `internal/provider/testdata/tools/`: %s.\n\n",
		map[bool]string{true: "las llamadas de la primera corrida válida con herramientas", false: "ninguna (no hubo corrida válida con herramientas)"}[saved])
	pass := withTools >= 4 && valid >= 4
	if pass {
		report.WriteString("**Resultado: pasa** (≥ 4 de 5 corridas usan herramientas y terminan con una salida válida). La 1.9 construye `read_source` y `search_sources`.\n")
	} else {
		report.WriteString("**Resultado: falla** (se pedían ≥ 4 de 5 corridas con herramientas y salida válida). Según la 1.9, se aplica B-22 en lugar de las herramientas.\n")
	}
	t.Logf("micro-test tools: pass=%v (%d/%d with tools, %d/%d valid)", pass, withTools, len(runs), valid, len(runs))
}
