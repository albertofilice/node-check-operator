package controllers

import (
	"context"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"k8s.io/client-go/kubernetes"

	nodecheckv1alpha1 "github.com/albertofilice/node-check-operator/api/v1alpha1"
)

// ConsolePluginReconciler reconciles ConsolePlugin resources
// This controller ensures that Deployment, Service, and ConsolePlugin CR exist
type ConsolePluginReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Clientset kubernetes.Interface
	Namespace string
	Image     string
}

//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=console.openshift.io,resources=consoleplugins,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors;prometheusrules,verbs=get;list;watch;create;update;patch;delete

// Reconcile ensures ConsolePlugin resources exist
func (r *ConsolePluginReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.Log.WithName("ConsolePluginReconciler")
	log.Info("Reconciling ConsolePlugin resources", "request", req)

	// Always ensure ConsolePlugin resources exist (they're part of the operator)
	// We reconcile on any NodeCheck change, Deployment/Service changes, or periodically
	// to ensure resources are up to date

	deploymentName := "node-check-console-plugin"
	serviceName := "node-check-console-plugin"
	consolePluginName := "node-check-console-plugin"
	namespace := r.Namespace
	image := r.Image
	if image == "" {
		image = "quay.io/rh_ee_afilice/node-check-operator-console-plugin:v1.0.7"
	}

	// Reconcile Deployment
	var deployment appsv1.Deployment
	if err := r.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: namespace}, &deployment); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Creating ConsolePlugin Deployment", "name", deploymentName)
			deployment = r.buildDeployment(deploymentName, namespace, image)
			if err := r.Create(ctx, &deployment); err != nil {
				log.Error(err, "unable to create Deployment")
				return ctrl.Result{}, err
			}
			log.Info("Created ConsolePlugin Deployment", "name", deploymentName)
		} else {
			log.Error(err, "unable to fetch Deployment")
			return ctrl.Result{}, err
		}
	} else {
		// Update if needed
		desiredDeployment := r.buildDeployment(deploymentName, namespace, image)
		if r.deploymentNeedsUpdate(&deployment, &desiredDeployment) {
			log.Info("Updating ConsolePlugin Deployment", "name", deploymentName)
			deployment.Spec = desiredDeployment.Spec
			if err := r.Update(ctx, &deployment); err != nil {
				log.Error(err, "unable to update Deployment")
				return ctrl.Result{}, err
			}
			log.Info("Updated ConsolePlugin Deployment", "name", deploymentName)
		}
	}

	// Reconcile ConsolePlugin Service
	var service corev1.Service
	if err := r.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: namespace}, &service); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Creating ConsolePlugin Service", "name", serviceName)
			service = r.buildService(serviceName, namespace)
			if err := r.Create(ctx, &service); err != nil {
				log.Error(err, "unable to create Service")
				return ctrl.Result{}, err
			}
			log.Info("Created ConsolePlugin Service", "name", serviceName)
		} else {
			log.Error(err, "unable to fetch Service")
			return ctrl.Result{}, err
		}
	} else {
		// Update if needed
		desiredService := r.buildService(serviceName, namespace)
		if r.serviceNeedsUpdate(&service, &desiredService) {
			log.Info("Updating ConsolePlugin Service", "name", serviceName)
			service.Spec = desiredService.Spec
			if err := r.Update(ctx, &service); err != nil {
				log.Error(err, "unable to update Service")
				return ctrl.Result{}, err
			}
			log.Info("Updated ConsolePlugin Service", "name", serviceName)
		}
	}

	// Reconcile Dashboard Service (for API proxy)
	dashboardServiceName := "node-check-operator-dashboard"
	var dashboardService corev1.Service
	if err := r.Get(ctx, types.NamespacedName{Name: dashboardServiceName, Namespace: namespace}, &dashboardService); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Creating Dashboard Service", "name", dashboardServiceName)
			dashboardService = r.buildDashboardService(dashboardServiceName, namespace)
			if err := r.Create(ctx, &dashboardService); err != nil {
				log.Error(err, "unable to create Dashboard Service")
				return ctrl.Result{}, err
			}
			log.Info("Created Dashboard Service", "name", dashboardServiceName)
		} else {
			log.Error(err, "unable to fetch Dashboard Service")
			return ctrl.Result{}, err
		}
	} else {
		// Update if needed
		desiredDashboardService := r.buildDashboardService(dashboardServiceName, namespace)
		if r.serviceNeedsUpdate(&dashboardService, &desiredDashboardService) {
			log.Info("Updating Dashboard Service", "name", dashboardServiceName)
			dashboardService.Spec = desiredDashboardService.Spec
			if err := r.Update(ctx, &dashboardService); err != nil {
				log.Error(err, "unable to update Dashboard Service")
				return ctrl.Result{}, err
			}
			log.Info("Updated Dashboard Service", "name", dashboardServiceName)
		}
	}

	// Ensure namespace is labelled for cluster monitoring
	log.Info("Ensuring namespace monitoring label", "namespace", namespace)
	if err := r.ensureNamespaceMonitoringLabel(ctx, namespace, log); err != nil {
		if errors.IsForbidden(err) {
			log.Info("Permission denied for namespace label update, continuing (may need RBAC update)", "error", err.Error())
		} else {
			log.Error(err, "unable to label namespace for cluster monitoring")
			// Don't fail the reconcile if we can't label the namespace
		}
	}

	// Ensure metrics Service, ServiceMonitor and PrometheusRule exist
	log.Info("Ensuring metrics Service", "namespace", namespace)
	if err := r.ensureMetricsService(ctx, namespace, log); err != nil {
		log.Error(err, "unable to ensure metrics Service")
		return ctrl.Result{}, err
	}

	log.Info("Ensuring ServiceMonitor", "namespace", namespace)
	if err := r.ensureServiceMonitor(ctx, namespace, log); err != nil {
		if meta.IsNoMatchError(err) {
			log.Info("ServiceMonitor CRD not installed, skipping creation")
		} else {
			log.Error(err, "unable to ensure ServiceMonitor")
			return ctrl.Result{}, err
		}
	}

	log.Info("Ensuring PrometheusRule", "namespace", namespace)
	if err := r.ensurePrometheusRule(ctx, namespace, log); err != nil {
		if meta.IsNoMatchError(err) {
			log.Info("PrometheusRule CRD not installed, skipping creation")
		} else {
			log.Error(err, "unable to ensure PrometheusRule")
			return ctrl.Result{}, err
		}
	}

	// Reconcile ConsolePlugin CR using unstructured
	consolePluginCR := r.buildConsolePluginCR(consolePluginName, namespace)
	existingCR := &unstructured.Unstructured{}
	existingCR.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "console.openshift.io",
		Version: "v1",
		Kind:    "ConsolePlugin",
	})
	
	if err := r.Get(ctx, types.NamespacedName{Name: consolePluginName}, existingCR); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Creating ConsolePlugin CR", "name", consolePluginName)
			if err := r.Create(ctx, consolePluginCR); err != nil {
				log.Error(err, "unable to create ConsolePlugin CR")
				// Don't fail if ConsolePlugin CR creation fails (might not have permissions)
				log.Info("ConsolePlugin CR creation failed, continuing...")
			} else {
				log.Info("Created ConsolePlugin CR", "name", consolePluginName)
			}
		} else {
			log.Error(err, "unable to fetch ConsolePlugin CR")
			// Don't fail if we can't fetch ConsolePlugin CR
		}
	} else {
		// Update if needed - compare critical fields instead of deep copying
		existingDisplayName, _, _ := unstructured.NestedString(existingCR.Object, "spec", "displayName")
		existingServiceName, _, _ := unstructured.NestedString(existingCR.Object, "spec", "backend", "service", "name")
		existingServiceNamespace, _, _ := unstructured.NestedString(existingCR.Object, "spec", "backend", "service", "namespace")
		existingServicePort, _, _ := unstructured.NestedInt64(existingCR.Object, "spec", "backend", "service", "port")
		
		desiredDisplayName, _, _ := unstructured.NestedString(consolePluginCR.Object, "spec", "displayName")
		desiredServiceName, _, _ := unstructured.NestedString(consolePluginCR.Object, "spec", "backend", "service", "name")
		desiredServiceNamespace, _, _ := unstructured.NestedString(consolePluginCR.Object, "spec", "backend", "service", "namespace")
		desiredServicePort, _, _ := unstructured.NestedInt64(consolePluginCR.Object, "spec", "backend", "service", "port")
		
		needsUpdate := existingDisplayName != desiredDisplayName ||
			existingServiceName != desiredServiceName ||
			existingServiceNamespace != desiredServiceNamespace ||
			existingServicePort != desiredServicePort
		
		if needsUpdate {
			log.Info("Updating ConsolePlugin CR", "name", consolePluginName)
			consolePluginCR.SetResourceVersion(existingCR.GetResourceVersion())
			if err := r.Update(ctx, consolePluginCR); err != nil {
				log.Error(err, "unable to update ConsolePlugin CR")
				// Don't fail if ConsolePlugin CR update fails
			} else {
				log.Info("Updated ConsolePlugin CR", "name", consolePluginName)
			}
		}
	}

	// Requeue periodically to ensure resources stay in sync (every 5 minutes)
	log.Info("ConsolePlugin reconcile completed successfully")
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// buildDeployment creates a Deployment spec for the ConsolePlugin
func (r *ConsolePluginReconciler) buildDeployment(name, namespace, image string) appsv1.Deployment {
	replicas := int32(1)
	return appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app": "node-check-console-plugin",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "node-check-console-plugin",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "node-check-console-plugin",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "node-check-operator-controller-manager",
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser:  func() *int64 { u := int64(0); return &u }(),
						RunAsNonRoot: func() *bool { b := false; return &b }(),
					},
					Volumes: []corev1.Volume{
						{
							Name: "tls-certs",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "node-check-console-plugin-tls",
									Optional:   func() *bool { b := true; return &b }(),
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "plugin",
							Image: image,
							SecurityContext: &corev1.SecurityContext{
								RunAsUser:                func() *int64 { u := int64(0); return &u }(),
								RunAsNonRoot:             func() *bool { b := false; return &b }(),
								AllowPrivilegeEscalation: func() *bool { b := false; return &b }(),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "tls-certs",
									MountPath: "/etc/nginx/ssl",
									ReadOnly:  true,
								},
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 443,
									Name:          "https",
									Protocol:      corev1.ProtocolTCP,
								},
								{
									ContainerPort: 80,
									Name:          "http",
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("64Mi"),
									corev1.ResourceCPU:    resource.MustParse("50m"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("128Mi"),
									corev1.ResourceCPU:    resource.MustParse("100m"),
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/",
										Port:   intstr.FromInt(443),
										Scheme: corev1.URISchemeHTTPS,
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       10,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/",
										Port:   intstr.FromInt(443),
										Scheme: corev1.URISchemeHTTPS,
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       5,
							},
						},
					},
				},
			},
		},
	}
}

