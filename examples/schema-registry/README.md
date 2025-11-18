# Schema Registry Integration Examples

This directory contains examples demonstrating how to use Gress with Confluent Schema Registry for data quality and schema evolution.

## Overview

Gress supports three schema formats:
- **Avro**: Compact binary format with rich schema evolution features
- **Protocol Buffers**: Efficient binary serialization with strong typing
- **JSON Schema**: Human-readable schema validation for JSON data

## Prerequisites

1. **Confluent Schema Registry** running on `http://localhost:8081`
2. **Apache Kafka** running on `localhost:9092`
3. **Gress** installed and configured

### Starting Schema Registry (Docker)

```bash
docker run -d \
  --name schema-registry \
  -p 8081:8081 \
  -e SCHEMA_REGISTRY_HOST_NAME=schema-registry \
  -e SCHEMA_REGISTRY_KAFKASTORE_BOOTSTRAP_SERVERS=localhost:9092 \
  confluentinc/cp-schema-registry:latest
```

## Example Configurations

### 1. Avro Schema Example

**Config**: `config-avro.yaml`

This example demonstrates:
- Reading Avro-encoded messages from Kafka
- Validating data against Avro schemas
- Writing processed data with Avro encoding
- Automatic schema registration and compatibility checking

**Schema**: `user-events.avsc`

```bash
# Register the schema
curl -X POST http://localhost:8081/subjects/user-events-value/versions \
  -H "Content-Type: application/vnd.schemaregistry.v1+json" \
  -d @- <<EOF
{
  "schema": "$(cat user-events.avsc | jq -c . | jq -R .)"
}
EOF

# Run Gress with Avro config
gress --config config-avro.yaml
```

### 2. Protocol Buffers Example

**Config**: `config-protobuf.yaml`

This example demonstrates:
- Processing Protobuf-encoded sensor data
- Schema versioning with backward transitive compatibility
- Efficient binary serialization

```bash
# Run Gress with Protobuf config
gress --config config-protobuf.yaml
```

### 3. JSON Schema Example

**Config**: `config-jsonschema.yaml`

**Schema**: `api-events.json`

This example demonstrates:
- Validating JSON data against JSON Schema
- Full compatibility mode (backward + forward)
- Dead letter queue for invalid messages

```bash
# Register the JSON Schema
curl -X POST http://localhost:8081/subjects/api-events-value/versions \
  -H "Content-Type: application/vnd.schemaregistry.v1+json" \
  -d @- <<EOF
{
  "schemaType": "JSON",
  "schema": "$(cat api-events.json | jq -c . | jq -R .)"
}
EOF

# Run Gress with JSON Schema config
gress --config config-jsonschema.yaml
```

## Configuration Options

### Schema Registry Settings

```yaml
schema_registry:
  enabled: true                    # Enable schema registry integration
  url: "http://localhost:8081"     # Schema registry URL
  cache_enabled: true              # Enable local schema caching
  cache_size: 1000                 # Max number of schemas to cache
  cache_ttl: 5m                    # Cache time-to-live
  timeout: 30s                     # Request timeout

  # Optional: Authentication
  username: "user"
  password: "secret"

  # Optional: TLS
  tls_enabled: true
  tls_cert_path: "/path/to/cert.pem"
  tls_key_path: "/path/to/key.pem"
  tls_ca_path: "/path/to/ca.pem"
```

### Source Schema Configuration

```yaml
sources:
  kafka:
    - name: "my-source"
      # ... kafka config ...
      value_schema:
        subject: "my-topic-value"       # Schema registry subject
        schema_type: "AVRO"              # AVRO, PROTOBUF, or JSON
        version: 0                       # 0 = latest, or specific version
        validate_on_read: true           # Validate on deserialization
      key_schema:                        # Optional key schema
        subject: "my-topic-key"
        schema_type: "AVRO"
        version: 0
```

### Sink Schema Configuration

```yaml
sinks:
  kafka:
    - name: "my-sink"
      # ... kafka config ...
      value_schema:
        subject: "output-topic-value"
        schema_type: "AVRO"
        version: 0
        auto_register: true              # Auto-register new schemas
        compatibility: "BACKWARD"         # Compatibility mode
        validate_on_write: true          # Validate on serialization
```

