package schema

import (
	"encoding/json"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/invopop/jsonschema"
)

// Generate produces an anthropic.ToolInputSchemaParam from a Go struct type T.
// It uses struct tags (json, jsonschema) to derive the JSON Schema.
func Generate[T any]() anthropic.ToolInputSchemaParam {
	var zero T
	s := jsonschema.Reflect(&zero)

	// The top-level schema wraps the actual type; extract properties and required
	// from the root definition.
	root := extractRoot(s)

	properties := schemaProperties(root)

	return anthropic.ToolInputSchemaParam{
		Properties: properties,
		Required:   root.Required,
	}
}

// extractRoot resolves the root schema, following $ref to $defs if needed.
func extractRoot(s *jsonschema.Schema) *jsonschema.Schema {
	if s.Ref != "" && s.Definitions != nil {
		// invopop/jsonschema puts the actual type under $defs with a ref like
		// "#/$defs/TypeName". Extract the type name from the ref.
		for _, def := range s.Definitions {
			if def.Type == "object" {
				return def
			}
		}
	}
	return s
}

// schemaProperties converts an ordered map of properties into a plain
// map[string]any suitable for the Anthropic API.
func schemaProperties(s *jsonschema.Schema) map[string]any {
	if s.Properties == nil {
		return nil
	}
	props := make(map[string]any)
	for pair := s.Properties.Oldest(); pair != nil; pair = pair.Next() {
		props[pair.Key] = propertySchema(pair.Value)
	}
	return props
}

// propertySchema converts a single property schema to a serializable map.
func propertySchema(s *jsonschema.Schema) map[string]any {
	m := make(map[string]any)

	if s.Type != "" {
		m["type"] = s.Type
	}
	if s.Description != "" {
		m["description"] = s.Description
	}
	if s.Default != nil {
		m["default"] = s.Default
	}
	if len(s.Enum) > 0 {
		m["enum"] = s.Enum
	}

	// Handle pointer types: invopop/jsonschema uses anyOf for nullable types
	if len(s.AnyOf) > 0 {
		for _, sub := range s.AnyOf {
			if sub.Type != "null" && sub.Type != "" {
				m["type"] = sub.Type
				break
			}
		}
	}

	// Nested object properties
	if s.Properties != nil {
		m["type"] = "object"
		m["properties"] = schemaProperties(s)
		if len(s.Required) > 0 {
			m["required"] = s.Required
		}
	}

	// Array items
	if s.Items != nil {
		m["items"] = propertySchema(s.Items)
	}

	return m
}

// GenerateJSON is a convenience that returns the schema as raw JSON bytes.
func GenerateJSON[T any]() (json.RawMessage, error) {
	param := Generate[T]()
	return json.Marshal(param)
}
