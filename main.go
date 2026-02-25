package main

import (
	"context"
	"flag"
	"os"
	"strings"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	nodecheckv1alpha1 "github.com/albertofilice/node-check-operator/api/v1alpha1"
	"github.com/albertofilice/node-check-operator/controllers"
	"github.com/albertofilice/node-check-operator/pkg/dashboard"
	_ "github.com/albertofilice/node-check-operator/pkg/metrics" // Import to initialize metrics
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/client-go/kubernetes"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(monitoringv1.AddToScheme(scheme))
	
	// Register types with SchemeBuilder first (required for AddToScheme)
	nodecheckv1alpha1.SchemeBuilder.Register(
		&nodecheckv1alpha1.NodeCheck{},
		&nodecheckv1alpha1.NodeCheckList{},
	)
	
	// Add the types to the scheme using AddToScheme
	utilruntime.Must(nodecheckv1alpha1.AddToScheme(scheme))
	
	// Also register directly using AddKnownTypes as a fallback
	scheme.AddKnownTypes(nodecheckv1alpha1.GroupVersion,
		&nodecheckv1alpha1.NodeCheck{},
		&nodecheckv1alpha1.NodeCheckList{},
	)
	
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var mode string
	var enableOpenShiftFeatures bool = true
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":31680", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":31681", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&mode, "mode", "operator", "Mode to run in: 'operator' (manages resources) or 'executor' (executes checks)")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	if val := strings.ToLower(os.Getenv("ENABLE_OPENSHIFT_FEATURES")); val != "" {
		switch val {
		case "false", "0", "no", "disabled":
			enableOpenShiftFeatures = false
		default:
			enableOpenShiftFeatures = true
		}
	}
	
	if mode != "operator" && mode != "executor" {
		setupLog.Error(nil, "Invalid mode", "mode", mode, "validModes", []string{"operator", "executor"})
		os.Exit(1)
	}
	setupLog.Info("Starting in mode", "mode", mode)
	setupLog.Info("OpenShift integrations enabled", "enabled", enableOpenShiftFeatures)

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Verify that NodeCheck type is registered in the scheme
	nodeCheckGVK, _, err := scheme.ObjectKinds(&nodecheckv1alpha1.NodeCheck{})
	if err != nil || len(nodeCheckGVK) == 0 {
		setupLog.Error(err, "NodeCheck type is not registered in scheme")
		os.Exit(1)
	}
	setupLog.Info("NodeCheck type registered in scheme", "gvk", nodeCheckGVK[0])

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "node-check-operator.openshift.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Verify the manager's scheme also has the type
	managerScheme := mgr.GetScheme()
	managerGVK, _, err := managerScheme.ObjectKinds(&nodecheckv1alpha1.NodeCheck{})
	if err != nil || len(managerGVK) == 0 {
		setupLog.Error(err, "NodeCheck type is not registered in manager scheme")
		os.Exit(1)
	}
	setupLog.Info("NodeCheck type registered in manager scheme", "gvk", managerGVK[0])

	// Create a test instance to verify it can get its GVK
	testNodeCheck := &nodecheckv1alpha1.NodeCheck{}
	testGVK, _, testErr := managerScheme.ObjectKinds(testNodeCheck)
	if testErr != nil || len(testGVK) == 0 {
		setupLog.Error(testErr, "Cannot get GVK for NodeCheck instance", "gvks", testGVK)
		os.Exit(1)
	}
	setupLog.Info("Test NodeCheck instance GVK", "gvk", testGVK[0])

	// Try to get the type from the scheme by GVK
	obj, err := managerScheme.New(nodecheckv1alpha1.GroupVersion.WithKind("NodeCheck"))
	if err != nil {
		setupLog.Error(err, "Cannot create new NodeCheck from scheme by GVK")
		os.Exit(1)
	}
	setupLog.Info("Successfully created NodeCheck from scheme", "type", obj.GetObjectKind().GroupVersionKind())

	// Create Kubernetes clientset for dashboard and checks
	config := ctrl.GetConfigOrDie()
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		setupLog.Error(err, "unable to create Kubernetes clientset")
		os.Exit(1)
	}

	// Create Dashboard Service immediately (only in operator mode with OpenShift features)
	// This ensures the Service exists before the dashboard server starts, allowing
	// the Service Serving Certificate Signer to create the TLS secret
	if mode == "operator" && enableOpenShiftFeatures {
		namespace := "node-check-operator-system"
		serviceName := "node-check-operator-dashboard"
		
		setupLog.Info("Ensuring Dashboard Service exists", "service", serviceName, "namespace", namespace)
		
		// Create Service directly using the clientset (works before manager starts)
		ctx := context.Background()
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
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
						Port:       31682,
						TargetPort: intstr.FromInt(31682),
						Protocol:   corev1.ProtocolTCP,
						Name:       "dashboard",
					},
				},
				Type: corev1.ServiceTypeClusterIP,
			},
		}
		
		// Try to get the Service first
		_, err := clientset.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				// Service doesn't exist, create it
				_, err = clientset.CoreV1().Services(namespace).Create(ctx, service, metav1.CreateOptions{})
				if err != nil {
					setupLog.Error(err, "unable to create Dashboard Service")
					// Don't exit - the ConsolePlugin controller will create it later
				} else {
					setupLog.Info("Created Dashboard Service", "service", serviceName)
				}
			} else {
				setupLog.Error(err, "unable to check Dashboard Service")
			}
		} else {
			setupLog.Info("Dashboard Service already exists", "service", serviceName)
		}
		
		// Start dashboard server asynchronously after a short delay
		// This gives the Service Serving Certificate Signer time to create the secret
		dashboardServer := dashboard.NewDashboardServer(mgr.GetClient(), clientset, namespace, 31682)
		go func() {
			setupLog.Info("Waiting for TLS certificates to be created by Service Serving Certificate Signer", "waitSeconds", 15)
			time.Sleep(15 * time.Second) // Wait for Service Serving Certificate Signer
			
			setupLog.Info("Starting dashboard server", "port", 31682)
			if err := dashboardServer.Start(); err != nil {
				setupLog.Error(err, "unable to start dashboard server")
				// Don't exit - the operator can still function without the dashboard
			} else {
				setupLog.Info("Dashboard server started successfully", "port", 31682)
			}
		}()
	} else if mode == "operator" {
		setupLog.Info("OpenShift-specific features disabled; skipping dashboard server")
	}

	// Get namespace from environment or use default
	namespace := os.Getenv("WATCH_NAMESPACE")
	if namespace == "" {
		namespace = "node-check-operator-system"
	}

	// Get image from environment or use default
	operatorImage := os.Getenv("OPERATOR_IMAGE")
	if operatorImage == "" {
		operatorImage = "quay.io/rh_ee_afilice/node-check-operator:v1.0.8"
	}
	consolePluginImage := os.Getenv("CONSOLE_PLUGIN_IMAGE")
	if consolePluginImage == "" {
		consolePluginImage = "quay.io/rh_ee_afilice/node-check-operator-console-plugin:v1.0.8"
	}

	// Setup controllers based on mode
	if mode == "operator" {
		// Operator mode: manages resources, creates child NodeChecks, manages DaemonSet and ConsolePlugin
		if err = (&controllers.NodeCheckReconciler{
			Client:    mgr.GetClient(),
			Scheme:    managerScheme,
			Clientset: clientset,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "NodeCheck")
			os.Exit(1)
		}
		
		// Controller for executor DaemonSet
		if err = (&controllers.ExecutorDaemonSetReconciler{
			Client:    mgr.GetClient(),
			Scheme:    managerScheme,
			Clientset: clientset,
			Namespace: namespace,
			Image:     operatorImage,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "ExecutorDaemonSet")
			os.Exit(1)
		}
		
		// Controller for ConsolePlugin resources (only when OpenShift features are enabled)
		if enableOpenShiftFeatures {
			consolePluginReconciler := &controllers.ConsolePluginReconciler{
				Client:    mgr.GetClient(),
				Scheme:    managerScheme,
				Clientset: clientset,
				Namespace: namespace,
				Image:     consolePluginImage,
			}
			if err = consolePluginReconciler.SetupWithManager(mgr); err != nil {
				setupLog.Error(err, "unable to create controller", "controller", "ConsolePlugin")
				os.Exit(1)
			}
			
			// Trigger initial reconcile for ConsolePlugin resources on startup
			// This ensures resources are created even if no NodeCheck exists yet
			go func() {
				// Wait for manager to be ready
				time.Sleep(2 * time.Second)
				setupLog.Info("Triggering initial ConsolePlugin reconcile on startup")
				ctx := context.Background()
				// Create a dummy request to trigger reconcile
				req := ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name:      "startup-trigger",
						Namespace: namespace,
					},
				}
				// This will fail to get the NodeCheck, but the reconcile will still create resources
				_, _ = consolePluginReconciler.Reconcile(ctx, req)
			}()
		} else {
			setupLog.Info("OpenShift-specific features disabled; skipping ConsolePlugin controller registration")
		}
		
	} else if mode == "executor" {
		// Executor mode: only executes checks
		if err = (&controllers.NodeCheckExecutorReconciler{
			Client:    mgr.GetClient(),
			Scheme:    managerScheme,
			Clientset: clientset,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "NodeCheckExecutor")
			os.Exit(1)
		}
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