// buildService creates a Service spec for the ConsolePlugin
func (r *ConsolePluginReconciler) buildService(name, namespace string) corev1.Service {
	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app": "node-check-console-plugin",
			},
			Annotations: map[string]string{
				"service.beta.openshift.io/serving-cert-secret-name": "node-check-console-plugin-tls",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "node-check-console-plugin",
			},
			Ports: []corev1.ServicePort{
				{
					Port:       443,
					TargetPort: intstr.FromInt(443),
					Protocol:   corev1.ProtocolTCP,
					Name:       "https",
				},
				{
					Port:       80,
					TargetPort: intstr.FromInt(80),
					Protocol:   corev1.ProtocolTCP,
					Name:       "http",
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
}

// deploymentNeedsUpdate checks if the Deployment needs to be updated
func (r *ConsolePluginReconciler) deploymentNeedsUpdate(current, desired *appsv1.Deployment) bool {
	if len(current.Spec.Template.Spec.Containers) == 0 || len(desired.Spec.Template.Spec.Containers) == 0 {
		return true
	}
	if current.Spec.Template.Spec.Containers[0].Image != desired.Spec.Template.Spec.Containers[0].Image {
		return true
	}
	return false
}

// buildDashboardService creates a Service spec for the Dashboard API
func (r *ConsolePluginReconciler) buildDashboardService(name, namespace string) corev1.Service {
	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"control-plane": "controller-manager",
			},
			Annotations: map[string]string{
				"service.beta.openshift.io/serving-cert-secret-name": "node-check-operator-dashboard-tls",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"control-plane": "controller-manager",
			},
			Ports: []corev1.ServicePort{
				{
					Port:       8082,
					TargetPort: intstr.FromInt(8082),
					Protocol:   corev1.ProtocolTCP,
					Name:       "dashboard",
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
}

// serviceNeedsUpdate checks if the Service needs to be updated
func (r *ConsolePluginReconciler) serviceNeedsUpdate(current, desired *corev1.Service) bool {
	// Services rarely need updates, but check ports
	if len(current.Spec.Ports) != len(desired.Spec.Ports) {
		return true
	}
	// Check selector
	if !reflect.DeepEqual(current.Spec.Selector, desired.Spec.Selector) {
		return true
	}
	// Add more comparisons as needed
	return false
}

// buildConsolePluginCR creates a ConsolePlugin CR using unstructured
func (r *ConsolePluginReconciler) buildConsolePluginCR(name, namespace string) *unstructured.Unstructured {
	cr := &unstructured.Unstructured{}
	cr.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "console.openshift.io",
		Version: "v1",
		Kind:    "ConsolePlugin",
	})
	cr.SetName(name)
	
	// Build spec
	spec := map[string]interface{}{
		"displayName": "Node Check Operator",
		"backend": map[string]interface{}{
			"type": "Service",
			"service": map[string]interface{}{
				"name":      "node-check-console-plugin",
				"namespace": namespace,
				"port":      443,
				"basePath":  "/",
			},
		},
		"proxy": []interface{}{
			map[string]interface{}{
				"alias": "api-v1",
				"endpoint": map[string]interface{}{
					"type": "Service",
					"service": map[string]interface{}{
						"name":      "node-check-operator-dashboard",
						"namespace": namespace,
						"port":      8082,
					},
				},
			},
		},
	}
	
	cr.Object["spec"] = spec
	return cr
}

