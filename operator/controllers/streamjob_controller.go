package controllers

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	gressv1alpha1 "github.com/therealutkarshpriyadarshi/gress/operator/api/v1alpha1"
)

const (
	streamJobFinalizer = "gress.io/finalizer"

	// Condition types
	ConditionTypeReady       = "Ready"
	ConditionTypeProgressing = "Progressing"
	ConditionTypeDegraded    = "Degraded"

	// Phases
	PhaseP ending      = "Pending"
	PhaseRunning      = "Running"
	PhaseSucceeded    = "Succeeded"
	PhaseFailed       = "Failed"
	PhaseSuspended    = "Suspended"
)

// StreamJobReconciler reconciles a StreamJob object
type StreamJobReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=gress.io,resources=streamjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gress.io,resources=streamjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gress.io,resources=streamjobs/finalizers,verbs=update
// +kubebuilder:rbac:groups=gress.io,resources=checkpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop
func (r *StreamJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling StreamJob", "name", req.Name, "namespace", req.Namespace)

	// Fetch the StreamJob instance
	streamJob := &gressv1alpha1.StreamJob{}
	if err := r.Get(ctx, req.NamespacedName, streamJob); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("StreamJob resource not found, ignoring")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get StreamJob")
		return ctrl.Result{}, err
	}

	// Examine if the object is under deletion
	if !streamJob.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, streamJob)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(streamJob, streamJobFinalizer) {
		controllerutil.AddFinalizer(streamJob, streamJobFinalizer)
		if err := r.Update(ctx, streamJob); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Initialize status if needed
	if streamJob.Status.Phase == "" {
		streamJob.Status.Phase = PhasePending
		streamJob.Status.StartTime = &metav1.Time{Time: time.Now()}
		if err := r.Status().Update(ctx, streamJob); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Reconcile the StatefulSet or Deployment based on state backend
	if err := r.reconcileWorkload(ctx, streamJob); err != nil {
		logger.Error(err, "Failed to reconcile workload")
		r.updateCondition(streamJob, ConditionTypeDegraded, metav1.ConditionTrue, "ReconcileFailed", err.Error())
		if statusErr := r.Status().Update(ctx, streamJob); statusErr != nil {
			logger.Error(statusErr, "Failed to update status")
		}
		return ctrl.Result{}, err
	}

	// Reconcile the Service for the job
	if err := r.reconcileService(ctx, streamJob); err != nil {
		logger.Error(err, "Failed to reconcile service")
		return ctrl.Result{}, err
	}

	// Reconcile ConfigMap if configRef is not provided
	if streamJob.Spec.ConfigRef == nil {
		if err := r.reconcileConfigMap(ctx, streamJob); err != nil {
			logger.Error(err, "Failed to reconcile configmap")
			return ctrl.Result{}, err
		}
	}

	// Reconcile PVCs for state and checkpoints
	if err := r.reconcilePVCs(ctx, streamJob); err != nil {
		logger.Error(err, "Failed to reconcile PVCs")
		return ctrl.Result{}, err
	}

	// Reconcile auto-scaler if enabled
	if streamJob.Spec.AutoScaler != nil && streamJob.Spec.AutoScaler.Enabled {
		if err := r.reconcileAutoScaler(ctx, streamJob); err != nil {
			logger.Error(err, "Failed to reconcile auto-scaler")
			return ctrl.Result{}, err
		}
	}

	// Update status based on workload status
	if err := r.updateStatus(ctx, streamJob); err != nil {
		logger.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	// Reconcile periodic checkpoints
	if err := r.reconcileCheckpoints(ctx, streamJob); err != nil {
		logger.Error(err, "Failed to reconcile checkpoints")
		// Don't fail reconciliation on checkpoint errors
	}

	logger.Info("Successfully reconciled StreamJob")
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// reconcileDelete handles cleanup when StreamJob is deleted
func (r *StreamJobReconciler) reconcileDelete(ctx context.Context, streamJob *gressv1alpha1.StreamJob) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Deleting StreamJob", "name", streamJob.Name)

	if controllerutil.ContainsFinalizer(streamJob, streamJobFinalizer) {
		// Trigger savepoint before deletion if configured
		if streamJob.Spec.Checkpoint != nil {
			if err := r.createSavepoint(ctx, streamJob); err != nil {
				logger.Error(err, "Failed to create savepoint before deletion")
				// Continue with deletion even if savepoint fails
			}
		}

		// Clean up checkpoints based on retention policy
		if err := r.cleanupCheckpoints(ctx, streamJob); err != nil {
			logger.Error(err, "Failed to cleanup checkpoints")
			// Continue with deletion
		}

		// Remove finalizer
		controllerutil.RemoveFinalizer(streamJob, streamJobFinalizer)
		if err := r.Update(ctx, streamJob); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// reconcileWorkload creates or updates the StatefulSet/Deployment for the job
func (r *StreamJobReconciler) reconcileWorkload(ctx context.Context, streamJob *gressv1alpha1.StreamJob) error {
	logger := log.FromContext(ctx)

	// Determine if we need StatefulSet (for stateful state backends) or Deployment
	useStatefulSet := false
	if streamJob.Spec.StateBackend != nil && streamJob.Spec.StateBackend.Type == "rocksdb" {
		useStatefulSet = true
	}

	if useStatefulSet {
		return r.reconcileStatefulSet(ctx, streamJob)
	}
	return r.reconcileDeployment(ctx, streamJob)
}

// reconcileStatefulSet creates or updates a StatefulSet for stateful jobs
func (r *StreamJobReconciler) reconcileStatefulSet(ctx context.Context, streamJob *gressv1alpha1.StreamJob) error {
	logger := log.FromContext(ctx)

	statefulSet := &appsv1.StatefulSet{}
	statefulSetName := streamJob.Name
	err := r.Get(ctx, types.NamespacedName{Name: statefulSetName, Namespace: streamJob.Namespace}, statefulSet)

	if err != nil && errors.IsNotFound(err) {
		// Create new StatefulSet
		statefulSet = r.buildStatefulSet(streamJob)
		if err := controllerutil.SetControllerReference(streamJob, statefulSet, r.Scheme); err != nil {
			return err
		}
		logger.Info("Creating StatefulSet", "name", statefulSet.Name)
		return r.Create(ctx, statefulSet)
	} else if err != nil {
		return err
	}

	// Update existing StatefulSet if needed
	if r.shouldUpdateStatefulSet(streamJob, statefulSet) {
		// Trigger savepoint before update if configured
		if streamJob.Spec.Checkpoint != nil {
			if err := r.createSavepoint(ctx, streamJob); err != nil {
				logger.Error(err, "Failed to create savepoint before update")
			}
		}

		updated := r.buildStatefulSet(streamJob)
		statefulSet.Spec = updated.Spec
		logger.Info("Updating StatefulSet", "name", statefulSet.Name)
		return r.Update(ctx, statefulSet)
	}

	return nil
}

// reconcileDeployment creates or updates a Deployment for stateless jobs
func (r *StreamJobReconciler) reconcileDeployment(ctx context.Context, streamJob *gressv1alpha1.StreamJob) error {
	logger := log.FromContext(ctx)

	deployment := &appsv1.Deployment{}
	deploymentName := streamJob.Name
	err := r.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: streamJob.Namespace}, deployment)

	if err != nil && errors.IsNotFound(err) {
		// Create new Deployment
		deployment = r.buildDeployment(streamJob)
		if err := controllerutil.SetControllerReference(streamJob, deployment, r.Scheme); err != nil {
			return err
		}
		logger.Info("Creating Deployment", "name", deployment.Name)
		return r.Create(ctx, deployment)
	} else if err != nil {
		return err
	}

	// Update existing Deployment if needed
	if r.shouldUpdateDeployment(streamJob, deployment) {
		updated := r.buildDeployment(streamJob)
		deployment.Spec = updated.Spec
		logger.Info("Updating Deployment", "name", deployment.Name)
		return r.Update(ctx, deployment)
	}

	return nil
}

// buildStatefulSet constructs a StatefulSet from StreamJob spec
func (r *StreamJobReconciler) buildStatefulSet(streamJob *gressv1alpha1.StreamJob) *appsv1.StatefulSet {
	labels := map[string]string{
		"app":        "gress",
		"component":  "streamjob",
		"streamjob":  streamJob.Name,
		"managed-by": "gress-operator",
	}

	podSpec := r.buildPodSpec(streamJob)

	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      streamJob.Name,
			Namespace: streamJob.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &streamJob.Spec.Parallelism,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			ServiceName: streamJob.Name,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: podSpec,
			},
			PodManagementPolicy: appsv1.ParallelPodManagement,
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
		},
	}

	// Add VolumeClaimTemplates for state and checkpoints
	if streamJob.Spec.StateBackend != nil && streamJob.Spec.StateBackend.PersistentVolumeClaim != nil {
		statefulSet.Spec.VolumeClaimTemplates = append(statefulSet.Spec.VolumeClaimTemplates,
			r.buildPVCTemplate("state", streamJob.Spec.StateBackend.PersistentVolumeClaim))
	}

	if streamJob.Spec.Checkpoint != nil && streamJob.Spec.Checkpoint.Storage != nil &&
		streamJob.Spec.Checkpoint.Storage.PersistentVolumeClaim != nil {
		statefulSet.Spec.VolumeClaimTemplates = append(statefulSet.Spec.VolumeClaimTemplates,
			r.buildPVCTemplate("checkpoint", streamJob.Spec.Checkpoint.Storage.PersistentVolumeClaim))
	}

	return statefulSet
}

