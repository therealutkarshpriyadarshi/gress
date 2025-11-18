package schema

import (
	"context"
	"time"
)

// SchemaType represents the format of a schema
type SchemaType string

const (
	SchemaTypeAvro     SchemaType = "AVRO"
	SchemaTypeProtobuf SchemaType = "PROTOBUF"
	SchemaTypeJSON     SchemaType = "JSON"
)

// CompatibilityMode defines how schema versions are compatible
type CompatibilityMode string

const (
	CompatibilityNone              CompatibilityMode = "NONE"
	CompatibilityBackward          CompatibilityMode = "BACKWARD"
	CompatibilityBackwardTransitive CompatibilityMode = "BACKWARD_TRANSITIVE"
	CompatibilityForward           CompatibilityMode = "FORWARD"
	CompatibilityForwardTransitive  CompatibilityMode = "FORWARD_TRANSITIVE"
	CompatibilityFull              CompatibilityMode = "FULL"
	CompatibilityFullTransitive    CompatibilityMode = "FULL_TRANSITIVE"
)

// SchemaMetadata contains metadata about a schema
type SchemaMetadata struct {
	ID            int           `json:"id"`
	Version       int           `json:"version"`
	Schema        string        `json:"schema"`
	Subject       string        `json:"subject"`
	SchemaType    SchemaType    `json:"schemaType"`
	References    []Reference   `json:"references,omitempty"`
	Metadata      *Metadata     `json:"metadata,omitempty"`
	RuleSet       *RuleSet      `json:"ruleSet,omitempty"`
}

// Reference represents a reference to another schema
type Reference struct {
	Name    string `json:"name"`
	Subject string `json:"subject"`
	Version int    `json:"version"`
}

// Metadata contains custom metadata for a schema
type Metadata struct {
	Tags       map[string]string `json:"tags,omitempty"`
	Properties map[string]string `json:"properties,omitempty"`
	Sensitive  []string          `json:"sensitive,omitempty"`
}

// RuleSet contains data governance rules
type RuleSet struct {
	MigrationRules []Rule `json:"migrationRules,omitempty"`
	DomainRules    []Rule `json:"domainRules,omitempty"`
}

// Rule represents a data governance rule
type Rule struct {
	Name     string            `json:"name"`
	Doc      string            `json:"doc,omitempty"`
	Kind     string            `json:"kind"`
	Mode     string            `json:"mode"`
	Type     string            `json:"type"`
	Tags     []string          `json:"tags,omitempty"`
	Params   map[string]string `json:"params,omitempty"`
	Expr     string            `json:"expr,omitempty"`
	OnSuccess string            `json:"onSuccess,omitempty"`
	OnFailure string            `json:"onFailure,omitempty"`
	Disabled  bool              `json:"disabled,omitempty"`
}

// RegistryClient defines the interface for interacting with a schema registry
type RegistryClient interface {
	// GetSchema retrieves a schema by ID
	GetSchema(ctx context.Context, schemaID int) (*SchemaMetadata, error)

	// GetLatestSchema retrieves the latest version of a schema for a subject
	GetLatestSchema(ctx context.Context, subject string) (*SchemaMetadata, error)

	// GetSchemaByVersion retrieves a specific version of a schema
	GetSchemaByVersion(ctx context.Context, subject string, version int) (*SchemaMetadata, error)

	// RegisterSchema registers a new schema or returns existing if identical
	RegisterSchema(ctx context.Context, subject string, schema string, schemaType SchemaType, references []Reference) (*SchemaMetadata, error)

	// DeleteSubject deletes a subject and all its versions
	DeleteSubject(ctx context.Context, subject string, permanent bool) error

	// DeleteSchemaVersion deletes a specific version
	DeleteSchemaVersion(ctx context.Context, subject string, version int, permanent bool) error

	// GetCompatibility gets the compatibility mode for a subject
	GetCompatibility(ctx context.Context, subject string) (CompatibilityMode, error)

	// SetCompatibility sets the compatibility mode for a subject
	SetCompatibility(ctx context.Context, subject string, mode CompatibilityMode) error

	// TestCompatibility tests if a schema is compatible
	TestCompatibility(ctx context.Context, subject string, schema string, schemaType SchemaType, version int) (bool, error)

	// GetSubjects lists all subjects
	GetSubjects(ctx context.Context) ([]string, error)

	// GetVersions lists all versions for a subject
	GetVersions(ctx context.Context, subject string) ([]int, error)

	// Close closes the registry client
	Close() error
}

