package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StreamJobSpec defines the desired state of StreamJob
type StreamJobSpec struct {
	// Image is the container image for the stream processing job
	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// ImagePullPolicy describes a policy for if/when to pull a container image
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// ImagePullSecrets is an optional list of references to secrets in the same namespace to use for pulling any of the images
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// Parallelism defines the number of parallel task instances
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	Parallelism int32 `json:"parallelism,omitempty"`

	// RestartPolicy defines the restart behavior of individual task instances
	// +kubebuilder:validation:Enum=Never;OnFailure;Always
	// +kubebuilder:default=OnFailure
	RestartPolicy string `json:"restartPolicy,omitempty"`

	// Resources defines the compute resources required by the job
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// StateBackend configures the state storage backend
	// +optional
	StateBackend *StateBackendSpec `json:"stateBackend,omitempty"`

	// Checkpoint defines checkpointing configuration
	// +optional
	Checkpoint *CheckpointSpec `json:"checkpoint,omitempty"`

	// Sources defines the data sources for the stream job
	// +optional
	Sources []SourceSpec `json:"sources,omitempty"`

	// Sinks defines the data sinks for the stream job
	// +optional
	Sinks []SinkSpec `json:"sinks,omitempty"`

	// Pipeline defines the stream processing operators
	// +optional
	Pipeline []OperatorSpec `json:"pipeline,omitempty"`

	// Config is a reference to a ConfigMap containing job configuration
	// +optional
	ConfigRef *corev1.LocalObjectReference `json:"configRef,omitempty"`

	// Env defines environment variables for the job
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Volumes defines additional volumes to mount
	// +optional
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// VolumeMounts defines additional volume mounts
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// ServiceAccountName is the name of the ServiceAccount to use to run this job
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// AutoScaler defines auto-scaling configuration
	// +optional
	AutoScaler *AutoScalerSpec `json:"autoScaler,omitempty"`

	// SavepointPath is the path to restore from a savepoint
	// +optional
	SavepointPath string `json:"savepointPath,omitempty"`

	// Affinity defines pod affinity rules
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Tolerations defines pod tolerations
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// NodeSelector defines node selection constraints
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
}

// StateBackendSpec defines the state storage configuration
type StateBackendSpec struct {
	// Type is the state backend type (rocksdb, memory)
	// +kubebuilder:validation:Enum=rocksdb;memory
	// +kubebuilder:default=rocksdb
	Type string `json:"type"`

	// RocksDB specific configuration
	// +optional
	RocksDB *RocksDBSpec `json:"rocksdb,omitempty"`

	// PersistentVolumeClaim for state storage
	// +optional
	PersistentVolumeClaim *corev1.PersistentVolumeClaimSpec `json:"persistentVolumeClaim,omitempty"`
}

// RocksDBSpec defines RocksDB-specific configuration
type RocksDBSpec struct {
	// WriteBufferSize in bytes
	// +optional
	WriteBufferSize int64 `json:"writeBufferSize,omitempty"`

	// BlockCacheSize in bytes
	// +optional
	BlockCacheSize int64 `json:"blockCacheSize,omitempty"`

	// EnableStatistics enables RocksDB statistics
	// +optional
	EnableStatistics bool `json:"enableStatistics,omitempty"`
}

// CheckpointSpec defines checkpointing configuration
type CheckpointSpec struct {
	// Interval is the checkpoint interval duration (e.g., "30s", "5m")
	// +kubebuilder:default="30s"
	Interval string `json:"interval,omitempty"`

	// Mode defines the checkpoint mode (exactly-once, at-least-once)
	// +kubebuilder:validation:Enum=exactly-once;at-least-once
	// +kubebuilder:default=exactly-once
	Mode string `json:"mode,omitempty"`

	// Timeout for checkpoint completion
	// +optional
	Timeout string `json:"timeout,omitempty"`

	// MinPauseBetweenCheckpoints minimum pause between checkpoints
	// +optional
	MinPauseBetweenCheckpoints string `json:"minPauseBetweenCheckpoints,omitempty"`

	// MaxConcurrentCheckpoints maximum number of concurrent checkpoints
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	MaxConcurrentCheckpoints int32 `json:"maxConcurrentCheckpoints,omitempty"`

	// Storage defines where checkpoints are stored
	// +optional
	Storage *CheckpointStorageSpec `json:"storage,omitempty"`
}

