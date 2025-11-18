package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestValidateAvroSchema(t *testing.T) {
	tests := []struct {
		name    string
		schema  string
		wantErr bool
	}{
		{
			name: "valid simple record",
			schema: `{
				"type": "record",
				"name": "User",
				"fields": [
					{"name": "id", "type": "string"},
					{"name": "age", "type": "int"}
				]
			}`,
			wantErr: false,
		},
		{
			name: "valid complex record with unions",
			schema: `{
				"type": "record",
				"name": "Event",
				"fields": [
					{"name": "id", "type": "string"},
					{"name": "timestamp", "type": "long"},
					{"name": "data", "type": ["null", "string"], "default": null}
				]
			}`,
			wantErr: false,
		},
		{
			name:    "invalid schema - malformed JSON",
			schema:  `{invalid json`,
			wantErr: true,
		},
		{
			name: "invalid schema - missing required field",
			schema: `{
				"type": "record",
				"fields": [
					{"name": "id", "type": "string"}
				]
			}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAvroSchema(tt.schema)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetAvroSchemaFields(t *testing.T) {
	schema := `{
		"type": "record",
		"name": "User",
		"fields": [
			{"name": "id", "type": "string"},
			{"name": "name", "type": "string"},
			{"name": "age", "type": "int"}
		]
	}`

	fields, err := GetAvroSchemaFields(schema)
	assert.NoError(t, err)
	assert.Equal(t, []string{"id", "name", "age"}, fields)
}

func TestAvroCodecEncodeDecode(t *testing.T) {
	logger := zap.NewNop()

	// Mock registry for testing
	mockRegistry := &mockRegistryClient{
		schemas: make(map[int]*SchemaMetadata),
	}

	codec := NewAvroCodec(mockRegistry, logger)

	schema := `{
		"type": "record",
		"name": "TestRecord",
		"fields": [
			{"name": "id", "type": "string"},
			{"name": "count", "type": "int"}
		]
	}`

	metadata := &SchemaMetadata{
		ID:         1,
		Version:    1,
		Schema:     schema,
		Subject:    "test-subject",
		SchemaType: SchemaTypeAvro,
	}

	mockRegistry.schemas[1] = metadata

	// Test data
	data := map[string]interface{}{
		"id":    "test-123",
		"count": 42,
	}

	// Encode
	encoded, err := codec.Encode(data, metadata)
	assert.NoError(t, err)
	assert.NotNil(t, encoded)

	// Verify wire format (magic byte + schema ID + data)
	assert.Equal(t, byte(0x0), encoded[0]) // Magic byte

	// Decode
	decoded, err := codec.Decode(encoded, metadata)
	assert.NoError(t, err)
	assert.NotNil(t, decoded)

	// Verify decoded data
	decodedMap, ok := decoded.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "test-123", decodedMap["id"])
	assert.Equal(t, int32(42), decodedMap["count"])
}

// mockRegistryClient is a simple mock for testing
type mockRegistryClient struct {
	schemas map[int]*SchemaMetadata
}

func (m *mockRegistryClient) GetSchema(ctx interface{}, schemaID int) (*SchemaMetadata, error) {
	if schema, ok := m.schemas[schemaID]; ok {
		return schema, nil
	}
	return nil, assert.AnError
}

func (m *mockRegistryClient) GetLatestSchema(ctx interface{}, subject string) (*SchemaMetadata, error) {
	return nil, assert.AnError
}

func (m *mockRegistryClient) GetSchemaByVersion(ctx interface{}, subject string, version int) (*SchemaMetadata, error) {
	return nil, assert.AnError
}

func (m *mockRegistryClient) RegisterSchema(ctx interface{}, subject string, schema string, schemaType SchemaType, references []Reference) (*SchemaMetadata, error) {
	return nil, assert.AnError
}

func (m *mockRegistryClient) DeleteSubject(ctx interface{}, subject string, permanent bool) error {
	return assert.AnError
}

func (m *mockRegistryClient) DeleteSchemaVersion(ctx interface{}, subject string, version int, permanent bool) error {
	return assert.AnError
}

func (m *mockRegistryClient) GetCompatibility(ctx interface{}, subject string) (CompatibilityMode, error) {
	return "", assert.AnError
}

func (m *mockRegistryClient) SetCompatibility(ctx interface{}, subject string, mode CompatibilityMode) error {
	return assert.AnError
}

func (m *mockRegistryClient) TestCompatibility(ctx interface{}, subject string, schema string, schemaType SchemaType, version int) (bool, error) {
	return false, assert.AnError
}

func (m *mockRegistryClient) GetSubjects(ctx interface{}) ([]string, error) {
	return nil, assert.AnError
}

func (m *mockRegistryClient) GetVersions(ctx interface{}, subject string) ([]int, error) {
	return nil, assert.AnError
}

func (m *mockRegistryClient) Close() error {
	return nil
}