func (r *ConsolePluginReconciler) ensureNamespaceMonitoringLabel(ctx context.Context, namespace string, log logr.Logger) error {
	var ns corev1.Namespace
	if err := r.Get(ctx, types.NamespacedName{Name: namespace}, &ns); err != nil {
		return err
	}

	if ns.Labels == nil {
		ns.Labels = map[string]string{}
	}
	if ns.Labels["openshift.io/cluster-monitoring"] == "true" {
		return nil
	}

	ns.Labels["openshift.io/cluster-monitoring"] = "true"
	if err := r.Update(ctx, &ns); err != nil {
		return err
	}
	log.Info("Namespace labeled for cluster monitoring", "namespace", namespace)
	return nil
}

func (r *ConsolePluginReconciler) ensureMetricsService(ctx context.Context, namespace string, log logr.Logger) error {
	metricsServiceName := "node-check-operator-metrics"
	var service corev1.Service
	if err := r.Get(ctx, types.NamespacedName{Name: metricsServiceName, Namespace: namespace}, &service); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Creating metrics Service", "name", metricsServiceName)
			service = r.buildMetricsService(metricsServiceName, namespace)
			return r.Create(ctx, &service)
		}
		return err
	}

	desired := r.buildMetricsService(metricsServiceName, namespace)
	if r.serviceNeedsUpdate(&service, &desired) {
		log.Info("Updating metrics Service", "name", metricsServiceName)
		service.Spec = desired.Spec
		return r.Update(ctx, &service)
	}
	return nil
}

