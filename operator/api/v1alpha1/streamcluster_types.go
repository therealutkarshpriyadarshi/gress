package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StreamClusterSpec defines the desired state of StreamCluster
type StreamClusterSpec struct {
	// Version of the gress image to use
	// +kubebuilder:validation:Required
	Version string `json:"version"`

	// Image is the base container image for the cluster
	// +optional
	Image string `json:"image,omitempty"`

	// ImagePullPolicy describes a policy for if/when to pull a container image
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// ImagePullSecrets is an optional list of references to secrets
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// TaskManagers defines the task manager configuration
	// +optional
	TaskManagers *TaskManagerSpec `json:"taskManagers,omitempty"`

	// JobManagers defines the job manager configuration (for HA setup)
	// +optional
	JobManagers *JobManagerSpec `json:"jobManagers,omitempty"`

	// HighAvailability defines HA configuration
	// +optional
	HighAvailability *HighAvailabilitySpec `json:"highAvailability,omitempty"`

	// StateBackend configures the default state storage backend for the cluster
	// +optional
	StateBackend *StateBackendSpec `json:"stateBackend,omitempty"`

	// Checkpoint defines default checkpointing configuration
	// +optional
	Checkpoint *CheckpointSpec `json:"checkpoint,omitempty"`

	// ServiceAccountName is the name of the ServiceAccount to use
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// PodSecurityContext defines security context for pods
	// +optional
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`

	// SecurityContext defines security context for containers
	// +optional
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty"`

	// Monitoring defines monitoring configuration
	// +optional
	Monitoring *MonitoringSpec `json:"monitoring,omitempty"`

	// Logging defines logging configuration
	// +optional
	Logging *LoggingSpec `json:"logging,omitempty"`

	// NetworkPolicy defines network policy settings
	// +optional
	NetworkPolicy *NetworkPolicySpec `json:"networkPolicy,omitempty"`
}

// TaskManagerSpec defines task manager configuration
type TaskManagerSpec struct {
	// Replicas is the number of task manager replicas
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas"`

	// Resources defines the compute resources
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Storage defines storage requirements
	// +optional
	Storage *StorageSpec `json:"storage,omitempty"`

	// Affinity defines pod affinity rules
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Tolerations defines pod tolerations
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// NodeSelector defines node selection constraints
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Env defines environment variables
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// TaskSlots defines the number of task slots per task manager
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	TaskSlots int32 `json:"taskSlots,omitempty"`
}

// JobManagerSpec defines job manager configuration
type JobManagerSpec struct {
	// Replicas is the number of job manager replicas (typically 1 or 3 for HA)
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas"`

	// Resources defines the compute resources
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Storage defines storage requirements for job manager
	// +optional
	Storage *StorageSpec `json:"storage,omitempty"`

	// Affinity defines pod affinity rules
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Tolerations defines pod tolerations
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// NodeSelector defines node selection constraints
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Env defines environment variables
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// ServiceType defines the service type for job manager (ClusterIP, NodePort, LoadBalancer)
	// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
	// +kubebuilder:default=ClusterIP
	ServiceType corev1.ServiceType `json:"serviceType,omitempty"`

	// Ingress defines ingress configuration for job manager UI
	// +optional
	Ingress *IngressSpec `json:"ingress,omitempty"`
}

// StorageSpec defines storage configuration
type StorageSpec struct {
	// Size is the storage size
	// +kubebuilder:validation:Required
	Size string `json:"size"`

	// StorageClassName for dynamic provisioning
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`

	// VolumeMode defines whether volume is Filesystem or Block
	// +optional
	VolumeMode *corev1.PersistentVolumeMode `json:"volumeMode,omitempty"`

	// AccessModes defines access modes
	// +optional
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`
}

// IngressSpec defines ingress configuration
type IngressSpec struct {
	// Enabled flag
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Annotations for the ingress
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// IngressClassName for the ingress
	// +optional
	IngressClassName *string `json:"ingressClassName,omitempty"`

	// Host for the ingress
	// +optional
	Host string `json:"host,omitempty"`

	// TLS configuration
	// +optional
	TLS []IngressTLSSpec `json:"tls,omitempty"`
}

// IngressTLSSpec defines TLS configuration for ingress
type IngressTLSSpec struct {
	// Hosts covered by the certificate
	// +optional
	Hosts []string `json:"hosts,omitempty"`

	// SecretName containing the certificate
	// +optional
	SecretName string `json:"secretName,omitempty"`
}

// HighAvailabilitySpec defines high availability configuration
type HighAvailabilitySpec struct {
	// Enabled flag
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`

	// Mode defines the HA mode (kubernetes, zookeeper)
	// +kubebuilder:validation:Enum=kubernetes;zookeeper
	// +kubebuilder:default=kubernetes
	Mode string `json:"mode,omitempty"`

	// Zookeeper configuration for ZooKeeper-based HA
	// +optional
	Zookeeper *ZookeeperSpec `json:"zookeeper,omitempty"`

	// StorageDir for HA metadata
	// +optional
	StorageDir string `json:"storageDir,omitempty"`
}

// ZookeeperSpec defines ZooKeeper configuration
type ZookeeperSpec struct {
	// Quorum connection string
	// +kubebuilder:validation:Required
	Quorum string `json:"quorum"`

	// RootPath for ZooKeeper nodes
	// +kubebuilder:default="/gress"
	RootPath string `json:"rootPath,omitempty"`
}

// MonitoringSpec defines monitoring configuration
type MonitoringSpec struct {
	// Enabled flag
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`

	// Port for metrics endpoint
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=9091
	Port int32 `json:"port,omitempty"`

	// ServiceMonitor defines Prometheus Operator ServiceMonitor configuration
	// +optional
	ServiceMonitor *ServiceMonitorSpec `json:"serviceMonitor,omitempty"`
}

