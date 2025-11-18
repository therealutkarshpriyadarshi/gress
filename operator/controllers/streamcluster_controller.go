package controllers

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	gressv1alpha1 "github.com/therealutkarshpriyadarshi/gress/operator/api/v1alpha1"
)

const (
	streamClusterFinalizer = "gress.io/streamcluster-finalizer"
)

// StreamClusterReconciler reconciles a StreamCluster object
type StreamClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=gress.io,resources=streamclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gress.io,resources=streamclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gress.io,resources=streamclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop
func (r *StreamClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling StreamCluster", "name", req.Name, "namespace", req.Namespace)

	// Fetch the StreamCluster instance
	cluster := &gressv1alpha1.StreamCluster{}
	if err := r.Get(ctx, req.NamespacedName, cluster); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("StreamCluster resource not found, ignoring")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get StreamCluster")
		return ctrl.Result{}, err
	}

	// Examine if the object is under deletion
	if !cluster.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, cluster)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(cluster, streamClusterFinalizer) {
		controllerutil.AddFinalizer(cluster, streamClusterFinalizer)
		if err := r.Update(ctx, cluster); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Initialize status if needed
	if cluster.Status.Phase == "" {
		cluster.Status.Phase = "Pending"
		if err := r.Status().Update(ctx, cluster); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Reconcile Job Managers if configured
	if cluster.Spec.JobManagers != nil {
		if err := r.reconcileJobManagers(ctx, cluster); err != nil {
			logger.Error(err, "Failed to reconcile job managers")
			r.updateCondition(cluster, ConditionTypeDegraded, metav1.ConditionTrue, "JobManagersFailed", err.Error())
			if statusErr := r.Status().Update(ctx, cluster); statusErr != nil {
				logger.Error(statusErr, "Failed to update status")
			}
			return ctrl.Result{}, err
		}
	}

	// Reconcile Task Managers
	if cluster.Spec.TaskManagers != nil {
		if err := r.reconcileTaskManagers(ctx, cluster); err != nil {
			logger.Error(err, "Failed to reconcile task managers")
			return ctrl.Result{}, err
		}
	}

	// Reconcile monitoring if enabled
	if cluster.Spec.Monitoring != nil && cluster.Spec.Monitoring.Enabled {
		if err := r.reconcileMonitoring(ctx, cluster); err != nil {
			logger.Error(err, "Failed to reconcile monitoring")
			// Don't fail on monitoring errors
		}
	}

	// Update cluster status
	if err := r.updateClusterStatus(ctx, cluster); err != nil {
		logger.Error(err, "Failed to update cluster status")
		return ctrl.Result{}, err
	}

	logger.Info("Successfully reconciled StreamCluster")
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// reconcileDelete handles cleanup when StreamCluster is deleted
func (r *StreamClusterReconciler) reconcileDelete(ctx context.Context, cluster *gressv1alpha1.StreamCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Deleting StreamCluster", "name", cluster.Name)

	if controllerutil.ContainsFinalizer(cluster, streamClusterFinalizer) {
		// Perform cleanup tasks here
		// Remove finalizer
		controllerutil.RemoveFinalizer(cluster, streamClusterFinalizer)
		if err := r.Update(ctx, cluster); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// reconcileJobManagers creates or updates job manager resources
func (r *StreamClusterReconciler) reconcileJobManagers(ctx context.Context, cluster *gressv1alpha1.StreamCluster) error {
	logger := log.FromContext(ctx)

	// Create StatefulSet for Job Managers (for HA support)
	sts := &appsv1.StatefulSet{}
	stsName := fmt.Sprintf("%s-jobmanager", cluster.Name)
	err := r.Get(ctx, types.NamespacedName{Name: stsName, Namespace: cluster.Namespace}, sts)

	if err != nil && errors.IsNotFound(err) {
		sts = r.buildJobManagerStatefulSet(cluster)
		if err := controllerutil.SetControllerReference(cluster, sts, r.Scheme); err != nil {
			return err
		}
		logger.Info("Creating JobManager StatefulSet", "name", sts.Name)
		if err := r.Create(ctx, sts); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		// Update if needed
		updated := r.buildJobManagerStatefulSet(cluster)
		if *sts.Spec.Replicas != *updated.Spec.Replicas {
			sts.Spec.Replicas = updated.Spec.Replicas
			sts.Spec.Template = updated.Spec.Template
			if err := r.Update(ctx, sts); err != nil {
				return err
			}
		}
	}

	// Create Service for Job Managers
	svc := &corev1.Service{}
	svcName := fmt.Sprintf("%s-jobmanager", cluster.Name)
	err = r.Get(ctx, types.NamespacedName{Name: svcName, Namespace: cluster.Namespace}, svc)

	if err != nil && errors.IsNotFound(err) {
		svc = r.buildJobManagerService(cluster)
		if err := controllerutil.SetControllerReference(cluster, svc, r.Scheme); err != nil {
			return err
		}
		logger.Info("Creating JobManager Service", "name", svc.Name)
		return r.Create(ctx, svc)
	}

	return err
}

// reconcileTaskManagers creates or updates task manager resources
func (r *StreamClusterReconciler) reconcileTaskManagers(ctx context.Context, cluster *gressv1alpha1.StreamCluster) error {
	logger := log.FromContext(ctx)

	// Create StatefulSet for Task Managers
	sts := &appsv1.StatefulSet{}
	stsName := fmt.Sprintf("%s-taskmanager", cluster.Name)
	err := r.Get(ctx, types.NamespacedName{Name: stsName, Namespace: cluster.Namespace}, sts)

	if err != nil && errors.IsNotFound(err) {
		sts = r.buildTaskManagerStatefulSet(cluster)
		if err := controllerutil.SetControllerReference(cluster, sts, r.Scheme); err != nil {
			return err
		}
		logger.Info("Creating TaskManager StatefulSet", "name", sts.Name)
		if err := r.Create(ctx, sts); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		// Update if needed
		updated := r.buildTaskManagerStatefulSet(cluster)
		if *sts.Spec.Replicas != *updated.Spec.Replicas {
			sts.Spec.Replicas = updated.Spec.Replicas
			sts.Spec.Template = updated.Spec.Template
			if err := r.Update(ctx, sts); err != nil {
				return err
			}
		}
	}

	// Create headless Service for Task Managers
	svc := &corev1.Service{}
	svcName := fmt.Sprintf("%s-taskmanager", cluster.Name)
	err = r.Get(ctx, types.NamespacedName{Name: svcName, Namespace: cluster.Namespace}, svc)

	if err != nil && errors.IsNotFound(err) {
		svc = r.buildTaskManagerService(cluster)
		if err := controllerutil.SetControllerReference(cluster, svc, r.Scheme); err != nil {
			return err
		}
		logger.Info("Creating TaskManager Service", "name", svc.Name)
		return r.Create(ctx, svc)
	}

	return err
}

// buildJobManagerStatefulSet constructs StatefulSet for job managers
func (r *StreamClusterReconciler) buildJobManagerStatefulSet(cluster *gressv1alpha1.StreamCluster) *appsv1.StatefulSet {
	labels := map[string]string{
		"app":           "gress",
		"component":     "jobmanager",
		"cluster":       cluster.Name,
		"managed-by":    "gress-operator",
	}

	replicas := int32(1)
	if cluster.Spec.JobManagers != nil {
		replicas = cluster.Spec.JobManagers.Replicas
	}

	image := cluster.Spec.Image
	if image == "" {
		image = fmt.Sprintf("gress:%s", cluster.Spec.Version)
	}

	resources := corev1.ResourceRequirements{}
	if cluster.Spec.JobManagers != nil {
		resources = cluster.Spec.JobManagers.Resources
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-jobmanager", cluster.Name),
			Namespace: cluster.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &replicas,
			ServiceName: fmt.Sprintf("%s-jobmanager", cluster.Name),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: cluster.Spec.ServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:      "jobmanager",
							Image:     image,
							Resources: resources,
							Command:   []string{"/usr/local/bin/gress", "jobmanager"},
							Env: []corev1.EnvVar{
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
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "rpc",
									ContainerPort: 6123,
								},
								{
									Name:          "ui",
									ContainerPort: 8081,
								},
								{
									Name:          "metrics",
									ContainerPort: 9091,
								},
							},
						},
					},
				},
			},
		},
	}

	// Add storage if configured
	if cluster.Spec.JobManagers != nil && cluster.Spec.JobManagers.Storage != nil {
		sts.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "storage",
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse(cluster.Spec.JobManagers.Storage.Size),
						},
					},
					StorageClassName: cluster.Spec.JobManagers.Storage.StorageClassName,
				},
			},
		}

		sts.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			sts.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      "storage",
				MountPath: "/var/lib/gress",
			},
		)
	}

	return sts
}

