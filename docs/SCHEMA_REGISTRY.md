# Schema Registry Integration

Gress provides comprehensive integration with Confluent Schema Registry, enabling data quality enforcement, schema evolution management, and automatic serialization/deserialization for stream processing pipelines.

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Supported Schema Formats](#supported-schema-formats)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Schema Evolution](#schema-evolution)
- [API Reference](#api-reference)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

## Overview

Schema Registry integration provides:

- **Data Quality**: Ensure all data conforms to defined schemas
- **Schema Evolution**: Manage schema changes safely with compatibility checking
- **Automatic Serialization**: Convert between objects and wire format automatically
- **Type Safety**: Strong typing for stream events
- **Performance**: Local caching and efficient binary formats

## Features

### Core Features

- ✅ **Multi-format Support**: Avro, Protocol Buffers, JSON Schema
- ✅ **Automatic Codec Selection**: Transparent serialization/deserialization
- ✅ **Schema Versioning**: Pin to specific versions or use latest
- ✅ **Compatibility Checking**: Multiple compatibility modes (BACKWARD, FORWARD, FULL, etc.)
- ✅ **Local Caching**: Reduce latency with configurable schema caching
- ✅ **Auto-registration**: Automatically register new schemas
- ✅ **Validation Operators**: Stream processing validation operators
- ✅ **Metrics Integration**: Prometheus metrics for monitoring
- ✅ **TLS Support**: Secure connections to Schema Registry
- ✅ **Authentication**: Basic auth support

## Supported Schema Formats

### 1. Apache Avro

**Best for**: Compact storage, rich schema evolution, complex data structures

```json
{
  "type": "record",
  "name": "User",
  "fields": [
    {"name": "id", "type": "string"},
    {"name": "email", "type": "string"},
    {"name": "age", "type": ["null", "int"], "default": null}
  ]
}
```

**Advantages**:
- Compact binary format
- Rich schema evolution features
- Excellent Kafka ecosystem support

### 2. Protocol Buffers

**Best for**: Performance-critical applications, microservices, cross-language compatibility

```protobuf
syntax = "proto3";

message User {
  string id = 1;
  string email = 2;
  int32 age = 3;
}
```

**Advantages**:
- Highly efficient binary encoding
- Strong typing
- Wide language support

### 3. JSON Schema

**Best for**: REST APIs, human-readable data, flexible validation

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "id": {"type": "string"},
    "email": {"type": "string", "format": "email"},
    "age": {"type": "integer", "minimum": 0}
  },
  "required": ["id", "email"]
}
```

**Advantages**:
- Human-readable
- Flexible validation rules
- Standard JSON tooling

## Quick Start

### 1. Enable Schema Registry

```yaml
# config.yaml
schema_registry:
  enabled: true
  url: "http://localhost:8081"
  cache_enabled: true
  cache_size: 1000
  cache_ttl: 5m
```

### 2. Configure Kafka Source

```yaml
sources:
  kafka:
    - name: "user-events"
      brokers: ["localhost:9092"]
      topics: ["users"]
      group_id: "gress-consumer"

      # Schema configuration for values
      value_schema:
        subject: "users-value"
        schema_type: "AVRO"
        version: 0  # 0 = latest
        validate_on_read: true
```

### 3. Configure Kafka Sink

```yaml
sinks:
  kafka:
    - name: "processed-users"
      brokers: ["localhost:9092"]
      topic: "processed-users"

      # Schema configuration for values
      value_schema:
        subject: "processed-users-value"
        schema_type: "AVRO"
        version: 0
        auto_register: true
        compatibility: "BACKWARD"
        validate_on_write: true
