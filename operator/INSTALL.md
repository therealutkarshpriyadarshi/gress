# Gress Operator Installation Guide

This guide provides detailed instructions for installing and configuring the Gress Kubernetes Operator.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Installation Methods](#installation-methods)
  - [Helm Installation](#helm-installation)
  - [kubectl Installation](#kubectl-installation)
  - [Operator Lifecycle Manager (OLM)](#operator-lifecycle-manager-olm)
- [Configuration](#configuration)
- [Verification](#verification)
- [Upgrading](#upgrading)
- [Uninstallation](#uninstallation)

## Prerequisites

### Kubernetes Cluster Requirements

- Kubernetes version 1.20 or higher
- kubectl configured to communicate with your cluster
- Sufficient cluster resources:
  - At least 2 nodes (recommended 3+ for HA)
  - 4 CPU cores and 8GB RAM minimum per node
  - Storage class for dynamic provisioning (for persistent state)

### Required Permissions

You need cluster-admin permissions to:
- Create Custom Resource Definitions (CRDs)
- Create ClusterRoles and ClusterRoleBindings
- Create namespaces

### Optional Dependencies

- **Prometheus Operator**: For ServiceMonitor support
- **cert-manager**: For webhook certificate management (if using webhooks)
- **Kafka**: For Kafka-based stream processing
- **Object Storage**: S3 or GCS for checkpoint storage (optional)

## Installation Methods

### Helm Installation

Helm is the recommended installation method.

#### Step 1: Add Helm Repository

```bash
# Add the Gress Helm repository (when published)
helm repo add gress https://charts.gress.io
helm repo update
```

For local development:
```bash
cd operator/helm
```

#### Step 2: Create Namespace

```bash
kubectl create namespace gress-system
```

#### Step 3: Install the Operator

Basic installation:
```bash
helm install gress-operator gress/gress-operator \
  --namespace gress-system
```

With custom values:
```bash
helm install gress-operator gress/gress-operator \
  --namespace gress-system \
  --values custom-values.yaml
```

#### Step 4: Verify Installation

```bash
kubectl get pods -n gress-system
kubectl get crds | grep gress.io
```

### kubectl Installation

For manual installation without Helm.

#### Step 1: Install CRDs

```bash
kubectl apply -f https://raw.githubusercontent.com/therealutkarshpriyadarshi/gress/main/operator/config/crd/gress.io_streamjobs.yaml
kubectl apply -f https://raw.githubusercontent.com/therealutkarshpriyadarshi/gress/main/operator/config/crd/gress.io_streamclusters.yaml
kubectl apply -f https://raw.githubusercontent.com/therealutkarshpriyadarshi/gress/main/operator/config/crd/gress.io_checkpoints.yaml
```

#### Step 2: Create Namespace and RBAC

```bash
kubectl create namespace gress-system

kubectl apply -f - <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: gress-operator
  namespace: gress-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: gress-operator-role
rules:
# Add rules from clusterrole.yaml here
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: gress-operator-rolebinding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: gress-operator-role
subjects:
- kind: ServiceAccount
  name: gress-operator
  namespace: gress-system
EOF
```

#### Step 3: Deploy Operator

```bash
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gress-operator
  namespace: gress-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: gress-operator
  template:
    metadata:
      labels:
        app: gress-operator
    spec:
      serviceAccountName: gress-operator
      containers:
      - name: manager
        image: ghcr.io/therealutkarshpriyadarshi/gress-operator:0.1.0
        command:
        - /manager
        args:
        - --leader-elect
        resources:
          limits:
            cpu: 500m
            memory: 512Mi
          requests:
            cpu: 100m
            memory: 128Mi
EOF
```

### Operator Lifecycle Manager (OLM)

For clusters with OLM installed:

```bash
kubectl create -f https://operatorhub.io/install/gress-operator.yaml
```

## Configuration

### Basic Configuration

Create a `custom-values.yaml` file:

```yaml
operator:
  replicaCount: 1

  image:
    repository: ghcr.io/therealutkarshpriyadarshi/gress-operator
    tag: "0.1.0"

  resources:
    limits:
      cpu: 500m
      memory: 512Mi
    requests:
      cpu: 100m
      memory: 128Mi

  leaderElection:
    enabled: true

  metrics:
    enabled: true
    serviceMonitor:
      enabled: false
```

### High Availability Configuration

For production deployments:

```yaml
operator:
  replicaCount: 3  # Multiple replicas with leader election

  leaderElection:
    enabled: true

  affinity:
    podAntiAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
      - labelSelector:
          matchExpressions:
          - key: app.kubernetes.io/name
            operator: In
            values:
            - gress-operator
        topologyKey: kubernetes.io/hostname
```

### Prometheus Integration

Enable ServiceMonitor for Prometheus Operator:

```yaml
operator:
  metrics:
    enabled: true
    serviceMonitor:
      enabled: true
      interval: 30s
      labels:
        prometheus: kube-prometheus
```

### Resource Limits

Adjust based on cluster size:

```yaml
operator:
  resources:
    limits:
      cpu: 1000m
      memory: 1Gi
    requests:
      cpu: 200m
      memory: 256Mi
```

### Security Context

For restricted environments:

```yaml
operator:
  podSecurityContext:
    runAsNonRoot: true
    runAsUser: 1000
    fsGroup: 1000

  securityContext:
    allowPrivilegeEscalation: false
    capabilities:
      drop:
      - ALL
    readOnlyRootFilesystem: true
```

## Verification

### Check Operator Status

```bash
# Check operator pod
kubectl get pods -n gress-system

# Check operator logs
kubectl logs -n gress-system deployment/gress-operator

# Check CRDs
kubectl get crds | grep gress.io
```

Expected output:
```
NAME                                CREATED AT
checkpoints.gress.io               2024-01-01T00:00:00Z
streamclusters.gress.io            2024-01-01T00:00:00Z
streamjobs.gress.io                2024-01-01T00:00:00Z
```

### Create Test StreamJob

```bash
kubectl apply -f - <<EOF
apiVersion: gress.io/v1alpha1
kind: StreamJob
metadata:
  name: test-job
spec:
  image: ghcr.io/therealutkarshpriyadarshi/gress:latest
  parallelism: 1
  sources:
    - name: test-source
      type: kafka
      kafka:
        brokers:
          - kafka:9092
        topics:
          - test-topic
        groupID: test-group
EOF
```

Check job status:
```bash
kubectl get streamjob test-job
kubectl describe streamjob test-job
```

### Verify Metrics

If metrics are enabled:
```bash
kubectl port-forward -n gress-system deployment/gress-operator 8080:8080
curl http://localhost:8080/metrics
```

## Upgrading

### Helm Upgrade

```bash
# Update Helm repository
helm repo update

# Upgrade operator
helm upgrade gress-operator gress/gress-operator \
  --namespace gress-system \
  --values custom-values.yaml
```

### kubectl Upgrade

```bash
# Update CRDs (if changed)
kubectl apply -f operator/config/crd/

# Update deployment
kubectl set image deployment/gress-operator \
  manager=ghcr.io/therealutkarshpriyadarshi/gress-operator:0.2.0 \
  -n gress-system
```

### Upgrade Considerations

1. **CRD Changes**: Always apply CRD updates before upgrading the operator
2. **Breaking Changes**: Review release notes for breaking changes
3. **Backup**: Create backups of critical StreamJobs before upgrading:
   ```bash
   kubectl get streamjob -A -o yaml > streamjobs-backup.yaml
   ```

## Uninstallation

### Helm Uninstallation

```bash
# Uninstall operator
helm uninstall gress-operator --namespace gress-system

# Delete namespace
kubectl delete namespace gress-system

# Delete CRDs (WARNING: This deletes all StreamJobs, StreamClusters, and Checkpoints)
kubectl delete crd streamjobs.gress.io
kubectl delete crd streamclusters.gress.io
kubectl delete crd checkpoints.gress.io
```

### kubectl Uninstallation

```bash
# Delete operator deployment
kubectl delete deployment gress-operator -n gress-system

# Delete RBAC
kubectl delete clusterrolebinding gress-operator-rolebinding
kubectl delete clusterrole gress-operator-role
kubectl delete serviceaccount gress-operator -n gress-system

# Delete namespace
kubectl delete namespace gress-system

# Delete CRDs
kubectl delete crd streamjobs.gress.io streamclusters.gress.io checkpoints.gress.io
```

### Cleanup Considerations

1. **Persistent Data**: PVCs for checkpoints and state are not automatically deleted
2. **External Storage**: Clean up S3/GCS checkpoints manually if needed
3. **Monitoring**: Remove ServiceMonitors if using Prometheus Operator

## Troubleshooting

### Common Installation Issues

#### CRD Installation Fails

```bash
# Check CRD status
kubectl get crds | grep gress.io

# View CRD details
kubectl describe crd streamjobs.gress.io
```

#### Operator Pod Not Starting

```bash
# Check pod status
kubectl get pods -n gress-system

# View pod logs
kubectl logs -n gress-system deployment/gress-operator

# Check events
kubectl get events -n gress-system --sort-by='.lastTimestamp'
```

#### RBAC Permission Errors

```bash
# Verify service account
kubectl get sa gress-operator -n gress-system

# Verify cluster role binding
kubectl get clusterrolebinding gress-operator-rolebinding

# Test permissions
kubectl auth can-i create streamjobs --as=system:serviceaccount:gress-system:gress-operator
```

### Getting Help

- GitHub Issues: https://github.com/therealutkarshpriyadarshi/gress/issues
- Documentation: https://docs.gress.io
- Community Slack: https://gress-community.slack.com

## Next Steps

After successful installation:

1. Review the [Usage Guide](README.md#usage)
2. Try the [Quick Start examples](README.md#quick-start)
3. Read about [Configuration options](README.md#configuration)
4. Set up [Monitoring](README.md#monitoring)
5. Plan for [High Availability](README.md#high-availability)
