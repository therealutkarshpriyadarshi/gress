package schema

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"go.uber.org/zap"
)

// jsonSchemaCodec implements the Codec interface for JSON Schema
type jsonSchemaCodec struct {
	registry RegistryClient
	logger   *zap.Logger

	// Cache compiled schemas
	schemas map[int]*jsonschema.Schema
}

// NewJSONSchemaCodec creates a new JSON Schema codec
func NewJSONSchemaCodec(registry RegistryClient, logger *zap.Logger) Codec {
	return &jsonSchemaCodec{
		registry: registry,
		logger:   logger,
		schemas:  make(map[int]*jsonschema.Schema),
	}
}

// Encode serializes data according to a JSON Schema
func (c *jsonSchemaCodec) Encode(data interface{}, schemaMetadata *SchemaMetadata) ([]byte, error) {
	if schemaMetadata.SchemaType != SchemaTypeJSON {
		return nil, fmt.Errorf("expected JSON schema, got %s", schemaMetadata.SchemaType)
	}

	// Get or compile schema
	schema, err := c.getSchema(schemaMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema: %w", err)
	}

	// Convert data to interface{} for validation
	var dataInterface interface{}
	switch v := data.(type) {
	case []byte:
		if err := json.Unmarshal(v, &dataInterface); err != nil {
			return nil, fmt.Errorf("failed to unmarshal data: %w", err)
		}
	case string:
		if err := json.Unmarshal([]byte(v), &dataInterface); err != nil {
			return nil, fmt.Errorf("failed to unmarshal data: %w", err)
		}
	case map[string]interface{}:
		dataInterface = v
	default:
		// Try to convert via JSON
		jsonBytes, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal data: %w", err)
		}
		if err := json.Unmarshal(jsonBytes, &dataInterface); err != nil {
			return nil, fmt.Errorf("failed to unmarshal data: %w", err)
		}
	}

	// Validate against schema
	if err := schema.Validate(dataInterface); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(dataInterface)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal to JSON: %w", err)
	}

	// Create Confluent wire format: magic byte (0x0) + schema ID (4 bytes) + JSON data
	buf := new(bytes.Buffer)

	// Write magic byte
	if err := buf.WriteByte(magicByte); err != nil {
		return nil, fmt.Errorf("failed to write magic byte: %w", err)
	}

	// Write schema ID (big-endian)
	if err := binary.Write(buf, binary.BigEndian, int32(schemaMetadata.ID)); err != nil {
		return nil, fmt.Errorf("failed to write schema ID: %w", err)
	}

	// Write JSON data
	if _, err := buf.Write(jsonBytes); err != nil {
		return nil, fmt.Errorf("failed to write JSON data: %w", err)
	}

	return buf.Bytes(), nil
}

// Decode deserializes data according to a JSON Schema
func (c *jsonSchemaCodec) Decode(data []byte, schemaMetadata *SchemaMetadata) (interface{}, error) {
	if len(data) < 5 {
		return nil, errors.New("data too short for Confluent wire format")
	}

	// Check magic byte
	if data[0] != magicByte {
		return nil, fmt.Errorf("invalid magic byte: expected 0x0, got 0x%x", data[0])
	}

	// Extract schema ID
	schemaID := int(binary.BigEndian.Uint32(data[1:5]))

	// Get schema from registry if not provided
	var metadata *SchemaMetadata
	var err error

	if schemaMetadata == nil || schemaMetadata.ID != schemaID {
		metadata, err = c.registry.GetSchema(nil, schemaID)
		if err != nil {
			return nil, fmt.Errorf("failed to get schema %d: %w", schemaID, err)
		}
	} else {
		metadata = schemaMetadata
	}

	if metadata.SchemaType != SchemaTypeJSON {
		return nil, fmt.Errorf("expected JSON schema, got %s", metadata.SchemaType)
	}

	// Get or compile schema
	schema, err := c.getSchema(metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema: %w", err)
	}

	// Unmarshal JSON data (skip magic byte and schema ID)
	var result interface{}
	if err := json.Unmarshal(data[5:], &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Validate against schema
	if err := schema.Validate(result); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return result, nil
}

// GetSchemaType returns the schema type this codec handles
func (c *jsonSchemaCodec) GetSchemaType() SchemaType {
	return SchemaTypeJSON
}

// getSchema retrieves or compiles a schema for the given metadata
func (c *jsonSchemaCodec) getSchema(metadata *SchemaMetadata) (*jsonschema.Schema, error) {
	// Check cache
	if schema, exists := c.schemas[metadata.ID]; exists {
		return schema, nil
	}

	// Compile schema
	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft2020

	// Add schema to compiler
	schemaURL := fmt.Sprintf("schema://%d", metadata.ID)
	if err := compiler.AddResource(schemaURL, strings.NewReader(metadata.Schema)); err != nil {
		return nil, fmt.Errorf("failed to add schema resource: %w", err)
	}

	schema, err := compiler.Compile(schemaURL)
	if err != nil {
		return nil, fmt.Errorf("failed to compile schema: %w", err)
	}

	// Cache it
	c.schemas[metadata.ID] = schema

	c.logger.Debug("Compiled JSON Schema",
		zap.Int("schema_id", metadata.ID),
		zap.String("subject", metadata.Subject),
	)

	return schema, nil
}

// ValidateJSONSchema validates a JSON Schema
func ValidateJSONSchema(schemaStr string) error {
	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft2020

	if err := compiler.AddResource("schema://validate", strings.NewReader(schemaStr)); err != nil {
		return fmt.Errorf("failed to add schema resource: %w", err)
	}

	_, err := compiler.Compile("schema://validate")
	if err != nil {
		return fmt.Errorf("invalid JSON Schema: %w", err)
	}

	return nil
}

// ExtractJSONSchemaProperties extracts property names from a JSON Schema
func ExtractJSONSchemaProperties(schemaStr string) ([]string, error) {
	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(schemaStr), &schema); err != nil {
		return nil, fmt.Errorf("failed to parse JSON Schema: %w", err)
	}

	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		return nil, errors.New("schema does not contain properties")
	}

	var propNames []string
	for name := range properties {
		propNames = append(propNames, name)
	}

	return propNames, nil
}

// GetRequiredFields extracts required field names from a JSON Schema
func GetRequiredFields(schemaStr string) ([]string, error) {
	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(schemaStr), &schema); err != nil {
		return nil, fmt.Errorf("failed to parse JSON Schema: %w", err)
	}

	required, ok := schema["required"].([]interface{})
	if !ok {
		return []string{}, nil
	}

	var requiredFields []string
	for _, field := range required {
		if fieldName, ok := field.(string); ok {
			requiredFields = append(requiredFields, fieldName)
		}
	}

	return requiredFields, nil
}

// ConvertValidationError converts a jsonschema validation error to ValidationError
func ConvertValidationError(err error) []*ValidationError {
	var validationErrors []*ValidationError

	if ve, ok := err.(*jsonschema.ValidationError); ok {
		validationErrors = append(validationErrors, &ValidationError{
			Field:   ve.InstanceLocation,
			Message: ve.Message,
		})

		// Add causes (nested errors)
		for _, cause := range ve.Causes {
			validationErrors = append(validationErrors, ConvertValidationError(cause)...)
		}
	}

	return validationErrors
}
