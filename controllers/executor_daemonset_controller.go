package controllers

import (
	"context"
	"fmt"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"k8s.io/client-go/kubernetes"

	nodecheckv1alpha1 "github.com/albertofilice/node-check-operator/api/v1alpha1"
)

// ExecutorDaemonSetReconciler reconciles DaemonSet for NodeCheck executors
// This controller ensures that a DaemonSet exists when there are NodeCheck resources
type ExecutorDaemonSetReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Clientset kubernetes.Interface
	Namespace string
	Image     string
}

//+kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups=nodecheck.openshift.io,resources=nodechecks,verbs=get;list;watch

// Reconcile ensures the executor DaemonSet exists when NodeChecks are present
func (r *ExecutorDaemonSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.Log.WithName("ExecutorDaemonSetReconciler")

	// Check if there are any NodeChecks
	var nodeChecks nodecheckv1alpha1.NodeCheckList
	if err := r.Client.List(ctx, &nodeChecks); err != nil {
		log.Error(err, "unable to list NodeChecks")
		return ctrl.Result{}, err
	}

	// Filter out template NodeChecks (nodeName="*" or "all")
	// NodeChecks with empty nodeName are considered active (will be auto-detected by executor)
	hasActiveNodeChecks := false
	for _, nc := range nodeChecks.Items {
		if nc.Spec.NodeName != "*" && nc.Spec.NodeName != "all" {
			hasActiveNodeChecks = true
			break
		}
	}

	daemonSetName := "node-check-executor"
	daemonSetNamespace := r.Namespace

	var daemonSet appsv1.DaemonSet
	err := r.Get(ctx, types.NamespacedName{Name: daemonSetName, Namespace: daemonSetNamespace}, &daemonSet)

	if hasActiveNodeChecks {
		// DaemonSet should exist
		if errors.IsNotFound(err) {
			// Create DaemonSet
			log.Info("Creating executor DaemonSet", "name", daemonSetName)
			daemonSet = r.buildDaemonSet(daemonSetName, daemonSetNamespace, &nodeChecks)
			if err := r.Create(ctx, &daemonSet); err != nil {
				log.Error(err, "unable to create DaemonSet")
				return ctrl.Result{}, err
			}
			log.Info("Created executor DaemonSet", "name", daemonSetName)
			return ctrl.Result{}, nil
		} else if err != nil {
			log.Error(err, "unable to fetch DaemonSet")
			return ctrl.Result{}, err
		}
		// DaemonSet exists, ensure it's up to date
		desiredDaemonSet := r.buildDaemonSet(daemonSetName, daemonSetNamespace, &nodeChecks)
		if r.daemonSetNeedsUpdate(&daemonSet, &desiredDaemonSet) {
			log.Info("Updating executor DaemonSet", "name", daemonSetName)
			daemonSet.Spec = desiredDaemonSet.Spec
			if err := r.Update(ctx, &daemonSet); err != nil {
				log.Error(err, "unable to update DaemonSet")
				return ctrl.Result{}, err
			}
			log.Info("Updated executor DaemonSet", "name", daemonSetName)
		}
	} else {
		// No active NodeChecks, DaemonSet should not exist
		if !errors.IsNotFound(err) {
			if err != nil {
				log.Error(err, "unable to fetch DaemonSet")
				return ctrl.Result{}, err
			}
			// Delete DaemonSet
			log.Info("Deleting executor DaemonSet (no active NodeChecks)", "name", daemonSetName)
			if err := r.Delete(ctx, &daemonSet); err != nil {
				log.Error(err, "unable to delete DaemonSet")
				return ctrl.Result{}, err
			}
			log.Info("Deleted executor DaemonSet", "name", daemonSetName)
		}
	}

	return ctrl.Result{}, nil
}