// CheckpointStorageSpec defines checkpoint storage configuration
type CheckpointStorageSpec struct {
	// Type is the storage type (filesystem, s3, gcs)
	// +kubebuilder:validation:Enum=filesystem;s3;gcs
	// +kubebuilder:default=filesystem
	Type string `json:"type"`

	// Path is the base path for checkpoints
	// +optional
	Path string `json:"path,omitempty"`

	// PersistentVolumeClaim for filesystem storage
	// +optional
	PersistentVolumeClaim *corev1.PersistentVolumeClaimSpec `json:"persistentVolumeClaim,omitempty"`

	// S3 specific configuration
	// +optional
	S3 *S3StorageSpec `json:"s3,omitempty"`

	// GCS specific configuration
	// +optional
	GCS *GCSStorageSpec `json:"gcs,omitempty"`
}

// S3StorageSpec defines S3 storage configuration
type S3StorageSpec struct {
	// Bucket name
	// +kubebuilder:validation:Required
	Bucket string `json:"bucket"`

	// Prefix for checkpoint objects
	// +optional
	Prefix string `json:"prefix,omitempty"`

	// Region
	// +optional
	Region string `json:"region,omitempty"`

	// Endpoint for S3-compatible storage
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// CredentialsSecret reference to AWS credentials
	// +optional
	CredentialsSecret *corev1.LocalObjectReference `json:"credentialsSecret,omitempty"`
}

// GCSStorageSpec defines GCS storage configuration
type GCSStorageSpec struct {
	// Bucket name
	// +kubebuilder:validation:Required
	Bucket string `json:"bucket"`

	// Prefix for checkpoint objects
	// +optional
	Prefix string `json:"prefix,omitempty"`

	// CredentialsSecret reference to GCS credentials
	// +optional
	CredentialsSecret *corev1.LocalObjectReference `json:"credentialsSecret,omitempty"`
}

// SourceSpec defines a data source
type SourceSpec struct {
	// Name of the source
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Type of the source (kafka, http, websocket, nats)
	// +kubebuilder:validation:Enum=kafka;http;websocket;nats;file
	Type string `json:"type"`

	// Kafka specific configuration
	// +optional
	Kafka *KafkaSourceSpec `json:"kafka,omitempty"`

	// HTTP specific configuration
	// +optional
	HTTP *HTTPSourceSpec `json:"http,omitempty"`

	// WebSocket specific configuration
	// +optional
	WebSocket *WebSocketSourceSpec `json:"websocket,omitempty"`

	// NATS specific configuration
	// +optional
	NATS *NATSSourceSpec `json:"nats,omitempty"`
}

// KafkaSourceSpec defines Kafka source configuration
type KafkaSourceSpec struct {
	// Brokers list
	// +kubebuilder:validation:Required
	Brokers []string `json:"brokers"`

	// Topics to consume from
	// +kubebuilder:validation:Required
	Topics []string `json:"topics"`

	// GroupID for consumer group
	// +kubebuilder:validation:Required
	GroupID string `json:"groupID"`

	// StartOffset (earliest, latest)
	// +kubebuilder:validation:Enum=earliest;latest
	// +kubebuilder:default=latest
	StartOffset string `json:"startOffset,omitempty"`

	// TLS configuration
	// +optional
	TLS *TLSSpec `json:"tls,omitempty"`

	// SASL configuration
	// +optional
	SASL *SASLSpec `json:"sasl,omitempty"`
}

// HTTPSourceSpec defines HTTP source configuration
type HTTPSourceSpec struct {
	// Port to listen on
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`

	// Path for the endpoint
	// +kubebuilder:default="/"
	Path string `json:"path,omitempty"`
}

// WebSocketSourceSpec defines WebSocket source configuration
type WebSocketSourceSpec struct {
	// Port to listen on
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`

	// Path for the endpoint
	// +kubebuilder:default="/"
	Path string `json:"path,omitempty"`
}

// NATSSourceSpec defines NATS source configuration
type NATSSourceSpec struct {
	// URL of NATS server
	// +kubebuilder:validation:Required
	URL string `json:"url"`

	// Subject to subscribe to
	// +kubebuilder:validation:Required
	Subject string `json:"subject"`

	// DurableName for durable subscriptions
	// +optional
	DurableName string `json:"durableName,omitempty"`

	// CredentialsSecret reference to NATS credentials
	// +optional
	CredentialsSecret *corev1.LocalObjectReference `json:"credentialsSecret,omitempty"`
}

// SinkSpec defines a data sink
type SinkSpec struct {
	// Name of the sink
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Type of the sink (kafka, timescaledb, console, http)
	// +kubebuilder:validation:Enum=kafka;timescaledb;console;http;file
	Type string `json:"type"`

	// Kafka specific configuration
	// +optional
	Kafka *KafkaSinkSpec `json:"kafka,omitempty"`

	// TimescaleDB specific configuration
	// +optional
	TimescaleDB *TimescaleDBSinkSpec `json:"timescaledb,omitempty"`

	// HTTP specific configuration
	// +optional
	HTTP *HTTPSinkSpec `json:"http,omitempty"`
}

