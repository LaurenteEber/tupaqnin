package provider_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/provider"
)

const smallFullSchema = `{
	"$schema": "https://json-schema.org/draft/2020-12/schema",
	"$id": "https://example.test/out.json",
	"type": "object",
	"additionalProperties": false,
	"required": ["operation"],
	"properties": {
		"operation": {"type": "string", "enum": ["replace", "insert_after"]},
		"anchor": {"type": "string", "minLength": 1, "pattern": "^[a-z]+$"},
		"tags": {"type": "array", "minItems": 1, "items": {
			"type": "object",
			"required": ["id"],
			"properties": {
				"id": {"$ref": "#/$defs/label"},
				"note": {"type": "string"}
			}
		}}
	},
	"if": {"properties": {"operation": {"const": "replace"}}},
	"then": {"properties": {"anchor": {"minLength": 1}}},
	"$defs": {"label": {"type": "string", "pattern": "^(?!new-)[a-z]+-[0-9]+$"}}
}`

func decode(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return doc
}

func TestB2_TransportSchemaDropsMetaAndRejectedKeywords(t *testing.T) {
	out, err := provider.TransportSchema(json.RawMessage(smallFullSchema))
	if err != nil {
		t.Fatalf("TransportSchema: %v", err)
	}
	for _, dropped := range []string{`"$schema"`, `"$id"`, `"if"`, `"then"`} {
		if strings.Contains(string(out), dropped) {
			t.Fatalf("transport schema still contains %s:\n%s", dropped, out)
		}
	}
	// Structure and the keywords strict admits (micro-test 0.8) stay:
	// type, enum, items, $ref, pattern, minLength, minItems.
	for _, kept := range []string{`"enum"`, `"$ref"`, `"items"`, `"type"`, `"pattern"`, `"minLength"`, `"minItems"`} {
		if !strings.Contains(string(out), kept) {
			t.Fatalf("transport schema lost %s:\n%s", kept, out)
		}
	}
}

func TestB2_TransportSchemaMakesEveryPropertyRequiredAndOptionalsNullable(t *testing.T) {
	out, err := provider.TransportSchema(json.RawMessage(smallFullSchema))
	if err != nil {
		t.Fatalf("TransportSchema: %v", err)
	}
	doc := decode(t, out)

	req, ok := doc["required"].([]any)
	if !ok || len(req) != 3 {
		t.Fatalf("root required: got %v, want all 3 properties", doc["required"])
	}
	props := doc["properties"].(map[string]any)

	// operation was required: not wrapped.
	if _, wrapped := props["operation"].(map[string]any)["anyOf"]; wrapped {
		t.Fatalf("required property must not become nullable: %v", props["operation"])
	}
	// anchor was optional: anyOf with a null branch.
	anchor := props["anchor"].(map[string]any)
	branches, ok := anchor["anyOf"].([]any)
	if !ok || len(branches) != 2 {
		t.Fatalf("optional property: got %v, want anyOf with null", anchor)
	}
	if branches[1].(map[string]any)["type"] != "null" {
		t.Fatalf("optional property second branch: got %v, want null", branches[1])
	}
	if doc["additionalProperties"] != false {
		t.Fatalf("root additionalProperties: got %v, want false", doc["additionalProperties"])
	}

	// Nested object in items gets the same treatment. tags itself was
	// optional at the root, so it arrives wrapped in anyOf: unwrap it.
	tagsWrapped := props["tags"].(map[string]any)
	tags := tagsWrapped["anyOf"].([]any)[0].(map[string]any)
	itemProps := tags["items"].(map[string]any)["properties"].(map[string]any)
	note := itemProps["note"].(map[string]any)
	if _, ok := note["anyOf"]; !ok {
		t.Fatalf("nested optional property must become nullable: %v", note)
	}
	itemReq := tags["items"].(map[string]any)["required"].([]any)
	if len(itemReq) != 2 {
		t.Fatalf("nested required: got %v, want id and note", itemReq)
	}
}
