package schema

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/linkedin/goavro/v2"
	"go.uber.org/zap"
)

const (
	// Magic byte prefix for Confluent wire format
	magicByte byte = 0x0
)

// avroCodec implements the Codec interface for Avro schemas
type avroCodec struct {
	registry RegistryClient
	logger   *zap.Logger

	// Cache compiled codecs
	codecs map[int]*goavro.Codec
}

// NewAvroCodec creates a new Avro codec
func NewAvroCodec(registry RegistryClient, logger *zap.Logger) Codec {
	return &avroCodec{
		registry: registry,
		logger:   logger,
		codecs:   make(map[int]*goavro.Codec),
	}
}

// Encode serializes data according to an Avro schema
func (c *avroCodec) Encode(data interface{}, schemaMetadata *SchemaMetadata) ([]byte, error) {
	if schemaMetadata.SchemaType != SchemaTypeAvro {
		return nil, fmt.Errorf("expected AVRO schema, got %s", schemaMetadata.SchemaType)
	}

	// Get or create codec
	codec, err := c.getCodec(schemaMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to get codec: %w", err)
	}

	// Convert data to native Go types if needed
	nativeData, err := c.toNative(data)
	if err != nil {
		return nil, fmt.Errorf("failed to convert data to native: %w", err)
	}

	// Encode using Avro
	avroBytes, err := codec.BinaryFromNative(nil, nativeData)
	if err != nil {
		return nil, fmt.Errorf("failed to encode Avro: %w", err)
	}

	// Create Confluent wire format: magic byte (0x0) + schema ID (4 bytes) + Avro data
	buf := new(bytes.Buffer)

	// Write magic byte
	if err := buf.WriteByte(magicByte); err != nil {
		return nil, fmt.Errorf("failed to write magic byte: %w", err)
	}

	// Write schema ID (big-endian)
	if err := binary.Write(buf, binary.BigEndian, int32(schemaMetadata.ID)); err != nil {
		return nil, fmt.Errorf("failed to write schema ID: %w", err)
	}

	// Write Avro data
	if _, err := buf.Write(avroBytes); err != nil {
		return nil, fmt.Errorf("failed to write Avro data: %w", err)
	}

	return buf.Bytes(), nil
}

// Decode deserializes data according to an Avro schema
func (c *avroCodec) Decode(data []byte, schemaMetadata *SchemaMetadata) (interface{}, error) {
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

	if metadata.SchemaType != SchemaTypeAvro {
		return nil, fmt.Errorf("expected AVRO schema, got %s", metadata.SchemaType)
	}

	// Get or create codec
	codec, err := c.getCodec(metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to get codec: %w", err)
	}

	// Decode Avro data (skip magic byte and schema ID)
	native, _, err := codec.NativeFromBinary(data[5:])
	if err != nil {
		return nil, fmt.Errorf("failed to decode Avro: %w", err)
	}

	return native, nil
}

// GetSchemaType returns the schema type this codec handles
func (c *avroCodec) GetSchemaType() SchemaType {
	return SchemaTypeAvro
}

// getCodec retrieves or creates a codec for the given schema
func (c *avroCodec) getCodec(metadata *SchemaMetadata) (*goavro.Codec, error) {
	// Check cache
	if codec, exists := c.codecs[metadata.ID]; exists {
		return codec, nil
	}

	// Create new codec
	codec, err := goavro.NewCodec(metadata.Schema)
	if err != nil {
		return nil, fmt.Errorf("failed to create Avro codec: %w", err)
	}

	// Cache it
	c.codecs[metadata.ID] = codec

	c.logger.Debug("Created Avro codec",
		zap.Int("schema_id", metadata.ID),
		zap.String("subject", metadata.Subject),
	)

	return codec, nil
}

// toNative converts various data types to native Go types for Avro
func (c *avroCodec) toNative(data interface{}) (interface{}, error) {
	// If already a map, return as-is
	if m, ok := data.(map[string]interface{}); ok {
		return m, nil
	}

	// If it's a struct, convert to map via JSON
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data to JSON: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON to map: %w", err)
	}

	return result, nil
}

// EncodeKey encodes a key value (typically simpler than full message encoding)
func (c *avroCodec) EncodeKey(key interface{}, schemaMetadata *SchemaMetadata) ([]byte, error) {
	return c.Encode(key, schemaMetadata)
}

// DecodeKey decodes a key value
func (c *avroCodec) DecodeKey(data []byte, schemaMetadata *SchemaMetadata) (interface{}, error) {
	return c.Decode(data, schemaMetadata)
}

// ValidateSchema validates an Avro schema
func ValidateAvroSchema(schemaStr string) error {
	_, err := goavro.NewCodec(schemaStr)
	if err != nil {
		return fmt.Errorf("invalid Avro schema: %w", err)
	}
	return nil
}

// AvroSchemaToJSON converts an Avro schema to a JSON representation
func AvroSchemaToJSON(schemaStr string) (map[string]interface{}, error) {
	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(schemaStr), &schema); err != nil {
		return nil, fmt.Errorf("failed to parse Avro schema: %w", err)
	}
	return schema, nil
}

// GetAvroSchemaFields extracts field names from an Avro schema
func GetAvroSchemaFields(schemaStr string) ([]string, error) {
	schema, err := AvroSchemaToJSON(schemaStr)
	if err != nil {
		return nil, err
	}

	fields, ok := schema["fields"].([]interface{})
	if !ok {
		return nil, errors.New("schema does not contain fields")
	}

	var fieldNames []string
	for _, field := range fields {
		if fieldMap, ok := field.(map[string]interface{}); ok {
			if name, ok := fieldMap["name"].(string); ok {
				fieldNames = append(fieldNames, name)
			}
		}
	}

	return fieldNames, nil
}
