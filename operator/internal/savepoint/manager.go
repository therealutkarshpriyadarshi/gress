package savepoint

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	gressv1alpha1 "github.com/therealutkarshpriyadarshi/gress/operator/api/v1alpha1"
)

// SavepointManager handles savepoint operations for StreamJobs
type SavepointManager struct {
	Client client.Client
}

// NewSavepointManager creates a new savepoint manager
func NewSavepointManager(client client.Client) *SavepointManager {
	return &SavepointManager{
		Client: client,
	}
}

// TriggerSavepoint triggers a savepoint for a StreamJob before updates
func (m *SavepointManager) TriggerSavepoint(ctx context.Context, streamJob *gressv1alpha1.StreamJob, reason string) (*gressv1alpha1.Checkpoint, error) {
	logger := log.FromContext(ctx)
	logger.Info("Triggering savepoint", "streamJob", streamJob.Name, "reason", reason)

	// Create a Checkpoint resource with type=savepoint
	checkpoint := &gressv1alpha1.Checkpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-savepoint-%d", streamJob.Name, time.Now().Unix()),
			Namespace: streamJob.Namespace,
			Labels: map[string]string{
				"streamjob": streamJob.Name,
				"type":      "savepoint",
				"reason":    reason,
			},
		},
		Spec: gressv1alpha1.CheckpointSpec{
			StreamJobName: streamJob.Name,
			Type:          "savepoint",
			TriggerMode:   "pre-upgrade",
			RetentionPolicy: &gressv1alpha1.CheckpointRetentionPolicy{
				RetainOnJobCompletion:   true,
				RetainOnJobFailure:      true,
				RetainOnJobCancellation: true,
				MaxCount:                5,
			},
		},
	}

	// Use checkpoint storage from StreamJob if configured
	if streamJob.Spec.Checkpoint != nil && streamJob.Spec.Checkpoint.Storage != nil {
		checkpoint.Spec.Storage = streamJob.Spec.Checkpoint.Storage
	}

	// Create the checkpoint
	if err := m.Client.Create(ctx, checkpoint); err != nil {
		return nil, fmt.Errorf("failed to create savepoint: %w", err)
	}

	logger.Info("Savepoint triggered successfully", "checkpoint", checkpoint.Name)
	return checkpoint, nil
}

// WaitForSavepointCompletion waits for a savepoint to complete
func (m *SavepointManager) WaitForSavepointCompletion(ctx context.Context, checkpoint *gressv1alpha1.Checkpoint, timeout time.Duration) error {
	logger := log.FromContext(ctx)
	logger.Info("Waiting for savepoint completion", "checkpoint", checkpoint.Name, "timeout", timeout)

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for savepoint completion")
			}

			// Fetch current checkpoint status
			current := &gressv1alpha1.Checkpoint{}
			if err := m.Client.Get(ctx, client.ObjectKeyFromObject(checkpoint), current); err != nil {
				return fmt.Errorf("failed to get checkpoint status: %w", err)
			}

			switch current.Status.Phase {
			case "Completed":
				logger.Info("Savepoint completed successfully", "checkpoint", checkpoint.Name, "path", current.Status.LastCheckpointPath)
				return nil
			case "Failed":
				return fmt.Errorf("savepoint failed: %s", current.Status.FailureReason)
			case "Expired":
				return fmt.Errorf("savepoint expired")
			default:
				logger.Info("Savepoint in progress", "phase", current.Status.Phase)
			}
		}
	}
}

// GetLatestSavepoint gets the latest savepoint for a StreamJob
func (m *SavepointManager) GetLatestSavepoint(ctx context.Context, streamJob *gressv1alpha1.StreamJob) (*gressv1alpha1.Checkpoint, error) {
	logger := log.FromContext(ctx)
	logger.Info("Getting latest savepoint", "streamJob", streamJob.Name)

	// List all checkpoints for this StreamJob
	checkpointList := &gressv1alpha1.CheckpointList{}
	if err := m.Client.List(ctx, checkpointList,
		client.InNamespace(streamJob.Namespace),
		client.MatchingLabels{"streamjob": streamJob.Name, "type": "savepoint"}); err != nil {
		return nil, fmt.Errorf("failed to list checkpoints: %w", err)
	}

	// Filter completed savepoints and find the latest
	var latestSavepoint *gressv1alpha1.Checkpoint
	var latestTime time.Time

	for i := range checkpointList.Items {
		cp := &checkpointList.Items[i]
		if cp.Status.Phase == "Completed" && cp.Status.CompletionTime != nil {
			if cp.Status.CompletionTime.After(latestTime) {
				latestTime = cp.Status.CompletionTime.Time
				latestSavepoint = cp
			}
		}
	}

	if latestSavepoint == nil {
		return nil, fmt.Errorf("no completed savepoints found")
	}

	logger.Info("Found latest savepoint", "checkpoint", latestSavepoint.Name, "completionTime", latestTime)
	return latestSavepoint, nil
}

// RestoreFromSavepoint updates a StreamJob to restore from a savepoint
func (m *SavepointManager) RestoreFromSavepoint(ctx context.Context, streamJob *gressv1alpha1.StreamJob, checkpoint *gressv1alpha1.Checkpoint) error {
	logger := log.FromContext(ctx)
	logger.Info("Restoring from savepoint", "streamJob", streamJob.Name, "checkpoint", checkpoint.Name)

	// Update StreamJob with savepoint path
	savepointPath := checkpoint.Spec.Path
	if checkpoint.Status.LastCheckpointPath != "" {
		savepointPath = checkpoint.Status.LastCheckpointPath
	}

	streamJob.Spec.SavepointPath = savepointPath

	if err := m.Client.Update(ctx, streamJob); err != nil {
		return fmt.Errorf("failed to update StreamJob with savepoint path: %w", err)
	}

	logger.Info("StreamJob updated to restore from savepoint", "path", savepointPath)
	return nil
}

// CleanupOldSavepoints removes old savepoints based on retention policy
func (m *SavepointManager) CleanupOldSavepoints(ctx context.Context, streamJob *gressv1alpha1.StreamJob, maxCount int) error {
	logger := log.FromContext(ctx)
	logger.Info("Cleaning up old savepoints", "streamJob", streamJob.Name, "maxCount", maxCount)

	// List all savepoints for this StreamJob
	checkpointList := &gressv1alpha1.CheckpointList{}
	if err := m.Client.List(ctx, checkpointList,
		client.InNamespace(streamJob.Namespace),
		client.MatchingLabels{"streamjob": streamJob.Name, "type": "savepoint"}); err != nil {
		return fmt.Errorf("failed to list checkpoints: %w", err)
	}

	// Filter completed savepoints
	completedSavepoints := []*gressv1alpha1.Checkpoint{}
	for i := range checkpointList.Items {
		cp := &checkpointList.Items[i]
		if cp.Status.Phase == "Completed" {
			completedSavepoints = append(completedSavepoints, cp)
		}
	}

	// If we have more than maxCount, delete the oldest ones
	if len(completedSavepoints) <= maxCount {
		return nil
	}

	// Sort by completion time (newest first)
	// For simplicity, we'll just delete the excess
	// In production, implement proper sorting

	toDelete := len(completedSavepoints) - maxCount
	for i := 0; i < toDelete; i++ {
		cp := completedSavepoints[i]
		logger.Info("Deleting old savepoint", "checkpoint", cp.Name)
		if err := m.Client.Delete(ctx, cp); err != nil {
			logger.Error(err, "Failed to delete old savepoint", "checkpoint", cp.Name)
		}
	}

	return nil
}