// buildDeployment constructs a Deployment from StreamJob spec
func (r *StreamJobReconciler) buildDeployment(streamJob *gressv1alpha1.StreamJob) *appsv1.Deployment {
	labels := map[string]string{
		"app":        "gress",
		"component":  "streamjob",
		"streamjob":  streamJob.Name,
		"managed-by": "gress-operator",
	}

	podSpec := r.buildPodSpec(streamJob)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      streamJob.Name,
			Namespace: streamJob.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &streamJob.Spec.Parallelism,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: podSpec,
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
		},
	}

	return deployment
}

// buildPodSpec constructs the PodSpec for the job
func (r *StreamJobReconciler) buildPodSpec(streamJob *gressv1alpha1.StreamJob) corev1.PodSpec {
	container := corev1.Container{
		Name:            "gress-job",
		Image:           streamJob.Spec.Image,
		ImagePullPolicy: streamJob.Spec.ImagePullPolicy,
		Resources:       streamJob.Spec.Resources,
		Env:             r.buildEnvVars(streamJob),
		VolumeMounts:    r.buildVolumeMounts(streamJob),
		Ports: []corev1.ContainerPort{
			{
				Name:          "metrics",
				ContainerPort: 9091,
				Protocol:      corev1.ProtocolTCP,
			},
		},
	}

	podSpec := corev1.PodSpec{
		ServiceAccountName: streamJob.Spec.ServiceAccountName,
		Containers:         []corev1.Container{container},
		Volumes:            r.buildVolumes(streamJob),
		ImagePullSecrets:   streamJob.Spec.ImagePullSecrets,
		Affinity:           streamJob.Spec.Affinity,
		Tolerations:        streamJob.Spec.Tolerations,
		NodeSelector:       streamJob.Spec.NodeSelector,
	}

	return podSpec
}