func (r *ConsolePluginReconciler) buildMetricsService(name, namespace string) corev1.Service {
	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app": "node-check-operator-metrics",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"control-plane": "controller-manager",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "metrics",
					Port:       8080,
					TargetPort: intstr.FromInt(8080),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
}

func (r *ConsolePluginReconciler) ensureServiceMonitor(ctx context.Context, namespace string, log logr.Logger) error {
	serviceMonitorName := "node-check-operator-metrics"
	var sm monitoringv1.ServiceMonitor
	if err := r.Get(ctx, types.NamespacedName{Name: serviceMonitorName, Namespace: namespace}, &sm); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Creating ServiceMonitor", "name", serviceMonitorName)
			sm = r.buildServiceMonitor(serviceMonitorName, namespace)
			return r.Create(ctx, &sm)
		}
		return err
	}

	desired := r.buildServiceMonitor(serviceMonitorName, namespace)
	if r.serviceMonitorNeedsUpdate(&sm, &desired) {
		log.Info("Updating ServiceMonitor", "name", serviceMonitorName)
		sm.Spec = desired.Spec
		return r.Update(ctx, &sm)
	}
	return nil
}

func (r *ConsolePluginReconciler) buildServiceMonitor(name, namespace string) monitoringv1.ServiceMonitor {
	path := "/metrics"
	interval := monitoringv1.Duration("30s")

	return monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app": "node-check-operator-metrics",
			},
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "node-check-operator-metrics",
				},
			},
			Endpoints: []monitoringv1.Endpoint{
				{
					Port:     "metrics",
					Path:     path,
					Interval: interval,
					Scheme:   "http",
				},
			},
		},
	}
}