// buildJobManagerService constructs Service for job managers
func (r *StreamClusterReconciler) buildJobManagerService(cluster *gressv1alpha1.StreamCluster) *corev1.Service {
	labels := map[string]string{
		"app":        "gress",
		"component":  "jobmanager",
		"cluster":    cluster.Name,
		"managed-by": "gress-operator",
	}

	serviceType := corev1.ServiceTypeClusterIP
	if cluster.Spec.JobManagers != nil {
		serviceType = cluster.Spec.JobManagers.ServiceType
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-jobmanager", cluster.Name),
			Namespace: cluster.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type:     serviceType,
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "rpc",
					Port:       6123,
					TargetPort: intstr.FromInt(6123),
				},
				{
					Name:       "ui",
					Port:       8081,
					TargetPort: intstr.FromInt(8081),
				},
				{
					Name:       "metrics",
					Port:       9091,
					TargetPort: intstr.FromInt(9091),
				},
			},
		},
	}
}

// buildTaskManagerStatefulSet constructs StatefulSet for task managers
func (r *StreamClusterReconciler) buildTaskManagerStatefulSet(cluster *gressv1alpha1.StreamCluster) *appsv1.StatefulSet {
	labels := map[string]string{
		"app":        "gress",
		"component":  "taskmanager",
		"cluster":    cluster.Name,
		"managed-by": "gress-operator",
	}

	replicas := int32(1)
	if cluster.Spec.TaskManagers != nil {
		replicas = cluster.Spec.TaskManagers.Replicas
	}

	image := cluster.Spec.Image
	if image == "" {
		image = fmt.Sprintf("gress:%s", cluster.Spec.Version)
	}

	resources := corev1.ResourceRequirements{}
	if cluster.Spec.TaskManagers != nil {
		resources = cluster.Spec.TaskManagers.Resources
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-taskmanager", cluster.Name),
			Namespace: cluster.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &replicas,
			ServiceName: fmt.Sprintf("%s-taskmanager", cluster.Name),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: cluster.Spec.ServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:      "taskmanager",
							Image:     image,
							Resources: resources,
							Command:   []string{"/usr/local/bin/gress", "taskmanager"},
							Env: []corev1.EnvVar{
								{
									Name:  "JOBMANAGER_HOST",
									Value: fmt.Sprintf("%s-jobmanager", cluster.Name),
								},
								{
									Name: "POD_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "data",
									ContainerPort: 6121,
								},
								{
									Name:          "metrics",
									ContainerPort: 9091,
								},
							},
						},
					},
				},
			},
		},
	}

	// Add storage if configured
	if cluster.Spec.TaskManagers != nil && cluster.Spec.TaskManagers.Storage != nil {
		sts.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "storage",
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse(cluster.Spec.TaskManagers.Storage.Size),
						},
					},
					StorageClassName: cluster.Spec.TaskManagers.Storage.StorageClassName,
				},
			},
		}

		sts.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			sts.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      "storage",
				MountPath: "/var/lib/gress",
			},
		)
	}

	return sts
}