// buildEnvVars constructs environment variables for the container
func (r *StreamJobReconciler) buildEnvVars(streamJob *gressv1alpha1.StreamJob) []corev1.EnvVar {
	envVars := []corev1.EnvVar{
		{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name: "POD_NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
		{
			Name: "POD_IP",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "status.podIP",
				},
			},
		},
	}

	// Add custom env vars
	envVars = append(envVars, streamJob.Spec.Env...)

	// Add config path if ConfigRef is provided
	if streamJob.Spec.ConfigRef != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "GRESS_CONFIG_PATH",
			Value: "/etc/gress/config.yaml",
		})
	}

	// Add savepoint path if provided
	if streamJob.Spec.SavepointPath != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "GRESS_SAVEPOINT_PATH",
			Value: streamJob.Spec.SavepointPath,
		})
	}

	return envVars
}

// buildVolumeMounts constructs volume mounts for the container
func (r *StreamJobReconciler) buildVolumeMounts(streamJob *gressv1alpha1.StreamJob) []corev1.VolumeMount {
	volumeMounts := []corev1.VolumeMount{}

	// Add state volume mount for RocksDB
	if streamJob.Spec.StateBackend != nil && streamJob.Spec.StateBackend.Type == "rocksdb" {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "state",
			MountPath: "/var/lib/gress/rocksdb",
		})
	}

	// Add checkpoint volume mount
	if streamJob.Spec.Checkpoint != nil && streamJob.Spec.Checkpoint.Storage != nil {
		path := "/var/lib/gress/checkpoints"
		if streamJob.Spec.Checkpoint.Storage.Path != "" {
			path = streamJob.Spec.Checkpoint.Storage.Path
		}
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "checkpoint",
			MountPath: path,
		})
	}

	// Add config volume mount
	if streamJob.Spec.ConfigRef != nil {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "config",
			MountPath: "/etc/gress",
		})
	}

	// Add custom volume mounts
	volumeMounts = append(volumeMounts, streamJob.Spec.VolumeMounts...)

	return volumeMounts
}

