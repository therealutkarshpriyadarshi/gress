package controllers

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	gressv1alpha1 "github.com/therealutkarshpriyadarshi/gress/operator/api/v1alpha1"
)

const (
	checkpointFinalizer = "gress.io/checkpoint-finalizer"

	// Checkpoint phases
	CheckpointPhasePending    = "Pending"
	CheckpointPhaseInProgress = "InProgress"
	CheckpointPhaseCompleted  = "Completed"
	CheckpointPhaseFailed     = "Failed"
	CheckpointPhaseExpired    = "Expired"
)

// CheckpointReconciler reconciles a Checkpoint object
type CheckpointReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=gress.io,resources=checkpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gress.io,resources=checkpoints/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gress.io,resources=checkpoints/finalizers,verbs=update
// +kubebuilder:rbac:groups=gress.io,resources=streamjobs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=pods/exec,verbs=create

// Reconcile is part of the main kubernetes reconciliation loop
func (r *CheckpointReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling Checkpoint", "name", req.Name, "namespace", req.Namespace)

	// Fetch the Checkpoint instance
	checkpoint := &gressv1alpha1.Checkpoint{}
	if err := r.Get(ctx, req.NamespacedName, checkpoint); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Checkpoint resource not found, ignoring")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Checkpoint")
		return ctrl.Result{}, err
	}

	// Examine if the object is under deletion
	if !checkpoint.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, checkpoint)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(checkpoint, checkpointFinalizer) {
		controllerutil.AddFinalizer(checkpoint, checkpointFinalizer)
		if err := r.Update(ctx, checkpoint); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Initialize status if needed
	if checkpoint.Status.Phase == "" {
		checkpoint.Status.Phase = CheckpointPhasePending
		checkpoint.Status.StartTime = &metav1.Time{Time: time.Now()}
		if err := r.Status().Update(ctx, checkpoint); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Check if checkpoint has expired
	if r.isExpired(checkpoint) {
		logger.Info("Checkpoint has expired", "name", checkpoint.Name)
		if err := r.expireCheckpoint(ctx, checkpoint); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Process checkpoint based on current phase
	switch checkpoint.Status.Phase {
	case CheckpointPhasePending:
		return r.reconcilePending(ctx, checkpoint)
	case CheckpointPhaseInProgress:
		return r.reconcileInProgress(ctx, checkpoint)
	case CheckpointPhaseCompleted:
		return r.reconcileCompleted(ctx, checkpoint)
	case CheckpointPhaseFailed:
		return r.reconcileFailed(ctx, checkpoint)
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// reconcileDelete handles cleanup when Checkpoint is deleted
func (r *CheckpointReconciler) reconcileDelete(ctx context.Context, checkpoint *gressv1alpha1.Checkpoint) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Deleting Checkpoint", "name", checkpoint.Name)

	if controllerutil.ContainsFinalizer(checkpoint, checkpointFinalizer) {
		// Delete checkpoint data from storage
		if err := r.deleteCheckpointData(ctx, checkpoint); err != nil {
			logger.Error(err, "Failed to delete checkpoint data")
			// Continue with deletion even if cleanup fails
		}

		// Remove finalizer
		controllerutil.RemoveFinalizer(checkpoint, checkpointFinalizer)
		if err := r.Update(ctx, checkpoint); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// reconcilePending handles pending checkpoints
func (r *CheckpointReconciler) reconcilePending(ctx context.Context, checkpoint *gressv1alpha1.Checkpoint) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Verify the associated StreamJob exists
	streamJob := &gressv1alpha1.StreamJob{}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      checkpoint.Spec.StreamJobName,
		Namespace: checkpoint.Namespace,
	}, streamJob); err != nil {
		if errors.IsNotFound(err) {
			logger.Error(err, "Associated StreamJob not found")
			checkpoint.Status.Phase = CheckpointPhaseFailed
			checkpoint.Status.FailureReason = "StreamJob not found"
			r.updateCondition(checkpoint, "CheckpointFailed", metav1.ConditionTrue, "StreamJobNotFound", "Associated StreamJob not found")
			return ctrl.Result{}, r.Status().Update(ctx, checkpoint)
		}
		return ctrl.Result{}, err
	}

	// Start checkpoint process
	logger.Info("Starting checkpoint", "type", checkpoint.Spec.Type)
	checkpoint.Status.Phase = CheckpointPhaseInProgress
	r.updateCondition(checkpoint, "CheckpointInProgress", metav1.ConditionTrue, "CheckpointStarted", "Checkpoint process has started")

	if err := r.Status().Update(ctx, checkpoint); err != nil {
		return ctrl.Result{}, err
	}

	// Trigger checkpoint on the StreamJob pods
	if err := r.triggerCheckpoint(ctx, checkpoint, streamJob); err != nil {
		logger.Error(err, "Failed to trigger checkpoint")
		checkpoint.Status.Phase = CheckpointPhaseFailed
		checkpoint.Status.FailureReason = err.Error()
		r.updateCondition(checkpoint, "CheckpointFailed", metav1.ConditionTrue, "TriggerFailed", err.Error())
		return ctrl.Result{}, r.Status().Update(ctx, checkpoint)
	}

	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

// reconcileInProgress monitors checkpoint progress
func (r *CheckpointReconciler) reconcileInProgress(ctx context.Context, checkpoint *gressv1alpha1.Checkpoint) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Check checkpoint status from pods
	completed, err := r.checkCheckpointStatus(ctx, checkpoint)
	if err != nil {
		logger.Error(err, "Failed to check checkpoint status")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	if completed {
		logger.Info("Checkpoint completed successfully")
		checkpoint.Status.Phase = CheckpointPhaseCompleted
		checkpoint.Status.CompletionTime = &metav1.Time{Time: time.Now()}

		// Calculate duration
		if checkpoint.Status.StartTime != nil {
			duration := checkpoint.Status.CompletionTime.Sub(checkpoint.Status.StartTime.Time)
			checkpoint.Status.Duration = duration.String()
		}

		// Set expiration time based on retention policy
		if checkpoint.Spec.RetentionPolicy != nil && checkpoint.Spec.RetentionPolicy.MaxAge != "" {
			maxAge, err := time.ParseDuration(checkpoint.Spec.RetentionPolicy.MaxAge)
			if err == nil {
				expirationTime := metav1.NewTime(time.Now().Add(maxAge))
				checkpoint.Status.ExpirationTime = &expirationTime
			}
		}

		r.updateCondition(checkpoint, "CheckpointCompleted", metav1.ConditionTrue, "CheckpointSucceeded", "Checkpoint completed successfully")
		return ctrl.Result{}, r.Status().Update(ctx, checkpoint)
	}

	// Check for timeout
	if r.isTimedOut(checkpoint) {
		logger.Info("Checkpoint timed out")
		checkpoint.Status.Phase = CheckpointPhaseFailed
		checkpoint.Status.FailureReason = "Checkpoint timed out"
		r.updateCondition(checkpoint, "CheckpointFailed", metav1.ConditionTrue, "Timeout", "Checkpoint operation timed out")
		return ctrl.Result{}, r.Status().Update(ctx, checkpoint)
	}

	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

// reconcileCompleted handles completed checkpoints
func (r *CheckpointReconciler) reconcileCompleted(ctx context.Context, checkpoint *gressv1alpha1.Checkpoint) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Apply retention policy
	if err := r.applyRetentionPolicy(ctx, checkpoint); err != nil {
		logger.Error(err, "Failed to apply retention policy")
		// Don't fail on retention policy errors
	}

	// Check if checkpoint should be expired
	if checkpoint.Status.ExpirationTime != nil && time.Now().After(checkpoint.Status.ExpirationTime.Time) {
		return r.expireCheckpoint(ctx, checkpoint)
	}

	// Requeue to check expiration later
	if checkpoint.Status.ExpirationTime != nil {
		timeUntilExpiration := time.Until(checkpoint.Status.ExpirationTime.Time)
		if timeUntilExpiration > 0 {
			return ctrl.Result{RequeueAfter: timeUntilExpiration}, nil
		}
	}

	return ctrl.Result{}, nil
}

// reconcileFailed handles failed checkpoints
func (r *CheckpointReconciler) reconcileFailed(ctx context.Context, checkpoint *gressv1alpha1.Checkpoint) (ctrl.Result, error) {
	// Failed checkpoints may be cleaned up based on retention policy
	if checkpoint.Spec.RetentionPolicy != nil && !checkpoint.Spec.RetentionPolicy.RetainOnJobFailure {
		// Delete the checkpoint
		return ctrl.Result{}, r.Delete(ctx, checkpoint)
	}
	return ctrl.Result{}, nil
}

// triggerCheckpoint triggers checkpoint on StreamJob pods
func (r *CheckpointReconciler) triggerCheckpoint(ctx context.Context, checkpoint *gressv1alpha1.Checkpoint, streamJob *gressv1alpha1.StreamJob) error {
	logger := log.FromContext(ctx)

	// This would typically:
	// 1. Get all pods for the StreamJob
	// 2. Send checkpoint trigger signal to each pod (via HTTP API or exec)
	// 3. Initialize checkpoint status for each task

	// For now, we'll simulate this by updating the status
	checkpoint.Status.TaskCheckpoints = []gressv1alpha1.TaskCheckpointStatus{}
	for i := int32(0); i < streamJob.Spec.Parallelism; i++ {
		checkpoint.Status.TaskCheckpoints = append(checkpoint.Status.TaskCheckpoints, gressv1alpha1.TaskCheckpointStatus{
			TaskIndex: i,
			State:     "pending",
		})
	}

	logger.Info("Checkpoint triggered", "parallelism", streamJob.Spec.Parallelism)
	return nil
}

// checkCheckpointStatus checks if all tasks have completed checkpointing
func (r *CheckpointReconciler) checkCheckpointStatus(ctx context.Context, checkpoint *gressv1alpha1.Checkpoint) (bool, error) {
	// This would query the actual checkpoint status from pods
	// For now, we'll simulate completion after some time
	if checkpoint.Status.StartTime != nil {
		elapsed := time.Since(checkpoint.Status.StartTime.Time)
		if elapsed > 10*time.Second {
			// Simulate all tasks completing
			for i := range checkpoint.Status.TaskCheckpoints {
				checkpoint.Status.TaskCheckpoints[i].State = "completed"
				checkpoint.Status.TaskCheckpoints[i].Size = 1024 * 1024 // 1MB
				checkpoint.Status.TaskCheckpoints[i].Duration = "2s"
			}

			// Calculate total size
			var totalSize int64
			for _, task := range checkpoint.Status.TaskCheckpoints {
				totalSize += task.Size
			}
			checkpoint.Status.Size = totalSize
			checkpoint.Status.StateSize = totalSize

			return true, nil
		}
	}

	return false, nil
}

// deleteCheckpointData deletes checkpoint data from storage
func (r *CheckpointReconciler) deleteCheckpointData(ctx context.Context, checkpoint *gressv1alpha1.Checkpoint) error {
	logger := log.FromContext(ctx)

	// This would delete checkpoint data from:
	// - Filesystem (if using PVC)
	// - S3 (if using S3 storage)
	// - GCS (if using GCS storage)

	logger.Info("Deleting checkpoint data", "path", checkpoint.Spec.Path)
	return nil
}

// applyRetentionPolicy applies retention policy to delete old checkpoints
func (r *CheckpointReconciler) applyRetentionPolicy(ctx context.Context, checkpoint *gressv1alpha1.Checkpoint) error {
	if checkpoint.Spec.RetentionPolicy == nil {
		return nil
	}

	logger := log.FromContext(ctx)

	// Get all checkpoints for the same StreamJob
	checkpointList := &gressv1alpha1.CheckpointList{}
	if err := r.List(ctx, checkpointList, client.InNamespace(checkpoint.Namespace)); err != nil {
		return err
	}

	// Filter checkpoints for the same job
	jobCheckpoints := []gressv1alpha1.Checkpoint{}
	for _, cp := range checkpointList.Items {
		if cp.Spec.StreamJobName == checkpoint.Spec.StreamJobName &&
			cp.Spec.Type == checkpoint.Spec.Type &&
			cp.Status.Phase == CheckpointPhaseCompleted {
			jobCheckpoints = append(jobCheckpoints, cp)
		}
	}

	// Apply MaxCount policy
	if checkpoint.Spec.RetentionPolicy.MaxCount > 0 && len(jobCheckpoints) > int(checkpoint.Spec.RetentionPolicy.MaxCount) {
		// Sort by creation time and delete oldest
		excess := len(jobCheckpoints) - int(checkpoint.Spec.RetentionPolicy.MaxCount)
		for i := 0; i < excess; i++ {
			logger.Info("Deleting old checkpoint due to retention policy", "name", jobCheckpoints[i].Name)
			if err := r.Delete(ctx, &jobCheckpoints[i]); err != nil {
				logger.Error(err, "Failed to delete old checkpoint")
			}
		}
	}

	return nil
}

// isExpired checks if checkpoint has expired
func (r *CheckpointReconciler) isExpired(checkpoint *gressv1alpha1.Checkpoint) bool {
	if checkpoint.Status.ExpirationTime != nil {
		return time.Now().After(checkpoint.Status.ExpirationTime.Time)
	}
	return false
}

// expireCheckpoint marks checkpoint as expired and deletes it
func (r *CheckpointReconciler) expireCheckpoint(ctx context.Context, checkpoint *gressv1alpha1.Checkpoint) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Expiring checkpoint", "name", checkpoint.Name)

	checkpoint.Status.Phase = CheckpointPhaseExpired
	r.updateCondition(checkpoint, "CheckpointExpired", metav1.ConditionTrue, "RetentionPolicyExpired", "Checkpoint expired based on retention policy")

	if err := r.Status().Update(ctx, checkpoint); err != nil {
		return ctrl.Result{}, err
	}

	// Delete the checkpoint
	return ctrl.Result{}, r.Delete(ctx, checkpoint)
}

// isTimedOut checks if checkpoint operation has timed out
func (r *CheckpointReconciler) isTimedOut(checkpoint *gressv1alpha1.Checkpoint) bool {
	if checkpoint.Status.StartTime == nil {
		return false
	}

	// Default timeout of 5 minutes
	timeout := 5 * time.Minute
	if checkpoint.Spec.TriggerMode == "savepoint" {
		// Savepoints may take longer
		timeout = 15 * time.Minute
	}

	elapsed := time.Since(checkpoint.Status.StartTime.Time)
	return elapsed > timeout
}

// updateCondition updates or adds a condition to the Checkpoint status
func (r *CheckpointReconciler) updateCondition(checkpoint *gressv1alpha1.Checkpoint, conditionType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		ObservedGeneration: checkpoint.Generation,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}

	meta.SetStatusCondition(&checkpoint.Status.Conditions, condition)
}

// SetupWithManager sets up the controller with the Manager
func (r *CheckpointReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gressv1alpha1.Checkpoint{}).
		Complete(r)
}
