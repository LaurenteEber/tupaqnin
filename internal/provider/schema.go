package provider

import (
	"encoding/json"
	"fmt"
	"sort"
)

// droppedForTransport are the only keywords the sent schema drops,
// per the findings of micro-test 0.8 (2026-10-28): strict mode
// rejects if/then and optional properties, and admits minLength,
// pattern, anyOf and const, so those stay in the sent schema. What
// the transport cannot express (the if/then conditions) is validated
// in Go against the full schema instead (B-2). The full schema in
// docs/ is never changed; this builds the transport copy.
var droppedForTransport = map[string]bool{
	"$schema": true, "$id": true,
	"if": true, "then": true, "else": true,
}

// TransportSchema builds the schema actually sent to Meta from the
// full principal schema (backlog 0.8, defaults note). It drops the
// keywords above and rewrites every object for strict mode
// (structured-output docs): additionalProperties is false everywhere
// and required lists every property; a property that was optional in
// the full schema becomes nullable (anyOf with null) so the model can
// still omit its value. Before validating an answer in Go against
// the full schema, the nulls of originally-optional properties are
// removed again (micro-test does this; the step gate will too).
func TransportSchema(full json.RawMessage) (json.RawMessage, error) {
	var doc any
	if err := json.Unmarshal(full, &doc); err != nil {
		return nil, fmt.Errorf("transport schema: parse full schema: %w", err)
	}
	out, err := json.Marshal(transportNode(doc))
	if err != nil {
		return nil, fmt.Errorf("transport schema: marshal: %w", err)
	}
	return out, nil
}

func transportNode(node any) any {
	switch n := node.(type) {
	case map[string]any:
		return transportObject(n)
	case []any:
		out := make([]any, len(n))
		for i, item := range n {
			out[i] = transportNode(item)
		}
		return out
	default:
		return node
	}
}

func transportObject(obj map[string]any) map[string]any {
	out := make(map[string]any, len(obj))
	props, hasProps := obj["properties"].(map[string]any)
	required := requiredSet(obj)

	for key, value := range obj {
		if droppedForTransport[key] {
			continue
		}
		switch key {
		case "properties":
			newProps := make(map[string]any, len(props))
			for name, propSchema := range props {
				prop := transportNode(propSchema)
				if !required[name] {
					prop = nullable(prop)
				}
				newProps[name] = prop
			}
			out[key] = newProps
		case "required":
			// Rebuilt below from the property names.
		case "additionalProperties":
			// Forced to false below when the object has properties.
		default:
			out[key] = transportNode(value)
		}
	}

	if hasProps {
		names := make([]string, 0, len(props))
		for name := range props {
			names = append(names, name)
		}
		sort.Strings(names)
		req := make([]any, len(names))
		for i, name := range names {
			req[i] = name
		}
		out["required"] = req
		out["additionalProperties"] = false
	}
	return out
}

func requiredSet(obj map[string]any) map[string]bool {
	set := make(map[string]bool)
	if req, ok := obj["required"].([]any); ok {
		for _, item := range req {
			if name, ok := item.(string); ok {
				set[name] = true
			}
		}
	}
	return set
}

// nullable wraps a schema so null is also accepted (anyOf, which the
// strict subset admits below the root).
func nullable(schema any) any {
	if obj, ok := schema.(map[string]any); ok {
		if branches, ok := obj["anyOf"].([]any); ok {
			for _, branch := range branches {
				if b, ok := branch.(map[string]any); ok && b["type"] == "null" {
					return schema
				}
			}
			obj["anyOf"] = append(branches, map[string]any{"type": "null"})
			return obj
		}
	}
	return map[string]any{"anyOf": []any{schema, map[string]any{"type": "null"}}}
}
