package controllers

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/api/core/v1"

	nodecheckv1alpha1 "github.com/albertofilice/node-check-operator/api/v1alpha1"
)

// NodeCheckReconciler reconciles a NodeCheck object
type NodeCheckReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Clientset kubernetes.Interface
}

//+kubebuilder:rbac:groups=nodecheck.openshift.io,resources=nodechecks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=nodecheck.openshift.io,resources=nodechecks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=nodecheck.openshift.io,resources=nodechecks/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=resourcequotas,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// This controller only manages NodeCheck resources (creates/updates child resources),
// it does NOT execute checks (that's done by NodeCheckExecutorReconciler).
func (r *NodeCheckReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.Log.WithName("NodeCheckReconciler")

	// Fetch the NodeCheck instance
	var nodeCheck nodecheckv1alpha1.NodeCheck
	if err := r.Get(ctx, req.NamespacedName, &nodeCheck); err != nil {
		log.Error(err, "unable to fetch NodeCheck")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	nodeName := nodeCheck.Spec.NodeName
	
	// If nodeName is "*" or "all", create/update child NodeChecks for each matching node
	if nodeName == "*" || nodeName == "all" {
		log.Info("Processing NodeCheck with nodeName='*' - creating/updating child resources", "nodeCheck", req.Name)
		return r.reconcileAllNodesMode(ctx, req, nodeCheck)
	}
	
	// If nodeName is empty, the executor will auto-detect the node (no action needed here)
	if nodeName == "" {
		log.Info("NodeCheck with empty nodeName - executor will auto-detect node", "nodeCheck", req.Name)
		return ctrl.Result{}, nil
	}
	
	// For specific node names, just ensure the NodeCheck exists (no action needed)
	// The executor will handle the checks
	log.Info("NodeCheck is for specific node, no resource management needed", "nodeCheck", req.Name, "nodeName", nodeName)
	return ctrl.Result{}, nil
}

// reconcileAllNodesMode handles NodeChecks with nodeName="*" by creating/updating
// child NodeChecks for each node in the cluster that matches the NodeSelector (if specified)
func (r *NodeCheckReconciler) reconcileAllNodesMode(ctx context.Context, req ctrl.Request, templateNodeCheck nodecheckv1alpha1.NodeCheck) (ctrl.Result, error) {
	log := ctrl.Log.WithName("NodeCheckReconciler")
	
	// Get all nodes in the cluster
	var nodes corev1.NodeList
	if err := r.Client.List(ctx, &nodes); err != nil {
		log.Error(err, "unable to list nodes")
		return ctrl.Result{}, err
	}
	
	log.Info("Found nodes in cluster", "nodeCount", len(nodes.Items))
	
	// Filter nodes by NodeSelector if specified
	filteredNodes := []corev1.Node{}
	for _, node := range nodes.Items {
		matches := true
		
		// Check if node matches NodeSelector
		if len(templateNodeCheck.Spec.NodeSelector) > 0 {
			for key, value := range templateNodeCheck.Spec.NodeSelector {
				if nodeValue, exists := node.Labels[key]; !exists || nodeValue != value {
					matches = false
					break
				}
			}
		}
		
		if matches {
			filteredNodes = append(filteredNodes, node)
		}
	}
	
	log.Info("Filtered nodes by NodeSelector", "totalNodes", len(nodes.Items), "matchingNodes", len(filteredNodes), "nodeSelector", templateNodeCheck.Spec.NodeSelector)
	
	// Build a set of matching node names for cleanup
	matchingNodeNames := make(map[string]bool)
	for _, node := range filteredNodes {
		matchingNodeNames[node.Name] = true
	}
	
	// Find and delete child NodeChecks for nodes that no longer match the selector
	var existingChildNodeChecks nodecheckv1alpha1.NodeCheckList
	// We identify child NodeChecks by name prefix (format: templateName-nodeName)
	if err := r.Client.List(ctx, &existingChildNodeChecks, client.InNamespace(req.Namespace)); err == nil {
		for _, child := range existingChildNodeChecks.Items {
			// Check if this is a child of the current template (name starts with template name + "-")
			if len(child.Name) > len(req.Name)+1 && child.Name[:len(req.Name)+1] == req.Name+"-" {
				// Extract node name from child name (format: templateName-nodeName)
				childNodeName := child.Name[len(req.Name)+1:]
				// If this node is not in the matching set, delete the child NodeCheck
				if !matchingNodeNames[childNodeName] {
					log.Info("Deleting child NodeCheck for node that no longer matches selector", 
						"childNodeCheckName", child.Name, "nodeName", childNodeName)
					if err := r.Delete(ctx, &child); err != nil {
						log.Error(err, "unable to delete orphaned child NodeCheck", "childNodeCheckName", child.Name)
					}
				}
			}
		}
	}
	
	// For each matching node, ensure a child NodeCheck exists and is in sync with the template
	for _, node := range filteredNodes {
		nodeName := node.Name
		childNodeCheckName := fmt.Sprintf("%s-%s", req.Name, nodeName)
		
		var childNodeCheck nodecheckv1alpha1.NodeCheck
		if err := r.Get(ctx, types.NamespacedName{Name: childNodeCheckName, Namespace: req.Namespace}, &childNodeCheck); err != nil {
			if client.IgnoreNotFound(err) != nil {
				log.Error(err, "unable to get child NodeCheck", "childNodeCheckName", childNodeCheckName)
				continue
			}
			// Child doesn't exist, create it
			childNodeCheck = templateNodeCheck
			childNodeCheck.Name = childNodeCheckName
			childNodeCheck.ResourceVersion = ""
			childNodeCheck.UID = ""
			childNodeCheck.Spec.NodeName = nodeName
			// Remove NodeSelector from child (it's already for a specific node)
			childNodeCheck.Spec.NodeSelector = nil
			// Keep Tolerations (they may be needed for the DaemonSet)
			childNodeCheck.Status = nodecheckv1alpha1.NodeCheckStatus{}
			
			if err := r.Create(ctx, &childNodeCheck); err != nil {
				log.Error(err, "unable to create child NodeCheck", "childNodeCheckName", childNodeCheckName)
				continue
			}
			log.Info("Created child NodeCheck", "childNodeCheckName", childNodeCheckName, "node", nodeName)
			continue
		}
		
		// Child exists, check if we need to sync the spec from the template
		needsUpdate := false
		if childNodeCheck.Spec.CheckInterval != templateNodeCheck.Spec.CheckInterval {
			childNodeCheck.Spec.CheckInterval = templateNodeCheck.Spec.CheckInterval
			needsUpdate = true
		}
		if !r.specsEqual(childNodeCheck.Spec.SystemChecks, templateNodeCheck.Spec.SystemChecks) {
			childNodeCheck.Spec.SystemChecks = templateNodeCheck.Spec.SystemChecks
			needsUpdate = true
		}
		if !r.kubernetesChecksEqual(childNodeCheck.Spec.KubernetesChecks, templateNodeCheck.Spec.KubernetesChecks) {
			childNodeCheck.Spec.KubernetesChecks = templateNodeCheck.Spec.KubernetesChecks
			needsUpdate = true
		}
		if childNodeCheck.Spec.NodeName != nodeName {
			childNodeCheck.Spec.NodeName = nodeName
			needsUpdate = true
		}
		// Sync Tolerations from template (NodeSelector is not synced as child is for specific node)
		if !reflect.DeepEqual(childNodeCheck.Spec.Tolerations, templateNodeCheck.Spec.Tolerations) {
			childNodeCheck.Spec.Tolerations = templateNodeCheck.Spec.Tolerations
			needsUpdate = true
		}
		// Ensure NodeSelector is nil for child (it's for a specific node)
		if len(childNodeCheck.Spec.NodeSelector) > 0 {
			childNodeCheck.Spec.NodeSelector = nil
			needsUpdate = true
		}
		
		if needsUpdate {
			// Update with retry logic for conflict errors
			maxRetries := 3
			updateSuccess := false
			for i := 0; i < maxRetries; i++ {
				if err := r.Update(ctx, &childNodeCheck); err != nil {
					if errors.IsConflict(err) {
						if i < maxRetries-1 {
							log.Info("Conflict updating child NodeCheck, retrying...", 
								"attempt", i+1, "maxRetries", maxRetries, "childNodeCheckName", childNodeCheckName)
							// Fetch latest version
							if err := r.Get(ctx, types.NamespacedName{Name: childNodeCheckName, Namespace: req.Namespace}, &childNodeCheck); err != nil {
								log.Error(err, "unable to fetch child NodeCheck for retry")
								break
							}
							// Re-apply spec changes
							if childNodeCheck.Spec.CheckInterval != templateNodeCheck.Spec.CheckInterval {
								childNodeCheck.Spec.CheckInterval = templateNodeCheck.Spec.CheckInterval
							}
							if !r.specsEqual(childNodeCheck.Spec.SystemChecks, templateNodeCheck.Spec.SystemChecks) {
								childNodeCheck.Spec.SystemChecks = templateNodeCheck.Spec.SystemChecks
							}
							if !r.kubernetesChecksEqual(childNodeCheck.Spec.KubernetesChecks, templateNodeCheck.Spec.KubernetesChecks) {
								childNodeCheck.Spec.KubernetesChecks = templateNodeCheck.Spec.KubernetesChecks
							}
							if childNodeCheck.Spec.NodeName != nodeName {
								childNodeCheck.Spec.NodeName = nodeName
							}
							if !reflect.DeepEqual(childNodeCheck.Spec.Tolerations, templateNodeCheck.Spec.Tolerations) {
								childNodeCheck.Spec.Tolerations = templateNodeCheck.Spec.Tolerations
							}
							if len(childNodeCheck.Spec.NodeSelector) > 0 {
								childNodeCheck.Spec.NodeSelector = nil
							}
							time.Sleep(time.Millisecond * 100 * time.Duration(i+1))
							continue
						}
					}
					log.Error(err, "unable to update child NodeCheck", "childNodeCheckName", childNodeCheckName)
			break
				}
				updateSuccess = true
				break
			}
			
			if updateSuccess {
				log.Info("Updated child NodeCheck spec from template", "childNodeCheckName", childNodeCheckName, "node", nodeName)
			}
		}
	}
	
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// specsEqual compares two SystemChecks structs for equality
func (r *NodeCheckReconciler) specsEqual(a, b nodecheckv1alpha1.SystemChecks) bool {
	return reflect.DeepEqual(a, b)
}

// kubernetesChecksEqual compares two KubernetesChecks structs for equality
func (r *NodeCheckReconciler) kubernetesChecksEqual(a, b nodecheckv1alpha1.KubernetesChecks) bool {
	return reflect.DeepEqual(a, b)
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeCheckReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&nodecheckv1alpha1.NodeCheck{}).
		Complete(r)
}