// buildVolumes constructs volumes for the pod
func (r *StreamJobReconciler) buildVolumes(streamJob *gressv1alpha1.StreamJob) []corev1.Volume {
	volumes := []corev1.Volume{}

	// Add config volume
	if streamJob.Spec.ConfigRef != nil {
		volumes = append(volumes, corev1.Volume{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: *streamJob.Spec.ConfigRef,
				},
			},
		})
	}

	// Add custom volumes
	volumes = append(volumes, streamJob.Spec.Volumes...)

	return volumes
}

// buildPVCTemplate constructs a PVC template for StatefulSet
func (r *StreamJobReconciler) buildPVCTemplate(name string, spec *corev1.PersistentVolumeClaimSpec) corev1.PersistentVolumeClaim {
	return corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: *spec,
	}
}

// Remaining methods continue in next part...

// shouldUpdateStatefulSet checks if StatefulSet needs update
func (r *StreamJobReconciler) shouldUpdateStatefulSet(streamJob *gressv1alpha1.StreamJob, sts *appsv1.StatefulSet) bool {
	// Check if replicas changed
	if *sts.Spec.Replicas != streamJob.Spec.Parallelism {
		return true
	}
	// Check if image changed
	if len(sts.Spec.Template.Spec.Containers) > 0 && sts.Spec.Template.Spec.Containers[0].Image != streamJob.Spec.Image {
		return true
	}
	return false
}

// shouldUpdateDeployment checks if Deployment needs update
func (r *StreamJobReconciler) shouldUpdateDeployment(streamJob *gressv1alpha1.StreamJob, dep *appsv1.Deployment) bool {
	if *dep.Spec.Replicas != streamJob.Spec.Parallelism {
		return true
	}
	if len(dep.Spec.Template.Spec.Containers) > 0 && dep.Spec.Template.Spec.Containers[0].Image != streamJob.Spec.Image {
		return true
	}
	return false
}

// reconcileService creates or updates the Service for the job
func (r *StreamJobReconciler) reconcileService(ctx context.Context, streamJob *gressv1alpha1.StreamJob) error {
	service := &corev1.Service{}
	serviceName := streamJob.Name
	err := r.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: streamJob.Namespace}, service)

	if err != nil && errors.IsNotFound(err) {
		service = r.buildService(streamJob)
		if err := controllerutil.SetControllerReference(streamJob, service, r.Scheme); err != nil {
			return err
		}
		return r.Create(ctx, service)
	}
	return err
}

// buildService constructs a Service for the job
func (r *StreamJobReconciler) buildService(streamJob *gressv1alpha1.StreamJob) *corev1.Service {
	labels := map[string]string{
		"app":        "gress",
		"component":  "streamjob",
		"streamjob":  streamJob.Name,
		"managed-by": "gress-operator",
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      streamJob.Name,
			Namespace: streamJob.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector:  labels,
			ClusterIP: corev1.ClusterIPNone, // Headless service for StatefulSet
			Ports: []corev1.ServicePort{
				{
					Name:     "metrics",
					Port:     9091,
					Protocol: corev1.ProtocolTCP,
				},
			},
		},
	}
}