// buildDaemonSet creates a DaemonSet spec for the executor
func (r *ExecutorDaemonSetReconciler) buildDaemonSet(name, namespace string, nodeChecks *nodecheckv1alpha1.NodeCheckList) appsv1.DaemonSet {
	image := r.Image
	if image == "" {
		image = "quay.io/rh_ee_afilice/node-check-operator:v1.0.7"
	}

	// Collect NodeSelector and Tolerations from all NodeChecks
	// Merge node selectors (all must match)
	mergedNodeSelector := make(map[string]string)
	mergedTolerations := make(map[string]corev1.Toleration)
	
	for _, nc := range nodeChecks.Items {
		// For template NodeChecks (nodeName="*" or "all"), only use Tolerations (not NodeSelector)
		// NodeSelector is used to filter which nodes get child NodeChecks, not for DaemonSet scheduling
		if nc.Spec.NodeName == "*" || nc.Spec.NodeName == "all" {
			// Only merge Tolerations from template NodeChecks
			for _, tol := range nc.Spec.Tolerations {
				key := fmt.Sprintf("%s:%s:%s", tol.Key, tol.Operator, tol.Effect)
				mergedTolerations[key] = tol
			}
			continue
		}
		
		// For specific NodeChecks, merge both NodeSelector and Tolerations
		// Merge node selectors
		for k, v := range nc.Spec.NodeSelector {
			mergedNodeSelector[k] = v
		}
		
		// Merge tolerations (use key as unique identifier)
		for _, tol := range nc.Spec.Tolerations {
			key := fmt.Sprintf("%s:%s:%s", tol.Key, tol.Operator, tol.Effect)
			mergedTolerations[key] = tol
		}
	}
	
	// Convert merged tolerations map to slice
	tolerationsList := make([]corev1.Toleration, 0, len(mergedTolerations))
	for _, tol := range mergedTolerations {
		tolerationsList = append(tolerationsList, tol)
	}

	daemonSet := appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app": "node-check-executor",
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "node-check-executor",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "node-check-executor",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "node-check-operator-controller-manager",
					HostNetwork:        true,
					NodeSelector:       mergedNodeSelector,
					Tolerations:        tolerationsList,
					Volumes: []corev1.Volume{
						{
							Name: "host-proc",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/proc",
									Type: func() *corev1.HostPathType { t := corev1.HostPathDirectory; return &t }(),
								},
							},
						},
						{
							Name: "host-sys",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/sys",
									Type: func() *corev1.HostPathType { t := corev1.HostPathDirectory; return &t }(),
								},
							},
						},
						{
							Name: "host-run",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/run",
									Type: func() *corev1.HostPathType { t := corev1.HostPathDirectory; return &t }(),
								},
							},
						},
						{
							Name: "host-root",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/",
									Type: func() *corev1.HostPathType { t := corev1.HostPathDirectory; return &t }(),
								},
							},
						},
						{
							Name: "host-dev",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/dev",
									Type: func() *corev1.HostPathType { t := corev1.HostPathDirectory; return &t }(),
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "executor",
							Image: image,
							Args: []string{
								"--mode=executor",
								"--leader-elect=false",
							},
							Env: []corev1.EnvVar{
								{
									Name: "NODE_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "spec.nodeName",
										},
									},
								},
								{
									Name: "WATCH_NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "host-proc",
									MountPath: "/host/proc",
									ReadOnly:  true,
								},
								{
									Name:      "host-sys",
									MountPath: "/host/sys",
									ReadOnly:  true,
								},
								{
									Name:      "host-run",
									MountPath: "/host/run",
									ReadOnly:  true,
								},
								{
									Name:      "host-root",
									MountPath: "/host/root",
									ReadOnly:  true,
								},
								{
									Name:      "host-dev",
									MountPath: "/host/dev",
									ReadOnly:  true,
								},
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8080,
									Name:          "metrics",
									Protocol:      corev1.ProtocolTCP,
								},
								{
									ContainerPort: 8081,
									Name:          "health",
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: func() *bool { b := true; return &b }(),
								Privileged:               func() *bool { b := true; return &b }(),
								RunAsNonRoot:             func() *bool { b := false; return &b }(),
								RunAsUser:                func() *int64 { u := int64(0); return &u }(),
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{
										"NET_ADMIN", "SYS_ADMIN", "DAC_READ_SEARCH", "SYS_PTRACE",
										"SYS_RESOURCE", "SYS_TIME", "SYSLOG", "AUDIT_CONTROL",
										"AUDIT_READ", "AUDIT_WRITE", "BLOCK_SUSPEND", "CHOWN",
										"DAC_OVERRIDE", "FOWNER", "IPC_LOCK", "IPC_OWNER",
										"KILL", "LEASE", "LINUX_IMMUTABLE", "MAC_ADMIN",
										"MAC_OVERRIDE", "MKNOD", "NET_BIND_SERVICE", "NET_BROADCAST",
										"NET_RAW", "SETFCAP", "SETGID", "SETPCAP", "SETUID",
										"SYS_ADMIN", "SYS_BOOT", "SYS_CHROOT", "SYS_MODULE",
										"SYS_NICE", "SYS_PACCT", "SYS_RESOURCE", "SYS_TIME",
										"SYS_TTY_CONFIG", "WAKE_ALARM",
									},
								},
							},
						},
					},
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: func() *bool { b := false; return &b }(),
					},
					TerminationGracePeriodSeconds: func() *int64 { t := int64(10); return &t }(),
				},
			},
		},
	}
	
	return daemonSet
}

// daemonSetNeedsUpdate checks if the DaemonSet needs to be updated
func (r *ExecutorDaemonSetReconciler) daemonSetNeedsUpdate(current, desired *appsv1.DaemonSet) bool {
	// Check image
	if len(current.Spec.Template.Spec.Containers) > 0 && len(desired.Spec.Template.Spec.Containers) > 0 {
		if current.Spec.Template.Spec.Containers[0].Image != desired.Spec.Template.Spec.Containers[0].Image {
			return true
		}
	}
	
	// Check NodeSelector
	if !reflect.DeepEqual(current.Spec.Template.Spec.NodeSelector, desired.Spec.Template.Spec.NodeSelector) {
		return true
	}
	
	// Check Tolerations
	if !reflect.DeepEqual(current.Spec.Template.Spec.Tolerations, desired.Spec.Template.Spec.Tolerations) {
		return true
	}
	
	return false
}

// SetupWithManager sets up the controller with the Manager.
func (r *ExecutorDaemonSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&nodecheckv1alpha1.NodeCheck{}).
		Complete(r)
}