```

### 4. Run Gress

```bash
gress --config config.yaml
```

## Configuration

### Schema Registry Settings

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | false | Enable schema registry integration |
| `url` | string | - | Schema registry URL |
| `username` | string | - | Basic auth username |
| `password` | string | - | Basic auth password |
| `timeout` | duration | 30s | Request timeout |
| `cache_enabled` | bool | true | Enable local schema caching |
| `cache_size` | int | 1000 | Maximum schemas to cache |
| `cache_ttl` | duration | 5m | Cache time-to-live |
| `tls_enabled` | bool | false | Enable TLS |
| `tls_cert_path` | string | - | Client certificate path |
| `tls_key_path` | string | - | Client key path |
| `tls_ca_path` | string | - | CA certificate path |
| `tls_skip_verify` | bool | false | Skip TLS verification (insecure) |

### Source Schema Configuration

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `subject` | string | - | Schema registry subject name |
| `schema_type` | string | - | AVRO, PROTOBUF, or JSON |
| `version` | int | 0 | Schema version (0 = latest) |
| `validate_on_read` | bool | false | Validate data on deserialization |

### Sink Schema Configuration

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `subject` | string | - | Schema registry subject name |
| `schema_type` | string | - | AVRO, PROTOBUF, or JSON |
| `version` | int | 0 | Schema version (0 = latest) |
| `auto_register` | bool | false | Auto-register new schemas |
| `compatibility` | string | BACKWARD | Compatibility mode |
| `validate_on_write` | bool | false | Validate data on serialization |

## Schema Evolution

### Compatibility Modes

#### BACKWARD
New schema can read data written with previous schema.

**Use case**: Adding optional fields

**Safe changes**:
- Add optional fields with defaults
- Remove optional fields

**Example**:
```json
// Version 1
{"type": "record", "name": "User", "fields": [
  {"name": "id", "type": "string"}
]}

// Version 2 (BACKWARD compatible)
{"type": "record", "name": "User", "fields": [
  {"name": "id", "type": "string"},
  {"name": "email", "type": ["null", "string"], "default": null}
]}
```

#### FORWARD
Old schema can read data written with new schema.

**Use case**: Removing fields

**Safe changes**:
- Add optional fields
- Remove fields

#### FULL
New schema is both BACKWARD and FORWARD compatible.

**Use case**: Maximum flexibility with safety

**Safe changes**:
- Add optional fields with defaults only

#### BACKWARD_TRANSITIVE
New schema compatible with ALL previous versions.

#### FORWARD_TRANSITIVE
Old schemas compatible with ALL new versions.

#### FULL_TRANSITIVE
All versions are backward and forward compatible.

#### NONE
No compatibility checking.

**Use case**: Development and testing only

### Evolution Example

```bash
# Version 1: Initial schema
curl -X POST http://localhost:8081/subjects/users-value/versions \
  -H "Content-Type: application/vnd.schemaregistry.v1+json" \
  -d '{
    "schema": "{\"type\":\"record\",\"name\":\"User\",\"fields\":[{\"name\":\"id\",\"type\":\"string\"}]}"
  }'

# Version 2: Add optional field (BACKWARD compatible)
curl -X POST http://localhost:8081/subjects/users-value/versions \
  -H "Content-Type: application/vnd.schemaregistry.v1+json" \
  -d '{
    "schema": "{\"type\":\"record\",\"name\":\"User\",\"fields\":[{\"name\":\"id\",\"type\":\"string\"},{\"name\":\"email\",\"type\":[\"null\",\"string\"],\"default\":null}]}"
  }'

# Test compatibility before registering
curl -X POST http://localhost:8081/compatibility/subjects/users-value/versions/latest \
  -H "Content-Type: application/vnd.schemaregistry.v1+json" \
  -d '{
    "schema": "..."
  }'
```

## API Reference

### CodecManager

Main interface for schema operations.

```go
import "github.com/therealutkarshpriyadarshi/gress/pkg/schema"

// Create codec manager
config := &schema.Config{
    RegistryURL: "http://localhost:8081",
    CacheEnabled: true,
    CacheSize: 1000,
    CacheTTL: 5 * time.Minute,
}

registry, _ := schema.NewRegistryClient(config, logger)
codecManager := schema.NewCodecManager(registry, logger)

// Encode data
schemaConfig := &schema.SchemaConfig{
    Subject: "users-value",
    SchemaType: schema.SchemaTypeAvro,
    Version: 0,
}

encoded, _ := codecManager.Encode(ctx, "users-value", data, schemaConfig)

// Decode data
decoded, _ := codecManager.Decode(ctx, encoded, schemaConfig)
```

### Schema Validation Operator

```go
import "github.com/therealutkarshpriyadarshi/gress/pkg/schema"

// Create validator
validator := schema.NewValidatorOperator(codecManager, schemaConfig, logger)

// Validate event
validatedEvent, err := validator.Validate(ctx, event)
if err != nil {
    // Handle validation failure
    log.Error("Validation failed", zap.Error(err))
}

// Filter invalid events
validEvent := validator.ValidateOrFilter(ctx, event)
if validEvent == nil {
    // Event was invalid
}
```

### Registry Client

Direct interaction with Schema Registry.

```go
// Get latest schema
metadata, _ := registry.GetLatestSchema(ctx, "users-value")