// buildTaskManagerService constructs headless Service for task managers
func (r *StreamClusterReconciler) buildTaskManagerService(cluster *gressv1alpha1.StreamCluster) *corev1.Service {
	labels := map[string]string{
		"app":        "gress",
		"component":  "taskmanager",
		"cluster":    cluster.Name,
		"managed-by": "gress-operator",
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-taskmanager", cluster.Name),
			Namespace: cluster.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: corev1.ClusterIPNone,
			Selector:  labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "data",
					Port:       6121,
					TargetPort: intstr.FromInt(6121),
				},
				{
					Name:       "metrics",
					Port:       9091,
					TargetPort: intstr.FromInt(9091),
				},
			},
		},
	}
}

// reconcileMonitoring creates ServiceMonitor for Prometheus
func (r *StreamClusterReconciler) reconcileMonitoring(ctx context.Context, cluster *gressv1alpha1.StreamCluster) error {
	// Implementation for ServiceMonitor creation
	// This requires Prometheus Operator CRDs
	return nil
}

// updateClusterStatus updates the cluster status
func (r *StreamClusterReconciler) updateClusterStatus(ctx context.Context, cluster *gressv1alpha1.StreamCluster) error {
	// Get JobManager status
	if cluster.Spec.JobManagers != nil {
		sts := &appsv1.StatefulSet{}
		name := fmt.Sprintf("%s-jobmanager", cluster.Name)
		if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: cluster.Namespace}, sts); err == nil {
			cluster.Status.JobManagerReplicas = sts.Status.Replicas
			cluster.Status.JobManagerReadyReplicas = sts.Status.ReadyReplicas
		}
	}

	// Get TaskManager status
	if cluster.Spec.TaskManagers != nil {
		sts := &appsv1.StatefulSet{}
		name := fmt.Sprintf("%s-taskmanager", cluster.Name)
		if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: cluster.Namespace}, sts); err == nil {
			cluster.Status.TaskManagerReplicas = sts.Status.Replicas
			cluster.Status.TaskManagerReadyReplicas = sts.Status.ReadyReplicas
		}
	}

	// Update phase
	jmReady := cluster.Spec.JobManagers == nil || cluster.Status.JobManagerReadyReplicas == cluster.Spec.JobManagers.Replicas
	tmReady := cluster.Spec.TaskManagers == nil || cluster.Status.TaskManagerReadyReplicas == cluster.Spec.TaskManagers.Replicas

	if jmReady && tmReady {
		cluster.Status.Phase = "Running"
		r.updateCondition(cluster, ConditionTypeReady, metav1.ConditionTrue, "ClusterReady", "All components are ready")
	} else {
		cluster.Status.Phase = "Pending"
		r.updateCondition(cluster, ConditionTypeProgressing, metav1.ConditionTrue, "ComponentsStarting", "Waiting for components to be ready")
	}

	cluster.Status.ObservedGeneration = cluster.Generation
	cluster.Status.LastUpdateTime = &metav1.Time{Time: time.Now()}

	// Set cluster endpoint
	if cluster.Spec.JobManagers != nil {
		cluster.Status.ClusterEndpoint = fmt.Sprintf("%s-jobmanager.%s.svc.cluster.local:8081", cluster.Name, cluster.Namespace)
		cluster.Status.MetricsEndpoint = fmt.Sprintf("%s-jobmanager.%s.svc.cluster.local:9091/metrics", cluster.Name, cluster.Namespace)
	}

	return r.Status().Update(ctx, cluster)
}

// updateCondition updates or adds a condition to the StreamCluster status
func (r *StreamClusterReconciler) updateCondition(cluster *gressv1alpha1.StreamCluster, conditionType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		ObservedGeneration: cluster.Generation,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}

	meta.SetStatusCondition(&cluster.Status.Conditions, condition)
}

// SetupWithManager sets up the controller with the Manager
func (r *StreamClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gressv1alpha1.StreamCluster{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