func (r *ConsolePluginReconciler) serviceMonitorNeedsUpdate(current, desired *monitoringv1.ServiceMonitor) bool {
	if len(current.Spec.Endpoints) != len(desired.Spec.Endpoints) {
		return true
	}
	if len(current.Spec.Endpoints) > 0 {
		if current.Spec.Endpoints[0].Path != desired.Spec.Endpoints[0].Path {
			return true
		}
		if current.Spec.Endpoints[0].Port != desired.Spec.Endpoints[0].Port {
			return true
		}
		if current.Spec.Endpoints[0].Interval != desired.Spec.Endpoints[0].Interval {
			return true
		}
	}
	return !reflect.DeepEqual(current.Spec.Selector.MatchLabels, desired.Spec.Selector.MatchLabels)
}

func (r *ConsolePluginReconciler) ensurePrometheusRule(ctx context.Context, namespace string, log logr.Logger) error {
	ruleName := "node-check-operator-rules"
	var rule monitoringv1.PrometheusRule
	if err := r.Get(ctx, types.NamespacedName{Name: ruleName, Namespace: namespace}, &rule); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Creating PrometheusRule", "name", ruleName)
			rule = r.buildPrometheusRule(ruleName, namespace)
			return r.Create(ctx, &rule)
		}
		return err
	}

	desired := r.buildPrometheusRule(ruleName, namespace)
	if r.prometheusRuleNeedsUpdate(&rule, &desired) {
		log.Info("Updating PrometheusRule", "name", ruleName)
		rule.Spec = desired.Spec
		return r.Update(ctx, &rule)
	}
	return nil
}

func (r *ConsolePluginReconciler) buildPrometheusRule(name, namespace string) monitoringv1.PrometheusRule {
	// Critical alert: fires when at least one node has overall critical status
	criticalExpr := intstr.FromString(`sum(nodecheck_node_status_total{status="Critical"}) > 0`)
	criticalFor := monitoringv1.Duration("5m")
	
	// Warning alert: fires when there are critical checks BUT nodes are not yet critical
	// This acts as an early warning before nodes become critical
	degradedExpr := intstr.FromString(`sum(nodecheck_check_status_total{status="Critical"}) > 0 and sum(nodecheck_node_status_total{status="Critical"}) == 0`)
	degradedFor := monitoringv1.Duration("15m")

	return monitoringv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app": "node-check-operator-metrics",
			},
		},
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{
				{
					Name: "node-check-operator.rules",
					Rules: []monitoringv1.Rule{
						{
							Alert: "NodeCheckCriticalDetected",
							Expr:  criticalExpr,
							For:   &criticalFor,
							Labels: map[string]string{
								"severity": "critical",
							},
							Annotations: map[string]string{
								"summary":     "At least one node has NodeCheck results in critical state",
								"description": "nodecheck_node_status_total reports {{ $value }} nodes with critical status.",
							},
						},
						{
							Alert: "NodeCheckDegradedChecks",
							Expr:  degradedExpr,
							For:   &degradedFor,
							Labels: map[string]string{
								"severity": "warning",
							},
							Annotations: map[string]string{
								"summary":     "Node checks report persistent critical failures",
								"description": "nodecheck_check_status_total reports {{ $value }} critical checks across the fleet.",
							},
						},
					},
				},
			},
		},
	}
}

func (r *ConsolePluginReconciler) prometheusRuleNeedsUpdate(current, desired *monitoringv1.PrometheusRule) bool {
	return !reflect.DeepEqual(current.Spec, desired.Spec)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConsolePluginReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&nodecheckv1alpha1.NodeCheck{}).
		Owns(&appsv1.Deployment{}). // Watch for changes to the console plugin deployment
		Owns(&corev1.Service{}).     // Watch for changes to the console plugin service
		Complete(r)
}