// ServiceMonitorSpec defines ServiceMonitor configuration
type ServiceMonitorSpec struct {
	// Enabled flag for creating ServiceMonitor
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Interval for scraping metrics
	// +kubebuilder:default="30s"
	Interval string `json:"interval,omitempty"`

	// ScrapeTimeout for metrics scraping
	// +optional
	ScrapeTimeout string `json:"scrapeTimeout,omitempty"`

	// Labels to add to ServiceMonitor
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// LoggingSpec defines logging configuration
type LoggingSpec struct {
	// Level is the log level (debug, info, warn, error)
	// +kubebuilder:validation:Enum=debug;info;warn;error
	// +kubebuilder:default=info
	Level string `json:"level,omitempty"`

	// Format is the log format (json, text)
	// +kubebuilder:validation:Enum=json;text
	// +kubebuilder:default=json
	Format string `json:"format,omitempty"`

	// LogAggregation defines log aggregation configuration
	// +optional
	LogAggregation *LogAggregationSpec `json:"logAggregation,omitempty"`
}

// LogAggregationSpec defines log aggregation configuration
type LogAggregationSpec struct {
	// Type of log aggregation (stdout, fluentd, elasticsearch)
	// +kubebuilder:validation:Enum=stdout;fluentd;elasticsearch
	// +kubebuilder:default=stdout
	Type string `json:"type,omitempty"`

	// FluentdConfig for fluentd integration
	// +optional
	FluentdConfig map[string]string `json:"fluentdConfig,omitempty"`

	// ElasticsearchConfig for Elasticsearch integration
	// +optional
	ElasticsearchConfig map[string]string `json:"elasticsearchConfig,omitempty"`
}

// NetworkPolicySpec defines network policy configuration
type NetworkPolicySpec struct {
	// Enabled flag
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// PodSelector for network policy
	// +optional
	PodSelector *metav1.LabelSelector `json:"podSelector,omitempty"`

	// Ingress rules
	// +optional
	Ingress []NetworkPolicyIngressRule `json:"ingress,omitempty"`

	// Egress rules
	// +optional
	Egress []NetworkPolicyEgressRule `json:"egress,omitempty"`
}

// NetworkPolicyIngressRule defines ingress network policy rule
type NetworkPolicyIngressRule struct {
	// From sources
	// +optional
	From []NetworkPolicyPeer `json:"from,omitempty"`

	// Ports allowed
	// +optional
	Ports []NetworkPolicyPort `json:"ports,omitempty"`
}

// NetworkPolicyEgressRule defines egress network policy rule
type NetworkPolicyEgressRule struct {
	// To destinations
	// +optional
	To []NetworkPolicyPeer `json:"to,omitempty"`

	// Ports allowed
	// +optional
	Ports []NetworkPolicyPort `json:"ports,omitempty"`
}

// NetworkPolicyPeer defines a peer for network policy
type NetworkPolicyPeer struct {
	// PodSelector selects pods
	// +optional
	PodSelector *metav1.LabelSelector `json:"podSelector,omitempty"`

	// NamespaceSelector selects namespaces
	// +optional
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`
}

// NetworkPolicyPort defines a port for network policy
type NetworkPolicyPort struct {
	// Protocol (TCP, UDP, SCTP)
	// +optional
	Protocol *corev1.Protocol `json:"protocol,omitempty"`

	// Port number or name
	// +optional
	Port *int32 `json:"port,omitempty"`
}

// StreamClusterStatus defines the observed state of StreamCluster
type StreamClusterStatus struct {
	// Phase represents the current phase of the cluster
	// +kubebuilder:validation:Enum=Pending;Running;Failed;Upgrading
	Phase string `json:"phase,omitempty"`

	// Conditions represent the latest available observations of the cluster's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// TaskManagerReplicas is the current number of task manager replicas
	// +optional
	TaskManagerReplicas int32 `json:"taskManagerReplicas,omitempty"`

	// TaskManagerReadyReplicas is the number of ready task manager replicas
	// +optional
	TaskManagerReadyReplicas int32 `json:"taskManagerReadyReplicas,omitempty"`

	// JobManagerReplicas is the current number of job manager replicas
	// +optional
	JobManagerReplicas int32 `json:"jobManagerReplicas,omitempty"`

	// JobManagerReadyReplicas is the number of ready job manager replicas
	// +optional
	JobManagerReadyReplicas int32 `json:"jobManagerReadyReplicas,omitempty"`

	// ObservedGeneration reflects the generation of the most recently observed spec
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// ClusterEndpoint is the endpoint for accessing the cluster
	// +optional
	ClusterEndpoint string `json:"clusterEndpoint,omitempty"`

	// MetricsEndpoint is the endpoint for accessing metrics
	// +optional
	MetricsEndpoint string `json:"metricsEndpoint,omitempty"`

	// LastUpdateTime is when the cluster was last updated
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="TaskManagers",type=integer,JSONPath=`.status.taskManagerReplicas`
// +kubebuilder:printcolumn:name="JobManagers",type=integer,JSONPath=`.status.jobManagerReplicas`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:resource:shortName=sc

// StreamCluster is the Schema for the streamclusters API
type StreamCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StreamClusterSpec   `json:"spec,omitempty"`
	Status StreamClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// StreamClusterList contains a list of StreamCluster
type StreamClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StreamCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&StreamCluster{}, &StreamClusterList{})
}
