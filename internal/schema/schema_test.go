package schema

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type SimpleInput struct {
	FilePath string `json:"file_path" jsonschema:"required,description=The absolute path to the file"`
	Content  string `json:"content" jsonschema:"required,description=The content to write"`
}

type InputWithOptional struct {
	Pattern string `json:"pattern" jsonschema:"required,description=The glob pattern"`
	Path    string `json:"path,omitempty" jsonschema:"description=The directory to search in"`
}

type InputWithPointer struct {
	FilePath string `json:"file_path" jsonschema:"required"`
	Offset   *int   `json:"offset,omitempty" jsonschema:"description=Line offset to start reading from"`
	Limit    *int   `json:"limit,omitempty" jsonschema:"description=Number of lines to read"`
}

type InputWithBool struct {
	FilePath   string `json:"file_path" jsonschema:"required"`
	OldString  string `json:"old_string" jsonschema:"required"`
	NewString  string `json:"new_string" jsonschema:"required"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
}

func TestGenerateSimple(t *testing.T) {
	schema := Generate[SimpleInput]()

	props, ok := schema.Properties.(map[string]any)
	require.True(t, ok, "Properties should be map[string]any")

	// Check file_path property
	fp, ok := props["file_path"].(map[string]any)
	require.True(t, ok, "file_path should exist")
	assert.Equal(t, "string", fp["type"])
	assert.Equal(t, "The absolute path to the file", fp["description"])

	// Check content property
	ct, ok := props["content"].(map[string]any)
	require.True(t, ok, "content should exist")
	assert.Equal(t, "string", ct["type"])

	// Check required fields
	assert.Contains(t, schema.Required, "file_path")
	assert.Contains(t, schema.Required, "content")
}

func TestGenerateOptionalFields(t *testing.T) {
	schema := Generate[InputWithOptional]()

	// pattern is required, path is not
	assert.Contains(t, schema.Required, "pattern")
	assert.NotContains(t, schema.Required, "path")

	props, ok := schema.Properties.(map[string]any)
	require.True(t, ok)

	path, ok := props["path"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "The directory to search in", path["description"])
}

func TestGeneratePointerFields(t *testing.T) {
	schema := Generate[InputWithPointer]()

	assert.Contains(t, schema.Required, "file_path")

	props, ok := schema.Properties.(map[string]any)
	require.True(t, ok)

	// Pointer fields should be present
	_, hasOffset := props["offset"]
	assert.True(t, hasOffset, "offset should be in properties")

	_, hasLimit := props["limit"]
	assert.True(t, hasLimit, "limit should be in properties")
}

func TestGenerateBoolField(t *testing.T) {
	schema := Generate[InputWithBool]()

	props, ok := schema.Properties.(map[string]any)
	require.True(t, ok)

	ra, ok := props["replace_all"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "boolean", ra["type"])
}

func TestGenerateJSONRoundtrip(t *testing.T) {
	schema := Generate[SimpleInput]()

	data, err := json.Marshal(schema)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))

	// Should have "type": "object" and "properties"
	assert.Equal(t, "object", m["type"])
	assert.NotNil(t, m["properties"])
	assert.NotNil(t, m["required"])
}
