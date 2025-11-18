# Changelog

All notable changes to the Gress Kubernetes Operator will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2024-01-01

### Added

#### Core Features
- Custom Resource Definitions (CRDs) for stream processing resources:
  - **StreamJob**: Define and manage stream processing jobs
  - **StreamCluster**: Manage distributed processing clusters
  - **Checkpoint**: Represent checkpoint and savepoint states

#### Operator Controller Logic
- **StreamJob Controller**: Full lifecycle management of stream processing jobs
  - StatefulSet creation for stateful operators (RocksDB)
  - Deployment creation for stateless operators
  - Service creation for metrics exposure
  - ConfigMap generation from job specifications
  - Status tracking and condition management

- **StreamCluster Controller**: Cluster resource management
  - Job Manager StatefulSet with HA support
  - Task Manager StatefulSet with configurable slots
  - Service creation for cluster components
  - Monitoring integration with ServiceMonitor support

- **Checkpoint Controller**: Checkpoint and savepoint management
  - Periodic checkpoint coordination
  - Manual savepoint triggering
  - Retention policy enforcement
  - Checkpoint cleanup and expiration

#### StatefulSet Management
- Persistent state storage with RocksDB backend
- PersistentVolumeClaim templates for state and checkpoints
- Volume management for stateful operators
- Proper pod identity and stable network IDs

#### Auto-Scaling
- Kafka lag-based auto-scaling
- CPU and memory-based scaling metrics
- Custom metrics support via Prometheus
- Configurable scaling policies (scale-up, scale-down)
- Stabilization windows to prevent flapping

#### Rolling Updates with Savepoints
- Automatic savepoint creation before updates
- Zero-downtime rolling updates
- Savepoint restoration on pod restart
- Update history tracking

#### High Availability
- Leader election for operator instances
- Multi-replica Job Manager support
- Kubernetes-native HA mode
- ZooKeeper HA mode support
- Failure detection and recovery

#### Storage Backends
- **Filesystem**: PVC-based checkpoint storage
- **Amazon S3**: S3-compatible object storage
- **Google Cloud Storage**: GCS checkpoint storage
- Configurable retention policies

#### Monitoring and Observability
- Prometheus metrics endpoint
- ServiceMonitor CRD support
- Per-job metrics collection
- Checkpoint statistics
- Kafka consumer lag metrics

#### Helm Chart
- Production-ready Helm chart
- Configurable deployment options
- RBAC templates
- ServiceAccount management
- Security contexts
- Resource limits and requests
- Pod scheduling configurations

#### Documentation
- Comprehensive README with usage examples
- Detailed installation guide
- Configuration reference
- Troubleshooting guide
- Sample manifests for common use cases

### Configuration Options

#### StreamJob Specifications
- Source configurations (Kafka, HTTP, WebSocket, NATS)
- Sink configurations (Kafka, TimescaleDB, HTTP)
- Pipeline operators (map, filter, aggregate, window, join)
- State backend settings
- Checkpoint configuration
- Auto-scaling policies
- Resource requirements
- Pod scheduling (affinity, tolerations, node selectors)

#### StreamCluster Specifications
- Job Manager configuration
- Task Manager configuration
- High availability settings
- Monitoring configuration
- Logging configuration
- Network policies
- Security contexts

#### Checkpoint Specifications
- Checkpoint types (checkpoint, savepoint)
- Trigger modes (periodic, manual, pre-upgrade)
- Storage configuration
- Retention policies

### Security

- Pod security contexts
- Container security contexts
- RBAC with minimal required permissions
- TLS support for Kafka sources/sinks
- SASL authentication support
- Secret management for credentials

### Examples

- Kafka processing job with auto-scaling
- Production StreamCluster with HA
- Manual savepoint creation
- S3-backed checkpoint storage
- Multi-source data ingestion

### Technical Details

- **Language**: Go 1.21+
- **Framework**: Kubebuilder/controller-runtime
- **Kubernetes Version**: 1.20+
- **Container Base**: distroless/static (security-hardened)

### Known Limitations

- Manual job specification (declarative pipeline DSL in future release)
- Basic Kafka lag calculation (enhanced metrics in future release)
- Limited webhook support (validation/mutation webhooks planned)

## [Unreleased]

### Planned

- Admission webhooks for validation and mutation
- Enhanced auto-scaling with custom metrics adapters
- Job scheduling and dependency management
- Multi-tenancy support with namespace isolation
- Integration with service mesh (Istio, Linkerd)
- Advanced checkpoint compression
- Incremental checkpointing optimization
- Dashboard UI for job management
- CLI tool for operator interaction

[0.1.0]: https://github.com/therealutkarshpriyadarshi/gress/releases/tag/operator-v0.1.0
