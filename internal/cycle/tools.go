package cycle

import (
	"context"
	"encoding/json"

	"github.com/LaurenteEber/tupaqnin/internal/provider"
)

// The principal's two tools over the person's material (R-3, bloque-2
// §3, glossary §11), in strict mode: the tools micro-test of 1.9
// passed with this shape (5 of 5 runs). The arguments' names are the
// app's: the design names only read_source(fuente, tramo) and
// search_sources(texto).
var principalTools = json.RawMessage(`[
 {"type":"function","name":"read_source","strict":true,
  "description":"Read one of the human's sources, literally, by paragraphs. span is a paragraph range like \"3-5\" or \"4\", or null for the whole source.",
  "parameters":{"type":"object","additionalProperties":false,"required":["source","span"],
   "properties":{"source":{"type":"string","description":"The source label from the index, for example src-1."},
    "span":{"type":["string","null"]}}}},
 {"type":"function","name":"search_sources","strict":true,
  "description":"Search a literal text in all of the human's sources. Returns each paragraph that holds it, with its source label and paragraph number.",
  "parameters":{"type":"object","additionalProperties":false,"required":["text"],
   "properties":{"text":{"type":"string"}}}}
]`)

// maxToolRounds caps the calls that answer tool calls within one model
// call of a step or the understanding. [EL DISEÑO NO LO DICE]: the
// design names no limit; the micro-test needed 2 calls per run. In the
// last round the calls are answered with ToolLimitMessage instead of
// being run, so no call is left without its output.
const maxToolRounds = 6

// ToolLimitMessage answers the tool calls of the last round.
const ToolLimitMessage = "Tool limit reached for this step: return your fixed output now with what you have read."

// runTool answers one tool call from the person's material, recording
// each read (via = read). An unknown tool or arguments that do not
// decode answer the model with what went wrong.
func (r *Runner) runTool(ctx context.Context, projectID, instanceID string, call provider.ToolCall) (string, error) {
	var args struct {
		Source string  `json:"source"`
		Span   *string `json:"span"`
		Text   string  `json:"text"`
	}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		return "The arguments are not valid JSON.", nil
	}
	switch call.Name {
	case "read_source":
		span := ""
		if args.Span != nil {
			span = *args.Span
		}
		return r.memory.ReadSource(ctx, projectID, instanceID, args.Source, span)
	case "search_sources":
		return r.memory.SearchSources(ctx, projectID, instanceID, args.Text)
	}
	return "There is no tool " + call.Name + ".", nil
}

// toolSet is what a model call may use (backlog-r3 3.2: respond takes
// the list of tools and its cap as parameters): the principal caps the
// rounds of answers (maxToolRounds), a reader caps its tool calls
// (MaxReaderToolCalls), every call counted.
type toolSet struct {
	defs json.RawMessage
	run  func(ctx context.Context, call provider.ToolCall) (string, error)
	// maxRounds caps the rounds of answers; in the last one the calls
	// are answered with ToolLimitMessage instead of being run.
	maxRounds int
	// maxCalls caps the tool calls, counted in *calls across the
	// model calls (a retry included); a call beyond it is answered with
	// ReaderToolCapMessage and never run. maxModelCalls caps the model
	// calls of one respond.
	maxCalls      int
	calls         *int
	maxModelCalls int
	// beforeTool runs before each tool is run (the round's cap, B-65);
	// an error stops the call.
	beforeTool func() error
}

// ReaderToolCapMessage answers a reader's tool call beyond its cap.
const ReaderToolCapMessage = "Tool cap reached (8 calls): this call was not run. Return your fixed output now with what you have read."

// principalToolSet is the principal's tools over the person's material
// (R-3), at most maxToolRounds rounds.
func (r *Runner) principalToolSet(projectID, instanceID string) toolSet {
	return toolSet{defs: principalTools, maxRounds: maxToolRounds,
		run: func(ctx context.Context, call provider.ToolCall) (string, error) {
			return r.runTool(ctx, projectID, instanceID, call)
		}}
}

// respond makes one principal call with its tools (R-3).
func (r *Runner) respond(ctx context.Context, req provider.Request, onDelta func(provider.Delta)) (provider.Response, []json.RawMessage, error) {
	return respondWith(ctx, r.prov, req, onDelta, r.principalToolSet(req.ProjectID, req.InstanceID))
}

// respondWith makes one model call and answers its tool calls until the
// model returns its output, within the set's caps. It returns the last
// response and the input that produced it (with every tool call and its
// output), so a retry continues the same conversation.
func respondWith(ctx context.Context, prov provider.Provider, req provider.Request, onDelta func(provider.Delta), ts toolSet) (provider.Response, []json.RawMessage, error) {
	req.Tools = ts.defs
	resp, err := prov.Respond(ctx, req, onDelta)
	for round := 0; err == nil && len(resp.ToolCalls) > 0; round++ {
		if ts.maxRounds > 0 && round >= ts.maxRounds {
			break
		}
		if ts.maxModelCalls > 0 && round+1 >= ts.maxModelCalls {
			break
		}
		input := append(append([]json.RawMessage{}, req.Input...), resp.RawItems...)
		for _, call := range resp.ToolCalls {
			out := ToolLimitMessage
			run := ts.maxRounds > 0 && round < ts.maxRounds-1
			if ts.maxCalls > 0 {
				*ts.calls++
				out, run = ReaderToolCapMessage, *ts.calls <= ts.maxCalls
			}
			if run {
				if ts.beforeTool != nil {
					if berr := ts.beforeTool(); berr != nil {
						return provider.Response{}, input, berr
					}
				}
				var terr error
				if out, terr = ts.run(ctx, call); terr != nil {
					return provider.Response{}, nil, terr
				}
			}
			input = append(input, provider.ToolOutputItem(call.CallID, out))
		}
		req.Input = input
		resp, err = prov.Respond(ctx, req, onDelta)
	}
	return resp, req.Input, err
}