// KafkaSinkSpec defines Kafka sink configuration
type KafkaSinkSpec struct {
	// Brokers list
	// +kubebuilder:validation:Required
	Brokers []string `json:"brokers"`

	// Topic to produce to
	// +kubebuilder:validation:Required
	Topic string `json:"topic"`

	// TLS configuration
	// +optional
	TLS *TLSSpec `json:"tls,omitempty"`

	// SASL configuration
	// +optional
	SASL *SASLSpec `json:"sasl,omitempty"`
}

// TimescaleDBSinkSpec defines TimescaleDB sink configuration
type TimescaleDBSinkSpec struct {
	// ConnectionString or reference to secret
	// +optional
	ConnectionString string `json:"connectionString,omitempty"`

	// ConnectionStringSecret reference
	// +optional
	ConnectionStringSecret *corev1.LocalObjectReference `json:"connectionStringSecret,omitempty"`

	// Table name
	// +kubebuilder:validation:Required
	Table string `json:"table"`

	// BatchSize for bulk inserts
	// +optional
	BatchSize int32 `json:"batchSize,omitempty"`
}

// HTTPSinkSpec defines HTTP sink configuration
type HTTPSinkSpec struct {
	// URL to send events to
	// +kubebuilder:validation:Required
	URL string `json:"url"`

	// Method (POST, PUT)
	// +kubebuilder:validation:Enum=POST;PUT
	// +kubebuilder:default=POST
	Method string `json:"method,omitempty"`

	// Headers to include
	// +optional
	Headers map[string]string `json:"headers,omitempty"`
}

// TLSSpec defines TLS configuration
type TLSSpec struct {
	// Enabled flag
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// CASecret reference to CA certificate
	// +optional
	CASecret *corev1.LocalObjectReference `json:"caSecret,omitempty"`

	// CertSecret reference to client certificate
	// +optional
	CertSecret *corev1.LocalObjectReference `json:"certSecret,omitempty"`

	// InsecureSkipVerify skips certificate verification
	// +optional
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}

// SASLSpec defines SASL configuration
type SASLSpec struct {
	// Mechanism (PLAIN, SCRAM-SHA-256, SCRAM-SHA-512)
	// +kubebuilder:validation:Enum=PLAIN;SCRAM-SHA-256;SCRAM-SHA-512
	Mechanism string `json:"mechanism"`

	// CredentialsSecret reference to SASL credentials
	// +kubebuilder:validation:Required
	CredentialsSecret corev1.LocalObjectReference `json:"credentialsSecret"`
}

// OperatorSpec defines a stream processing operator
type OperatorSpec struct {
	// Name of the operator
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Type of operator (map, filter, flatmap, keyby, aggregate, window)
	// +kubebuilder:validation:Enum=map;filter;flatmap;keyby;aggregate;reduce;window;join
	Type string `json:"type"`

	// Function is the transformation function (for map, filter, etc.)
	// +optional
	Function string `json:"function,omitempty"`

	// KeySelector for keyby operations
	// +optional
	KeySelector string `json:"keySelector,omitempty"`

	// Window configuration for windowing operations
	// +optional
	Window *WindowSpec `json:"window,omitempty"`

	// Join configuration for join operations
	// +optional
	Join *JoinSpec `json:"join,omitempty"`
}

// WindowSpec defines windowing configuration
type WindowSpec struct {
	// Type of window (tumbling, sliding, session)
	// +kubebuilder:validation:Enum=tumbling;sliding;session
	Type string `json:"type"`

	// Size of the window
	// +kubebuilder:validation:Required
	Size string `json:"size"`

	// Slide for sliding windows
	// +optional
	Slide string `json:"slide,omitempty"`

	// Gap for session windows
	// +optional
	Gap string `json:"gap,omitempty"`
}

// JoinSpec defines join configuration
type JoinSpec struct {
	// Type of join (inner, left, right, full)
	// +kubebuilder:validation:Enum=inner;left;right;full
	Type string `json:"type"`

	// RightStream to join with
	// +kubebuilder:validation:Required
	RightStream string `json:"rightStream"`

	// LeftKey for join
	// +kubebuilder:validation:Required
	LeftKey string `json:"leftKey"`

	// RightKey for join
	// +kubebuilder:validation:Required
	RightKey string `json:"rightKey"`

	// Window for windowed joins
	// +optional
	Window *WindowSpec `json:"window,omitempty"`
}