## Compatibility Modes

| Mode | Description | Use Case |
|------|-------------|----------|
| `BACKWARD` | New schema can read old data | Adding optional fields |
| `BACKWARD_TRANSITIVE` | New schema compatible with all old versions | Strict backward compatibility |
| `FORWARD` | Old schema can read new data | Removing fields |
| `FORWARD_TRANSITIVE` | Old schemas compatible with all new versions | Strict forward compatibility |
| `FULL` | New schema backward + forward compatible | Adding optional fields only |
| `FULL_TRANSITIVE` | All versions are backward + forward compatible | Strictest compatibility |
| `NONE` | No compatibility checking | Development/testing |

## Schema Evolution Best Practices

### 1. Avro Evolution

**Safe changes:**
- Adding fields with defaults
- Removing fields with defaults
- Changing field defaults
- Adding/removing union types

**Breaking changes:**
- Renaming fields
- Changing field types
- Removing required fields

### 2. Protobuf Evolution

**Safe changes:**
- Adding new fields
- Removing deprecated fields
- Adding new message types

**Breaking changes:**
- Changing field types
- Changing field numbers
- Renaming fields (without field aliases)

### 3. JSON Schema Evolution

**Safe changes (BACKWARD):**
- Adding optional properties
- Making required fields optional
- Relaxing constraints (e.g., removing `minimum`)

**Safe changes (FORWARD):**
- Adding required properties (with defaults)
- Tightening constraints

## Monitoring Schema Operations

Gress exposes Prometheus metrics for schema operations:

```
# Schema operations
gress_schema_operations_total{operation="encode",schema_type="AVRO",status="success"} 1000
gress_schema_operations_total{operation="decode",schema_type="AVRO",status="error"} 5

# Schema operation duration
gress_schema_operation_duration_seconds{operation="encode",schema_type="AVRO"} 0.001

# Schema cache
gress_schema_cache_size{codec_type="AVRO"} 42
```

## Testing Schema Changes

1. **Register new schema version:**
```bash
curl -X POST http://localhost:8081/subjects/my-subject/versions \
  -H "Content-Type: application/vnd.schemaregistry.v1+json" \
  -d '{"schema": "..."}'
```

2. **Check compatibility:**
```bash
curl -X POST http://localhost:8081/compatibility/subjects/my-subject/versions/latest \
  -H "Content-Type: application/vnd.schemaregistry.v1+json" \
  -d '{"schema": "..."}'
```

3. **List versions:**
```bash
curl http://localhost:8081/subjects/my-subject/versions
```

4. **Get schema by ID:**
```bash
curl http://localhost:8081/schemas/ids/1
```

## Troubleshooting

### Schema Not Found
- Verify schema is registered: `curl http://localhost:8081/subjects`
- Check subject naming: Should be `<topic>-value` or `<topic>-key`
- Ensure schema registry is accessible

### Compatibility Errors
- Review compatibility mode in config
- Check breaking changes in schema evolution
- Use schema registry API to test compatibility before deploying

### Performance Issues
- Enable schema caching: `cache_enabled: true`
- Increase cache size: `cache_size: 10000`
- Adjust cache TTL: `cache_ttl: 10m`

## Advanced Features

### Schema Validation Operator

Use the validation operator in your stream processing pipeline:

```go
import "github.com/therealutkarshpriyadarshi/gress/pkg/schema"

// Create validator
validator := schema.NewValidatorOperator(codecManager, schemaConfig, logger)

// Validate events
validatedEvent, err := validator.Validate(ctx, event)
if err != nil {
    // Handle validation failure
}
```

### Multi-Schema Validation

Support events matching multiple schemas:

```go
multiValidator := schema.NewMultiSchemaValidator(
    []*schema.ValidatorOperator{validator1, validator2},
    logger,
)
```

## References

- [Confluent Schema Registry Documentation](https://docs.confluent.io/platform/current/schema-registry/index.html)
- [Avro Specification](https://avro.apache.org/docs/current/spec.html)
- [Protocol Buffers Guide](https://developers.google.com/protocol-buffers)
- [JSON Schema Specification](https://json-schema.org/)