// Codec defines the interface for encoding/decoding data with schemas
type Codec interface {
	// Encode serializes data according to a schema
	Encode(data interface{}, schemaMetadata *SchemaMetadata) ([]byte, error)

	// Decode deserializes data according to a schema
	Decode(data []byte, schemaMetadata *SchemaMetadata) (interface{}, error)

	// GetSchemaType returns the schema type this codec handles
	GetSchemaType() SchemaType
}

// Config contains configuration for schema registry integration
type Config struct {
	// RegistryURL is the URL of the schema registry
	RegistryURL string `json:"registryUrl" yaml:"registryUrl"`

	// Username for basic auth (optional)
	Username string `json:"username,omitempty" yaml:"username,omitempty"`

	// Password for basic auth (optional)
	Password string `json:"password,omitempty" yaml:"password,omitempty"`

	// Timeout for registry operations
	Timeout time.Duration `json:"timeout" yaml:"timeout"`

	// CacheEnabled enables local schema caching
	CacheEnabled bool `json:"cacheEnabled" yaml:"cacheEnabled"`

	// CacheSize is the maximum number of schemas to cache
	CacheSize int `json:"cacheSize" yaml:"cacheSize"`

	// CacheTTL is the time-to-live for cached schemas
	CacheTTL time.Duration `json:"cacheTTL" yaml:"cacheTTL"`

	// TLSEnabled enables TLS for registry connections
	TLSEnabled bool `json:"tlsEnabled" yaml:"tlsEnabled"`

	// TLSCertPath is the path to the TLS certificate
	TLSCertPath string `json:"tlsCertPath,omitempty" yaml:"tlsCertPath,omitempty"`

	// TLSKeyPath is the path to the TLS key
	TLSKeyPath string `json:"tlsKeyPath,omitempty" yaml:"tlsKeyPath,omitempty"`

	// TLSCAPath is the path to the CA certificate
	TLSCAPath string `json:"tlsCAPath,omitempty" yaml:"tlsCAPath,omitempty"`

	// TLSSkipVerify skips TLS certificate verification (not recommended)
	TLSSkipVerify bool `json:"tlsSkipVerify" yaml:"tlsSkipVerify"`
}

// SchemaConfig defines schema configuration for a topic
type SchemaConfig struct {
	// Subject is the schema registry subject name
	Subject string `json:"subject" yaml:"subject"`

	// SchemaType is the type of schema (AVRO, PROTOBUF, JSON)
	SchemaType SchemaType `json:"schemaType" yaml:"schemaType"`

	// Version is the specific version to use (0 for latest)
	Version int `json:"version" yaml:"version"`

	// AutoRegister automatically registers schemas if they don't exist
	AutoRegister bool `json:"autoRegister" yaml:"autoRegister"`

	// Compatibility is the compatibility mode for the subject
	Compatibility CompatibilityMode `json:"compatibility" yaml:"compatibility"`

	// ValidateOnRead validates data on deserialization
	ValidateOnRead bool `json:"validateOnRead" yaml:"validateOnRead"`

	// ValidateOnWrite validates data on serialization
	ValidateOnWrite bool `json:"validateOnWrite" yaml:"validateOnWrite"`
}

// ValidationError represents a schema validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   interface{} `json:"value,omitempty"`
}

// Error implements the error interface
func (v *ValidationError) Error() string {
	if v.Value != nil {
		return v.Field + ": " + v.Message + " (value: " + string(v.Value.([]byte)) + ")"
	}
	return v.Field + ": " + v.Message
}

// SerializationStats tracks serialization performance
type SerializationStats struct {
	EncodeDuration time.Duration
	DecodeDuration time.Duration
	BytesEncoded   int64
	BytesDecoded   int64
	Errors         int64
	Successes      int64
}