// reconcileConfigMap creates ConfigMap for job configuration
func (r *StreamJobReconciler) reconcileConfigMap(ctx context.Context, streamJob *gressv1alpha1.StreamJob) error {
	// Implementation to create ConfigMap from StreamJob spec
	// This would convert sources, sinks, pipeline to gress config format
	return nil
}

// reconcilePVCs ensures PVCs exist if needed
func (r *StreamJobReconciler) reconcilePVCs(ctx context.Context, streamJob *gressv1alpha1.StreamJob) error {
	// PVCs are created via VolumeClaimTemplates in StatefulSet
	// This method can handle standalone PVCs if needed
	return nil
}

// reconcileAutoScaler creates or updates HPA for the job
func (r *StreamJobReconciler) reconcileAutoScaler(ctx context.Context, streamJob *gressv1alpha1.StreamJob) error {
	// Implementation for HorizontalPodAutoscaler creation
	// This would support custom metrics like Kafka lag
	return nil
}

// reconcileCheckpoints manages checkpoint resources
func (r *StreamJobReconciler) reconcileCheckpoints(ctx context.Context, streamJob *gressv1alpha1.StreamJob) error {
	// Implementation to create Checkpoint resources periodically
	return nil
}

// createSavepoint creates a savepoint before updates/deletion
func (r *StreamJobReconciler) createSavepoint(ctx context.Context, streamJob *gressv1alpha1.StreamJob) error {
	// Implementation to trigger savepoint creation
	return nil
}

// cleanupCheckpoints removes old checkpoints based on retention policy
func (r *StreamJobReconciler) cleanupCheckpoints(ctx context.Context, streamJob *gressv1alpha1.StreamJob) error {
	// Implementation to cleanup old checkpoints
	return nil
}

// updateStatus updates the StreamJob status based on workload status
func (r *StreamJobReconciler) updateStatus(ctx context.Context, streamJob *gressv1alpha1.StreamJob) error {
	// Get StatefulSet or Deployment status
	if streamJob.Spec.StateBackend != nil && streamJob.Spec.StateBackend.Type == "rocksdb" {
		sts := &appsv1.StatefulSet{}
		if err := r.Get(ctx, types.NamespacedName{Name: streamJob.Name, Namespace: streamJob.Namespace}, sts); err != nil {
			return err
		}
		streamJob.Status.CurrentReplicas = sts.Status.Replicas
		streamJob.Status.ReadyReplicas = sts.Status.ReadyReplicas
	} else {
		dep := &appsv1.Deployment{}
		if err := r.Get(ctx, types.NamespacedName{Name: streamJob.Name, Namespace: streamJob.Namespace}, dep); err != nil {
			return err
		}
		streamJob.Status.CurrentReplicas = dep.Status.Replicas
		streamJob.Status.ReadyReplicas = dep.Status.ReadyReplicas
	}

	// Update phase
	if streamJob.Status.ReadyReplicas == streamJob.Spec.Parallelism {
		streamJob.Status.Phase = PhaseRunning
		r.updateCondition(streamJob, ConditionTypeReady, metav1.ConditionTrue, "AllReplicasReady", "All replicas are ready")
	} else if streamJob.Status.ReadyReplicas > 0 {
		streamJob.Status.Phase = PhaseRunning
		r.updateCondition(streamJob, ConditionTypeProgressing, metav1.ConditionTrue, "SomeReplicasReady", "Some replicas are ready")
	} else {
		streamJob.Status.Phase = PhasePending
		r.updateCondition(streamJob, ConditionTypeProgressing, metav1.ConditionTrue, "WaitingForReplicas", "Waiting for replicas to be ready")
	}

	streamJob.Status.ObservedGeneration = streamJob.Generation
	return r.Status().Update(ctx, streamJob)
}

// updateCondition updates or adds a condition to the StreamJob status
func (r *StreamJobReconciler) updateCondition(streamJob *gressv1alpha1.StreamJob, conditionType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		ObservedGeneration: streamJob.Generation,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}

	meta.SetStatusCondition(&streamJob.Status.Conditions, condition)
}

// SetupWithManager sets up the controller with the Manager
func (r *StreamJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gressv1alpha1.StreamJob{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
