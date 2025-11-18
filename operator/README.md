# Gress Kubernetes Operator

The Gress Kubernetes Operator enables native Kubernetes deployment and management of Gress stream processing applications. It provides Custom Resource Definitions (CRDs) for declarative configuration and automated lifecycle management of stream processing jobs and clusters.

## Features

- **Declarative Job Configuration**: Define stream processing jobs using Kubernetes custom resources
- **StatefulSet Management**: Automatic management of stateful operators with persistent storage
- **Auto-Scaling**: Built-in auto-scaling based on Kafka lag, CPU, and memory metrics
- **Checkpoint Management**: Automated checkpointing and savepoint creation for exactly-once semantics
- **Rolling Updates**: Zero-downtime updates with automatic savepoint creation
- **High Availability**: Support for multi-replica job managers with leader election
- **Monitoring Integration**: Native Prometheus metrics and ServiceMonitor support
- **Storage Flexibility**: Support for filesystem, S3, and GCS checkpoint storage

## Architecture

The operator manages three main custom resources:

1. **StreamJob**: Defines individual stream processing jobs
2. **StreamCluster**: Manages distributed processing clusters with job and task managers
3. **Checkpoint**: Represents checkpoint and savepoint states

## Quick Start

### Prerequisites

- Kubernetes 1.20+
- kubectl configured to communicate with your cluster
- Helm 3.0+ (for Helm installation)

### Installation

#### Using Helm

```bash
# Add the Gress Helm repository (when published)
helm repo add gress https://charts.gress.io
helm repo update

# Install the operator
helm install gress-operator gress/gress-operator \
  --namespace gress-system \
  --create-namespace

# Or install from local chart
cd operator/helm
helm install gress-operator ./gress-operator \
  --namespace gress-system \
  --create-namespace
```

#### Using kubectl

```bash
# Install CRDs
kubectl apply -f operator/config/crd/

# Install RBAC and operator
kubectl apply -f operator/config/rbac/
kubectl apply -f operator/config/manager/
```

### Verify Installation

```bash
# Check operator pod is running
kubectl get pods -n gress-system

# Check CRDs are installed
kubectl get crds | grep gress.io
```

## Usage

### Creating a Stream Processing Job

Create a simple Kafka processing job:

```yaml
apiVersion: gress.io/v1alpha1
kind: StreamJob
metadata:
  name: my-stream-job
  namespace: default
spec:
  image: ghcr.io/therealutkarshpriyadarshi/gress:latest
  parallelism: 3

  stateBackend:
    type: rocksdb
    persistentVolumeClaim:
      accessModes:
        - ReadWriteOnce
      resources:
        requests:
          storage: 10Gi

  checkpoint:
    interval: "30s"
    mode: exactly-once
    storage:
      type: filesystem
      persistentVolumeClaim:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 20Gi

  sources:
    - name: kafka-input
      type: kafka
      kafka:
        brokers:
          - kafka:9092
        topics:
          - input-topic
        groupID: my-consumer-group

  sinks:
    - name: kafka-output
      type: kafka
      kafka:
        brokers:
          - kafka:9092
        topic: output-topic
```

Apply the configuration:

```bash
kubectl apply -f my-stream-job.yaml
```

### Monitoring Job Status

```bash
# Get job status
kubectl get streamjob my-stream-job

# Get detailed status
kubectl describe streamjob my-stream-job

# Watch job pods
kubectl get pods -l streamjob=my-stream-job -w

# View logs
kubectl logs -l streamjob=my-stream-job -f
```

### Creating a Stream Cluster

For production deployments with dedicated job and task managers:

```yaml
apiVersion: gress.io/v1alpha1
kind: StreamCluster
metadata:
  name: production-cluster
spec:
  version: "0.1.0"

  jobManagers:
    replicas: 3
    resources:
      requests:
        cpu: 500m
        memory: 1Gi

  taskManagers:
    replicas: 5
    taskSlots: 2
    resources:
      requests:
        cpu: 1000m
        memory: 2Gi

  highAvailability:
    enabled: true
    mode: kubernetes

  monitoring:
    enabled: true
    serviceMonitor:
      enabled: true
```

### Managing Checkpoints and Savepoints

#### Trigger a manual savepoint

```yaml
apiVersion: gress.io/v1alpha1
kind: Checkpoint
metadata:
  name: my-savepoint
spec:
  streamJobName: my-stream-job
  type: savepoint
  triggerMode: manual
  retentionPolicy:
    retainOnJobCompletion: true
    maxCount: 5
```

#### Restore from a savepoint

```yaml
apiVersion: gress.io/v1alpha1
kind: StreamJob
metadata:
  name: my-stream-job
spec:
  savepointPath: /checkpoints/savepoint-12345
  # ... rest of configuration
```

### Auto-Scaling

Enable auto-scaling based on Kafka lag:

```yaml
spec:
  autoScaler:
    enabled: true
    minReplicas: 2
    maxReplicas: 10
    metrics:
      - type: kafka_lag
        target: "5000"
      - type: cpu
        target: "70"
    behavior:
      scaleUp:
        stabilizationWindowSeconds: 60
        policies:
          - type: Pods
            value: 2
            periodSeconds: 60
      scaleDown:
        stabilizationWindowSeconds: 300
```