// Get specific version
metadata, _ := registry.GetSchemaByVersion(ctx, "users-value", 2)

// Register new schema
metadata, _ := registry.RegisterSchema(ctx, "users-value", schemaJSON,
    schema.SchemaTypeAvro, nil)

// Test compatibility
compatible, _ := registry.TestCompatibility(ctx, "users-value",
    newSchema, schema.SchemaTypeAvro, 1)
```

## Best Practices

### 1. Schema Design

- **Use meaningful names**: Clear, descriptive field names
- **Add documentation**: Include field descriptions
- **Plan for evolution**: Use optional fields with defaults
- **Avoid breaking changes**: Never rename or remove required fields

### 2. Compatibility

- **Start strict**: Begin with FULL compatibility
- **Test before deploying**: Use compatibility testing API
- **Version carefully**: Consider impact on consumers
- **Document changes**: Maintain changelog for schemas

### 3. Performance

- **Enable caching**: Reduce registry calls
- **Use appropriate cache TTL**: Balance freshness vs performance
- **Pin versions in production**: Avoid surprises from latest version
- **Monitor metrics**: Track schema operation performance

### 4. Error Handling

- **Validate early**: Use `validate_on_read` and `validate_on_write`
- **Handle failures gracefully**: Configure dead letter queues
- **Log schema errors**: Include schema ID and subject in logs
- **Alert on validation failures**: Monitor validation error rates

### 5. Security

- **Enable TLS**: Use encrypted connections in production
- **Use authentication**: Protect schema registry access
- **Restrict permissions**: Limit who can register schemas
- **Audit changes**: Track schema modifications

## Monitoring

Gress exposes Prometheus metrics for schema operations:

```prometheus
# Total operations
gress_schema_operations_total{operation="encode",schema_type="AVRO",status="success"}

# Operation duration
gress_schema_operation_duration_seconds{operation="decode",schema_type="AVRO"}

# Cache size
gress_schema_cache_size{codec_type="AVRO"}
```

### Example Alerts

```yaml
groups:
  - name: schema_registry
    rules:
      - alert: HighSchemaValidationErrors
        expr: rate(gress_schema_operations_total{status="error"}[5m]) > 0.01
        annotations:
          summary: "High schema validation error rate"

      - alert: SchemaRegistryDown
        expr: up{job="schema-registry"} == 0
        annotations:
          summary: "Schema Registry is down"
```

## Troubleshooting

### Issue: Schema Not Found

**Symptoms**: `schema not found` errors

**Solutions**:
1. Verify schema is registered:
   ```bash
   curl http://localhost:8081/subjects/my-subject/versions
   ```
2. Check subject naming convention (typically `<topic>-value` or `<topic>-key`)
3. Ensure schema registry URL is correct
4. Verify network connectivity

### Issue: Incompatible Schema

**Symptoms**: `schema is not compatible` errors

**Solutions**:
1. Check compatibility mode configuration
2. Review schema changes for breaking changes
3. Test compatibility before deploying:
   ```bash
   curl -X POST http://localhost:8081/compatibility/subjects/my-subject/versions/latest \
     -d '{"schema": "..."}'
   ```
4. Consider using a less strict compatibility mode for development

### Issue: Performance Problems

**Symptoms**: Slow serialization/deserialization

**Solutions**:
1. Enable schema caching: `cache_enabled: true`
2. Increase cache size: `cache_size: 10000`
3. Adjust cache TTL: `cache_ttl: 10m`
4. Monitor cache hit rate via metrics
5. Consider using Avro for better performance

### Issue: Validation Errors

**Symptoms**: Data doesn't match schema

**Solutions**:
1. Check schema definition matches data structure
2. Verify required fields are present
3. Check data types match schema
4. Use validation in development: `validate_on_read: true`
5. Review validation error logs for details

## Examples

See [examples/schema-registry/](../examples/schema-registry/) for complete working examples with:

- Avro schema configuration
- Protocol Buffers configuration
- JSON Schema configuration
- Sample schemas
- Setup instructions

## References

- [Confluent Schema Registry Documentation](https://docs.confluent.io/platform/current/schema-registry/)
- [Avro Specification](https://avro.apache.org/docs/current/spec.html)
- [Protocol Buffers Documentation](https://developers.google.com/protocol-buffers)
- [JSON Schema Specification](https://json-schema.org/)
- [Confluent Wire Format](https://docs.confluent.io/platform/current/schema-registry/serdes-develop/index.html#wire-format)