// AutoScalerSpec defines auto-scaling configuration
type AutoScalerSpec struct {
	// Enabled flag
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`

	// MinReplicas minimum number of replicas
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	MinReplicas int32 `json:"minReplicas,omitempty"`

	// MaxReplicas maximum number of replicas
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=10
	MaxReplicas int32 `json:"maxReplicas,omitempty"`

	// Metrics to use for scaling
	// +optional
	Metrics []ScalingMetric `json:"metrics,omitempty"`

	// Behavior defines scaling behavior
	// +optional
	Behavior *ScalingBehavior `json:"behavior,omitempty"`
}

// ScalingMetric defines a metric for auto-scaling
type ScalingMetric struct {
	// Type of metric (cpu, memory, kafka_lag, custom)
	// +kubebuilder:validation:Enum=cpu;memory;kafka_lag;custom
	Type string `json:"type"`

	// Target value for the metric
	// +optional
	Target string `json:"target,omitempty"`

	// Custom metric query (for Prometheus)
	// +optional
	CustomQuery string `json:"customQuery,omitempty"`
}

// ScalingBehavior defines scaling behavior policies
type ScalingBehavior struct {
	// ScaleUp defines scale-up behavior
	// +optional
	ScaleUp *ScalingPolicy `json:"scaleUp,omitempty"`

	// ScaleDown defines scale-down behavior
	// +optional
	ScaleDown *ScalingPolicy `json:"scaleDown,omitempty"`
}

// ScalingPolicy defines a scaling policy
type ScalingPolicy struct {
	// StabilizationWindowSeconds is the time window for stabilization
	// +optional
	StabilizationWindowSeconds int32 `json:"stabilizationWindowSeconds,omitempty"`

	// Policies is a list of scaling policies
	// +optional
	Policies []ScalingPolicyRule `json:"policies,omitempty"`
}

// ScalingPolicyRule defines a single scaling policy rule
type ScalingPolicyRule struct {
	// Type of policy (Pods, Percent)
	// +kubebuilder:validation:Enum=Pods;Percent
	Type string `json:"type"`

	// Value is the amount of change
	// +kubebuilder:validation:Minimum=1
	Value int32 `json:"value"`

	// PeriodSeconds is the time period
	// +kubebuilder:validation:Minimum=1
	PeriodSeconds int32 `json:"periodSeconds"`
}

// StreamJobStatus defines the observed state of StreamJob
type StreamJobStatus struct {
	// Phase represents the current phase of the job
	// +kubebuilder:validation:Enum=Pending;Running;Succeeded;Failed;Suspended
	Phase string `json:"phase,omitempty"`

	// Conditions represent the latest available observations of the job's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// CurrentReplicas is the current number of replicas
	// +optional
	CurrentReplicas int32 `json:"currentReplicas,omitempty"`

	// ReadyReplicas is the number of ready replicas
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// LastCheckpointTime is the time of the last successful checkpoint
	// +optional
	LastCheckpointTime *metav1.Time `json:"lastCheckpointTime,omitempty"`

	// LastCheckpointPath is the path to the last checkpoint
	// +optional
	LastCheckpointPath string `json:"lastCheckpointPath,omitempty"`

	// LastSavepointTime is the time of the last savepoint
	// +optional
	LastSavepointTime *metav1.Time `json:"lastSavepointTime,omitempty"`

	// LastSavepointPath is the path to the last savepoint
	// +optional
	LastSavepointPath string `json:"lastSavepointPath,omitempty"`

	// ObservedGeneration reflects the generation of the most recently observed spec
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// StartTime is when the job started
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is when the job completed
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// TaskStatuses shows status of individual task instances
	// +optional
	TaskStatuses []TaskStatus `json:"taskStatuses,omitempty"`
}

// TaskStatus represents the status of an individual task instance
type TaskStatus struct {
	// Index of the task
	Index int32 `json:"index"`

	// State of the task (pending, running, succeeded, failed)
	State string `json:"state"`

	// PodName associated with the task
	// +optional
	PodName string `json:"podName,omitempty"`

	// LastTransitionTime is when the state last changed
	// +optional
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`

	// Message provides details about the task state
	// +optional
	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.parallelism,statuspath=.status.currentReplicas,selectorpath=.status.selector
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Replicas",type=integer,JSONPath=`.status.currentReplicas`
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.readyReplicas`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:resource:shortName=sj

// StreamJob is the Schema for the streamjobs API
type StreamJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StreamJobSpec   `json:"spec,omitempty"`
	Status StreamJobStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// StreamJobList contains a list of StreamJob
type StreamJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StreamJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&StreamJob{}, &StreamJobList{})
}
