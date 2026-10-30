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

// respond makes one model call and answers its tool calls until the
// model returns its output, up to maxToolRounds more calls. It returns
// the last response and the input that produced it (with every tool
// call and its output), so a retry continues the same conversation.
func (r *Runner) respond(ctx context.Context, req provider.Request, onDelta func(provider.Delta)) (provider.Response, []json.RawMessage, error) {
	req.Tools = principalTools
	resp, err := r.prov.Respond(ctx, req, onDelta)
	for round := 0; err == nil && len(resp.ToolCalls) > 0 && round < maxToolRounds; round++ {
		input := append(append([]json.RawMessage{}, req.Input...), resp.RawItems...)
		for _, call := range resp.ToolCalls {
			out := ToolLimitMessage
			if round < maxToolRounds-1 {
				var terr error
				if out, terr = r.runTool(ctx, req.ProjectID, req.InstanceID, call); terr != nil {
					return provider.Response{}, nil, terr
				}
			}
			input = append(input, provider.ToolOutputItem(call.CallID, out))
		}
		req.Input = input
		resp, err = r.prov.Respond(ctx, req, onDelta)
	}
	return resp, req.Input, err
}
