package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateJSONSchema(t *testing.T) {
	tests := []struct {
		name    string
		schema  string
		wantErr bool
	}{
		{
			name: "valid simple schema",
			schema: `{
				"type": "object",
				"properties": {
					"name": {"type": "string"},
					"age": {"type": "integer"}
				},
				"required": ["name"]
			}`,
			wantErr: false,
		},
		{
			name: "valid complex schema with nested objects",
			schema: `{
				"type": "object",
				"properties": {
					"id": {"type": "string"},
					"profile": {
						"type": "object",
						"properties": {
							"email": {"type": "string", "format": "email"},
							"age": {"type": "integer", "minimum": 0}
						}
					}
				}
			}`,
			wantErr: false,
		},
		{
			name:    "invalid schema - malformed JSON",
			schema:  `{invalid`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateJSONSchema(tt.schema)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExtractJSONSchemaProperties(t *testing.T) {
	schema := `{
		"type": "object",
		"properties": {
			"id": {"type": "string"},
			"name": {"type": "string"},
			"email": {"type": "string", "format": "email"}
		}
	}`

	props, err := ExtractJSONSchemaProperties(schema)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"id", "name", "email"}, props)
}

func TestGetRequiredFields(t *testing.T) {
	tests := []struct {
		name     string
		schema   string
		expected []string
	}{
		{
			name: "with required fields",
			schema: `{
				"type": "object",
				"properties": {
					"id": {"type": "string"},
					"name": {"type": "string"}
				},
				"required": ["id", "name"]
			}`,
			expected: []string{"id", "name"},
		},
		{
			name: "without required fields",
			schema: `{
				"type": "object",
				"properties": {
					"id": {"type": "string"}
				}
			}`,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			required, err := GetRequiredFields(tt.schema)
			assert.NoError(t, err)
			assert.ElementsMatch(t, tt.expected, required)
		})
	}
}
