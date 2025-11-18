package schema

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

// protobufCodec implements the Codec interface for Protocol Buffers schemas
type protobufCodec struct {
	registry RegistryClient
	logger   *zap.Logger

	// Cache message descriptors
	descriptors map[int]protoreflect.MessageDescriptor
}

// NewProtobufCodec creates a new Protocol Buffers codec
func NewProtobufCodec(registry RegistryClient, logger *zap.Logger) Codec {
	return &protobufCodec{
		registry:    registry,
		logger:      logger,
		descriptors: make(map[int]protoreflect.MessageDescriptor),
	}
}

// Encode serializes data according to a Protobuf schema
func (c *protobufCodec) Encode(data interface{}, schemaMetadata *SchemaMetadata) ([]byte, error) {
	if schemaMetadata.SchemaType != SchemaTypeProtobuf {
		return nil, fmt.Errorf("expected PROTOBUF schema, got %s", schemaMetadata.SchemaType)
	}

	// Get or create descriptor
	descriptor, err := c.getDescriptor(schemaMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to get descriptor: %w", err)
	}

	// Create dynamic message
	msg := dynamicpb.NewMessage(descriptor)

	// Convert data to protobuf message
	if err := c.populateMessage(msg, data); err != nil {
		return nil, fmt.Errorf("failed to populate message: %w", err)
	}

	// Marshal to binary
	protoBytes, err := proto.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal protobuf: %w", err)
	}

	// Create Confluent wire format: magic byte (0x0) + schema ID (4 bytes) + protobuf data
	buf := new(bytes.Buffer)

	// Write magic byte
	if err := buf.WriteByte(magicByte); err != nil {
		return nil, fmt.Errorf("failed to write magic byte: %w", err)
	}

	// Write schema ID (big-endian)
	if err := binary.Write(buf, binary.BigEndian, int32(schemaMetadata.ID)); err != nil {
		return nil, fmt.Errorf("failed to write schema ID: %w", err)
	}

	// Write protobuf data
	if _, err := buf.Write(protoBytes); err != nil {
		return nil, fmt.Errorf("failed to write protobuf data: %w", err)
	}

	return buf.Bytes(), nil
}

// Decode deserializes data according to a Protobuf schema
func (c *protobufCodec) Decode(data []byte, schemaMetadata *SchemaMetadata) (interface{}, error) {
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

	if metadata.SchemaType != SchemaTypeProtobuf {
		return nil, fmt.Errorf("expected PROTOBUF schema, got %s", metadata.SchemaType)
	}

	// Get or create descriptor
	descriptor, err := c.getDescriptor(metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to get descriptor: %w", err)
	}

	// Create dynamic message
	msg := dynamicpb.NewMessage(descriptor)

	// Unmarshal protobuf data (skip magic byte and schema ID)
	if err := proto.Unmarshal(data[5:], msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal protobuf: %w", err)
	}

	// Convert to map for easier consumption
	result, err := c.messageToMap(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to convert message to map: %w", err)
	}

	return result, nil
}

// GetSchemaType returns the schema type this codec handles
func (c *protobufCodec) GetSchemaType() SchemaType {
	return SchemaTypeProtobuf
}

// getDescriptor retrieves or creates a message descriptor for the given schema
func (c *protobufCodec) getDescriptor(metadata *SchemaMetadata) (protoreflect.MessageDescriptor, error) {
	// Check cache
	if descriptor, exists := c.descriptors[metadata.ID]; exists {
		return descriptor, nil
	}

	// Parse the FileDescriptorSet from the schema
	var fds descriptorpb.FileDescriptorSet
	if err := proto.Unmarshal([]byte(metadata.Schema), &fds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal FileDescriptorSet: %w", err)
	}

	// Create file descriptor
	files, err := protodesc.NewFiles(&fds)
	if err != nil {
		return nil, fmt.Errorf("failed to create file descriptor: %w", err)
	}

	// Get the first file (assuming single file in schema)
	var fileDesc protoreflect.FileDescriptor
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		fileDesc = fd
		return false // stop after first file
	})

	if fileDesc == nil {
		return nil, errors.New("no file descriptor found in schema")
	}

	// Get the first message type
	messageTypes := fileDesc.Messages()
	if messageTypes.Len() == 0 {
		return nil, errors.New("no message types found in schema")
	}

	descriptor := messageTypes.Get(0)

	// Cache it
	c.descriptors[metadata.ID] = descriptor

	c.logger.Debug("Created Protobuf descriptor",
		zap.Int("schema_id", metadata.ID),
		zap.String("subject", metadata.Subject),
		zap.String("message_name", string(descriptor.Name())),
	)

	return descriptor, nil
}

// populateMessage populates a protobuf message from a data object
func (c *protobufCodec) populateMessage(msg *dynamicpb.Message, data interface{}) error {
	// Convert data to JSON first
	var jsonData []byte
	var err error

	switch v := data.(type) {
	case []byte:
		jsonData = v
	case string:
		jsonData = []byte(v)
	case map[string]interface{}:
		// Already in map format, convert to JSON
		jsonData, err = protojson.Marshal(msg)
		if err != nil {
			return fmt.Errorf("failed to marshal map to JSON: %w", err)
		}
	default:
		// Try to use protojson for other types
		if protoMsg, ok := data.(proto.Message); ok {
			jsonData, err = protojson.Marshal(protoMsg)
			if err != nil {
				return fmt.Errorf("failed to marshal proto message to JSON: %w", err)
			}
		} else {
			return fmt.Errorf("unsupported data type: %T", data)
		}
	}

	// Unmarshal JSON into the message
	if err := protojson.Unmarshal(jsonData, msg); err != nil {
		return fmt.Errorf("failed to unmarshal JSON into message: %w", err)
	}

	return nil
}

// messageToMap converts a protobuf message to a map
func (c *protobufCodec) messageToMap(msg *dynamicpb.Message) (map[string]interface{}, error) {
	// Marshal to JSON
	jsonData, err := protojson.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message to JSON: %w", err)
	}

	// Unmarshal to map
	var result map[string]interface{}
	if err := protojson.Unmarshal(jsonData, msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON to map: %w", err)
	}

	return result, nil
}

// ValidateProtobufSchema validates a Protocol Buffers schema
func ValidateProtobufSchema(schemaBytes []byte) error {
	var fds descriptorpb.FileDescriptorSet
	if err := proto.Unmarshal(schemaBytes, &fds); err != nil {
		return fmt.Errorf("invalid Protobuf schema: %w", err)
	}

	_, err := protodesc.NewFiles(&fds)
	if err != nil {
		return fmt.Errorf("failed to create file descriptor: %w", err)
	}

	return nil
}

// GetProtobufMessageName extracts the message name from a schema
func GetProtobufMessageName(schemaBytes []byte) (string, error) {
	var fds descriptorpb.FileDescriptorSet
	if err := proto.Unmarshal(schemaBytes, &fds); err != nil {
		return "", fmt.Errorf("failed to unmarshal FileDescriptorSet: %w", err)
	}

	files, err := protodesc.NewFiles(&fds)
	if err != nil {
		return "", fmt.Errorf("failed to create file descriptor: %w", err)
	}

	var messageName string
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		messageTypes := fd.Messages()
		if messageTypes.Len() > 0 {
			messageName = string(messageTypes.Get(0).FullName())
			return false
		}
		return true
	})

	if messageName == "" {
		return "", errors.New("no message types found in schema")
	}

	return messageName, nil
}
