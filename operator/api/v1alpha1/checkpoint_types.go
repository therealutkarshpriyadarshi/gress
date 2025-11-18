package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CheckpointSpec defines the desired state of Checkpoint
type CheckpointSpec struct {
	// StreamJobName is the name of the associated StreamJob
	// +kubebuilder:validation:Required
	StreamJobName string `json:"streamJobName"`

	// Type of checkpoint (checkpoint, savepoint)
	// +kubebuilder:validation:Enum=checkpoint;savepoint
	// +kubebuilder:default=checkpoint
	Type string `json:"type"`

	// TriggerMode defines how the checkpoint was triggered (periodic, manual, pre-upgrade)
	// +kubebuilder:validation:Enum=periodic;manual;pre-upgrade;on-cancel
	TriggerMode string `json:"triggerMode"`

	// Path where the checkpoint data is stored
	// +optional
	Path string `json:"path,omitempty"`

	// Storage defines where the checkpoint is stored
	// +optional
	Storage *CheckpointStorageSpec `json:"storage,omitempty"`

	// RetentionPolicy defines how long to keep this checkpoint
	// +optional
	RetentionPolicy *CheckpointRetentionPolicy `json:"retentionPolicy,omitempty"`

	// Metadata contains additional checkpoint metadata
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`
}

// CheckpointRetentionPolicy defines checkpoint retention configuration
type CheckpointRetentionPolicy struct {
	// RetainOnJobCompletion retains checkpoint after job completion
	// +kubebuilder:default=false
	RetainOnJobCompletion bool `json:"retainOnJobCompletion,omitempty"`

	// RetainOnJobFailure retains checkpoint after job failure
	// +kubebuilder:default=true
	RetainOnJobFailure bool `json:"retainOnJobFailure,omitempty"`

	// RetainOnJobCancellation retains checkpoint after job cancellation
	// +kubebuilder:default=true
	RetainOnJobCancellation bool `json:"retainOnJobCancellation,omitempty"`

	// MaxCount is the maximum number of checkpoints to retain (0 = unlimited)
	// +kubebuilder:default=3
	MaxCount int32 `json:"maxCount,omitempty"`

	// MaxAge is the maximum age for checkpoints (e.g., "7d", "24h")
	// +optional
	MaxAge string `json:"maxAge,omitempty"`
}

// CheckpointStatus defines the observed state of Checkpoint
type CheckpointStatus struct {
	// Phase represents the current phase of the checkpoint
	// +kubebuilder:validation:Enum=Pending;InProgress;Completed;Failed;Expired
	Phase string `json:"phase,omitempty"`

	// Conditions represent the latest available observations of the checkpoint's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// StartTime is when the checkpoint started
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is when the checkpoint completed
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Duration of the checkpoint operation
	// +optional
	Duration string `json:"duration,omitempty"`

	// Size of the checkpoint in bytes
	// +optional
	Size int64 `json:"size,omitempty"`

	// StateSize is the size of the state in bytes
	// +optional
	StateSize int64 `json:"stateSize,omitempty"`

	// CheckpointID is the unique identifier for this checkpoint
	// +optional
	CheckpointID int64 `json:"checkpointID,omitempty"`

	// AlignmentDuration is the time spent aligning buffers
	// +optional
	AlignmentDuration string `json:"alignmentDuration,omitempty"`

	// TaskCheckpoints shows per-task checkpoint status
	// +optional
	TaskCheckpoints []TaskCheckpointStatus `json:"taskCheckpoints,omitempty"`

	// ExpirationTime is when this checkpoint will expire
	// +optional
	ExpirationTime *metav1.Time `json:"expirationTime,omitempty"`

	// Message provides details about the checkpoint status
	// +optional
	Message string `json:"message,omitempty"`

	// FailureReason provides the reason if checkpoint failed
	// +optional
	FailureReason string `json:"failureReason,omitempty"`
}

// TaskCheckpointStatus represents the checkpoint status for a single task
type TaskCheckpointStatus struct {
	// TaskIndex is the index of the task
	TaskIndex int32 `json:"taskIndex"`

	// State of the task checkpoint (pending, completed, failed)
	// +kubebuilder:validation:Enum=pending;completed;failed
	State string `json:"state"`

	// Size of the task's state in bytes
	// +optional
	Size int64 `json:"size,omitempty"`

	// Duration for this task's checkpoint
	// +optional
	Duration string `json:"duration,omitempty"`

	// Message provides details
	// +optional
	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Job",type=string,JSONPath=`.spec.streamJobName`
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Size",type=integer,JSONPath=`.status.size`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:resource:shortName=cp

// Checkpoint is the Schema for the checkpoints API
type Checkpoint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CheckpointSpec   `json:"spec,omitempty"`
	Status CheckpointStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CheckpointList contains a list of Checkpoint
type CheckpointList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Checkpoint `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Checkpoint{}, &CheckpointList{})
}