### Rolling Updates

Update a job with zero downtime:

```bash
# Operator automatically creates a savepoint before update
kubectl apply -f my-stream-job-v2.yaml

# Monitor the update
kubectl rollout status statefulset my-stream-job
```

## Configuration

### StreamJob Configuration

Key configuration options:

- **parallelism**: Number of parallel task instances
- **stateBackend**: State storage configuration (rocksdb, memory)
- **checkpoint**: Checkpointing settings (interval, mode, storage)
- **sources**: Data source configurations (kafka, http, websocket, nats)
- **sinks**: Data sink configurations (kafka, timescaledb, http)
- **pipeline**: Processing operators (map, filter, aggregate, window, join)
- **autoScaler**: Auto-scaling configuration
- **resources**: CPU and memory requirements

### StreamCluster Configuration

Key configuration options:

- **jobManagers**: Job manager configuration (replicas, resources, storage)
- **taskManagers**: Task manager configuration (replicas, slots, resources)
- **highAvailability**: HA mode and settings
- **monitoring**: Prometheus metrics and ServiceMonitor
- **logging**: Log level and aggregation

### Checkpoint Configuration

Key configuration options:

- **type**: checkpoint or savepoint
- **triggerMode**: periodic, manual, pre-upgrade, on-cancel
- **storage**: Storage backend (filesystem, s3, gcs)
- **retentionPolicy**: Retention rules (maxCount, maxAge)

## Advanced Topics

### High Availability

For production deployments, enable HA with multiple job managers:

```yaml
spec:
  jobManagers:
    replicas: 3  # Recommended: 1, 3, or 5
  highAvailability:
    enabled: true
    mode: kubernetes  # or zookeeper
```

### Storage Backends

#### Filesystem (PVC)

```yaml
checkpoint:
  storage:
    type: filesystem
    path: /var/lib/gress/checkpoints
    persistentVolumeClaim:
      storageClassName: fast-ssd
      resources:
        requests:
          storage: 50Gi
```

#### S3

```yaml
checkpoint:
  storage:
    type: s3
    s3:
      bucket: my-checkpoints
      prefix: production/
      region: us-east-1
      credentialsSecret:
        name: aws-credentials
```

#### Google Cloud Storage

```yaml
checkpoint:
  storage:
    type: gcs
    gcs:
      bucket: my-checkpoints
      prefix: production/
      credentialsSecret:
        name: gcs-credentials
```

### Security

#### TLS for Kafka

```yaml
sources:
  - name: kafka-secure
    type: kafka
    kafka:
      brokers:
        - kafka:9093
      tls:
        enabled: true
        caSecret:
          name: kafka-ca-cert
        certSecret:
          name: kafka-client-cert
```

#### SASL Authentication

```yaml
sources:
  - name: kafka-sasl
    type: kafka
    kafka:
      brokers:
        - kafka:9092
      sasl:
        mechanism: SCRAM-SHA-512
        credentialsSecret:
          name: kafka-credentials
```

### Monitoring

#### Prometheus Integration

```yaml
monitoring:
  enabled: true
  serviceMonitor:
    enabled: true
    interval: 30s
    labels:
      prometheus: kube-prometheus
```

Metrics available:
- `gress_events_processed_total`
- `gress_events_per_second`
- `gress_checkpoint_duration_seconds`
- `gress_state_size_bytes`
- `gress_kafka_consumer_lag`

#### Grafana Dashboards

Pre-built dashboards are available in `deployments/grafana/`.

## Troubleshooting

### Job Not Starting

Check operator logs:
```bash
kubectl logs -n gress-system deployment/gress-operator
```

Check job events:
```bash
kubectl describe streamjob my-stream-job
```

### Checkpoint Failures

View checkpoint status:
```bash
kubectl get checkpoint -l streamjob=my-stream-job
kubectl describe checkpoint <checkpoint-name>
```

### Performance Issues

1. Check resource usage:
   ```bash
   kubectl top pods -l streamjob=my-stream-job
   ```

2. Enable auto-scaling:
   ```yaml
   spec:
     autoScaler:
       enabled: true
   ```

3. Increase parallelism:
   ```yaml
   spec:
     parallelism: 5
   ```

## Development

### Building the Operator

```bash
cd operator
make build
```

### Running Locally

```bash
make install  # Install CRDs
make run      # Run operator locally
```

### Testing

```bash
make test
```

### Building Docker Image

```bash
make docker-build IMG=myregistry/gress-operator:v0.1.0
make docker-push IMG=myregistry/gress-operator:v0.1.0
```

## Contributing

See [CONTRIBUTING.md](../CONTRIBUTING.md) for guidelines.

## License

Licensed under the Apache License 2.0. See [LICENSE](../LICENSE) for details.

## Support

- GitHub Issues: https://github.com/therealutkarshpriyadarshi/gress/issues
- Documentation: https://docs.gress.io
- Slack: https://gress-community.slack.com
