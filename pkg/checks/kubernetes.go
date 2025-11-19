package checks

import (
	"context"
	"fmt"
	"strings"

	"github.com/albertofilice/node-check-operator/api/v1alpha1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
)

// KubernetesChecker handles Kubernetes-level checks
type KubernetesChecker struct {
	nodeName      string
	client        *kubernetes.Clientset
	dynamicClient dynamic.Interface
	metricsClient *metricsclient.Clientset
	isOpenShift   *bool // Cached OpenShift detection result
}

// isOpenShiftCluster detects if we're running on OpenShift
func (kc *KubernetesChecker) isOpenShiftCluster(ctx context.Context) bool {
	if kc.isOpenShift != nil {
		return *kc.isOpenShift
	}
	
	// Check for OpenShift-specific API group
	gvr := schema.GroupVersionResource{
		Group:    "config.openshift.io",
		Version:  "v1",
		Resource: "clusteroperators",
	}
	
	_, err := kc.dynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{Limit: 1})
	isOpenShift := err == nil
	
	kc.isOpenShift = &isOpenShift
	return isOpenShift
}

// NewKubernetesChecker creates a new Kubernetes checker
func NewKubernetesChecker(nodeName string) (*KubernetesChecker, error) {
	// Try to get in-cluster config first
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", "")
		if err != nil {
			return nil, fmt.Errorf("failed to get Kubernetes config: %v", err)
		}
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %v", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %v", err)
	}

	metricsClient, err := metricsclient.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics client: %v", err)
	}

	return &KubernetesChecker{
		nodeName:      nodeName,
		client:        client,
		dynamicClient: dynamicClient,
		metricsClient: metricsClient,
	}, nil
}

// CheckNodeStatus performs node status monitoring
func (kc *KubernetesChecker) CheckNodeStatus(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	// Get node information
	node, err := kc.client.CoreV1().Nodes().Get(ctx, kc.nodeName, metav1.GetOptions{})
	if err != nil {
		result.Status = "Critical"
		result.Message = fmt.Sprintf("Failed to get node information: %v", err)
	result.Details = mapToRawExtension(details)
		return result
	}

	// Extract node details
	nodeDetails := map[string]interface{}{
		"name":           node.Name,
		"creation_time":  node.CreationTimestamp,
		"labels":         node.Labels,
		"annotations":    node.Annotations,
		"taints":         node.Spec.Taints,
		"unschedulable":  node.Spec.Unschedulable,
	}

	// Check node conditions
	conditions := make(map[string]interface{})
	criticalConditions := []string{}
	warningConditions := []string{}

	for _, condition := range node.Status.Conditions {
		conditionInfo := map[string]interface{}{
			"status":             condition.Status,
			"reason":             condition.Reason,
			"message":            condition.Message,
			"last_heartbeat":     condition.LastHeartbeatTime,
			"last_transition":    condition.LastTransitionTime,
		}
		conditions[string(condition.Type)] = conditionInfo

		// Check for critical conditions
		if condition.Type == "Ready" && condition.Status != "True" {
			criticalConditions = append(criticalConditions, fmt.Sprintf("Node not ready: %s", condition.Message))
		}
		if condition.Type == "MemoryPressure" && condition.Status == "True" {
			warningConditions = append(warningConditions, "Memory pressure detected")
		}
		if condition.Type == "DiskPressure" && condition.Status == "True" {
			warningConditions = append(warningConditions, "Disk pressure detected")
		}
		if condition.Type == "PIDPressure" && condition.Status == "True" {
			warningConditions = append(warningConditions, "PID pressure detected")
		}
	}

	nodeDetails["conditions"] = conditions
	details["node"] = nodeDetails
	details["critical_conditions"] = criticalConditions
	details["warning_conditions"] = warningConditions

	// Check node resources
	if node.Status.Capacity != nil {
		capacity := make(map[string]string)
		for resource, quantity := range node.Status.Capacity {
			capacity[string(resource)] = quantity.String()
		}
		details["capacity"] = capacity
	}

	if node.Status.Allocatable != nil {
		allocatable := make(map[string]string)
		for resource, quantity := range node.Status.Allocatable {
			allocatable[string(resource)] = quantity.String()
		}
		details["allocatable"] = allocatable
	}

	if len(criticalConditions) > 0 {
		result.Status = "Critical"
		result.Message = fmt.Sprintf("Critical node conditions: %s", fmt.Sprintf("%v", criticalConditions))
	} else if len(warningConditions) > 0 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("Warning node conditions: %s", fmt.Sprintf("%v", warningConditions))
	} else {
		result.Status = "Healthy"
		result.Message = "Node status is healthy"
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckPods performs pod monitoring
func (kc *KubernetesChecker) CheckPods(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	// Get all pods on the node
	pods, err := kc.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", kc.nodeName),
	})
	if err != nil {
		result.Status = "Critical"
		result.Message = fmt.Sprintf("Failed to list pods: %v", err)
	result.Details = mapToRawExtension(details)
		return result
	}

	podDetails := make([]map[string]interface{}, 0)
	failedPods := []string{}
	pendingPods := []string{}
	crashLoopPods := []string{}

	for _, pod := range pods.Items {
		podInfo := map[string]interface{}{
			"name":         pod.Name,
			"namespace":    pod.Namespace,
			"phase":        pod.Status.Phase,
			"creation_time": pod.CreationTimestamp,
			"restart_count": 0,
		}

		// Count container restarts
		totalRestarts := int32(0)
		for _, containerStatus := range pod.Status.ContainerStatuses {
			totalRestarts += containerStatus.RestartCount
		}
		podInfo["restart_count"] = totalRestarts

		// Check for failed pods
		if pod.Status.Phase == "Failed" {
			failedPods = append(failedPods, fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
		}

		// Check for pending pods
		if pod.Status.Phase == "Pending" {
			pendingPods = append(pendingPods, fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
		}

		// Check for crash loop backoff
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.State.Waiting != nil && 
			   containerStatus.State.Waiting.Reason == "CrashLoopBackOff" {
				crashLoopPods = append(crashLoopPods, fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
			}
		}

		podDetails = append(podDetails, podInfo)
	}

	details["pods"] = podDetails
	details["total_pods"] = len(pods.Items)
	details["failed_pods"] = failedPods
	details["pending_pods"] = pendingPods
	details["crash_loop_pods"] = crashLoopPods

	if len(failedPods) > 0 {
		result.Status = "Critical"
		result.Message = fmt.Sprintf("Failed pods: %s", fmt.Sprintf("%v", failedPods))
	} else if len(crashLoopPods) > 0 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("Crash loop pods: %s", fmt.Sprintf("%v", crashLoopPods))
	} else if len(pendingPods) > 0 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("Pending pods: %s", fmt.Sprintf("%v", pendingPods))
	} else {
		result.Status = "Healthy"
		result.Message = "All pods are running normally"
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckServices performs service monitoring
func (kc *KubernetesChecker) CheckServices(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	// Get all services
	services, err := kc.client.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		result.Status = "Critical"
		result.Message = fmt.Sprintf("Failed to list services: %v", err)
	result.Details = mapToRawExtension(details)
		return result
	}

	serviceDetails := make([]map[string]interface{}, 0)
	servicesWithoutEndpoints := []string{}

	for _, service := range services.Items {
		serviceInfo := map[string]interface{}{
			"name":         service.Name,
			"namespace":    service.Namespace,
			"type":         service.Spec.Type,
			"cluster_ip":   service.Spec.ClusterIP,
			"external_ips": service.Spec.ExternalIPs,
		}

		// Skip ExternalName services - they don't have endpoints by design
		if service.Spec.Type == "ExternalName" {
			serviceInfo["skip_reason"] = "ExternalName service"
			serviceDetails = append(serviceDetails, serviceInfo)
			continue
		}

		// Skip all headless services (ClusterIP == "None")
		// Headless services are used for StatefulSets and may not have endpoints
		// until pods are ready, or may be used for service discovery without endpoints
		if service.Spec.ClusterIP == "None" {
			serviceInfo["skip_reason"] = "Headless service (ClusterIP=None)"
			serviceDetails = append(serviceDetails, serviceInfo)
			continue
		}

		// Check if service has endpoints using EndpointSlice (v1.33+)
		// Use label selector to find EndpointSlices for this service
		labelSelector := labels.SelectorFromSet(map[string]string{
			discoveryv1.LabelServiceName: service.Name,
		})
		
		endpointSlices, err := kc.client.DiscoveryV1().EndpointSlices(service.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector.String(),
		})
		
		hasEndpoints := false
		totalEndpoints := 0
		if err == nil && len(endpointSlices.Items) > 0 {
			// Count endpoints from all EndpointSlices
			for _, endpointSlice := range endpointSlices.Items {
				for _, endpoint := range endpointSlice.Endpoints {
					if endpoint.Conditions.Ready != nil && *endpoint.Conditions.Ready {
						totalEndpoints += len(endpoint.Addresses)
						hasEndpoints = true
					}
				}
			}
		}
		
		if !hasEndpoints {
			// Only warn if service has a selector (expects endpoints)
			// Headless services are already skipped above, so we only check services with selectors
			shouldWarn := len(service.Spec.Selector) > 0
			
			// If service has a selector, check if there are pods matching it
			// Only warn if there are pods in Running state but no endpoints
			// If no pods exist or all pods are not ready (Pending, Failed, Terminating), it's normal
			if shouldWarn && len(service.Spec.Selector) > 0 {
				selector := labels.SelectorFromSet(service.Spec.Selector)
				pods, err := kc.client.CoreV1().Pods(service.Namespace).List(ctx, metav1.ListOptions{
					LabelSelector: selector.String(),
				})
				if err == nil {
					if len(pods.Items) == 0 {
						// No pods match the selector, so it's normal that there are no endpoints
						shouldWarn = false
						serviceInfo["skip_reason"] = "No pods matching selector"
					} else {
						// Check if there are any pods in Running state
						hasRunningPods := false
						for _, pod := range pods.Items {
							// Skip pods that are being terminated
							if pod.DeletionTimestamp != nil {
								continue
							}
							// Check if pod is in Running state and at least one container is ready
							if pod.Status.Phase == "Running" {
								for _, containerStatus := range pod.Status.ContainerStatuses {
									if containerStatus.Ready {
										hasRunningPods = true
										break
									}
								}
								if hasRunningPods {
									break
								}
							}
						}
						if !hasRunningPods {
							// No running and ready pods, so it's normal that there are no endpoints
							shouldWarn = false
							serviceInfo["skip_reason"] = "No running/ready pods matching selector"
						}
					}
				}
			}
			
			// Ignore services in common operator namespaces
			// These are often used for metrics/health checks and may not have endpoints
			operatorNamespaces := []string{
				"openshift-machine-api",
				"openshift-cluster-version",
				"openshift-kube-apiserver",
				"openshift-kube-controller-manager",
				"openshift-kube-scheduler",
				"openshift-etcd",
				"openshift-network-operator",
				"openshift-ingress-operator",
				"openshift-dns-operator",
				"openshift-cluster-storage-operator",
				"openshift-image-registry",
				"cert-manager-operator",
				"open-cluster-management",
			}
			for _, ns := range operatorNamespaces {
				if service.Namespace == ns {
					shouldWarn = false
					serviceInfo["skip_reason"] = "Operator namespace service"
					break
				}
			}
			
			if shouldWarn {
				servicesWithoutEndpoints = append(servicesWithoutEndpoints, fmt.Sprintf("%s/%s", service.Namespace, service.Name))
			}
		} else {
			serviceInfo["endpoint_count"] = totalEndpoints
		}

		serviceDetails = append(serviceDetails, serviceInfo)
	}

	details["services"] = serviceDetails
	details["total_services"] = len(services.Items)
	details["services_without_endpoints"] = servicesWithoutEndpoints

	if len(servicesWithoutEndpoints) > 0 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("Services without endpoints: %s", fmt.Sprintf("%v", servicesWithoutEndpoints))
	} else {
		result.Status = "Healthy"
		result.Message = "All services have endpoints"
	}

	result.Details = mapToRawExtension(details)
	return result
}


// CheckClusterOperators performs ClusterOperator status monitoring (OpenShift)
func (kc *KubernetesChecker) CheckClusterOperators(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	// ClusterOperator is an OpenShift-specific resource
	// Use dynamic client to access config.openshift.io/v1/ClusterOperator
	gvr := schema.GroupVersionResource{
		Group:    "config.openshift.io",
		Version:  "v1",
		Resource: "clusteroperators",
	}

	operators, err := kc.dynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		// If ClusterOperators are not available (not OpenShift or no permissions), return warning
		result.Status = "Warning"
		result.Message = fmt.Sprintf("ClusterOperators not available: %v (this is normal on non-OpenShift clusters)", err)
		details["error"] = err.Error()
		result.Details = mapToRawExtension(details)
		return result
	}

	operatorDetails := make([]map[string]interface{}, 0)
	unavailableOperators := []string{}
	degradedOperators := []string{}
	progressingOperators := []string{}

	for _, item := range operators.Items {
		operatorInfo := map[string]interface{}{
			"name": item.GetName(),
		}

		// Extract conditions from unstructured object
		conditions, found, err := unstructured.NestedSlice(item.Object, "status", "conditions")
		if err == nil && found {
			operatorInfo["conditions"] = conditions
			
			// Check for Available, Degraded, and Progressing conditions
			for _, cond := range conditions {
				condMap, ok := cond.(map[string]interface{})
				if !ok {
					continue
				}
				
				condType, _ := condMap["type"].(string)
				condStatus, _ := condMap["status"].(string)
				condMessage, _ := condMap["message"].(string)
				
				switch condType {
				case "Available":
					if condStatus != "True" {
						unavailableOperators = append(unavailableOperators, fmt.Sprintf("%s: %s", item.GetName(), condMessage))
					}
				case "Degraded":
					if condStatus == "True" {
						degradedOperators = append(degradedOperators, fmt.Sprintf("%s: %s", item.GetName(), condMessage))
					}
				case "Progressing":
					if condStatus == "True" {
						progressingOperators = append(progressingOperators, fmt.Sprintf("%s: %s", item.GetName(), condMessage))
					}
				}
			}
		}

		operatorDetails = append(operatorDetails, operatorInfo)
	}

	details["operators"] = operatorDetails
	details["total_operators"] = len(operators.Items)
	details["unavailable_operators"] = unavailableOperators
	details["degraded_operators"] = degradedOperators
	details["progressing_operators"] = progressingOperators

	if len(unavailableOperators) > 0 || len(degradedOperators) > 0 {
		result.Status = "Critical"
		messages := []string{}
		if len(unavailableOperators) > 0 {
			messages = append(messages, fmt.Sprintf("Unavailable: %v", unavailableOperators))
		}
		if len(degradedOperators) > 0 {
			messages = append(messages, fmt.Sprintf("Degraded: %v", degradedOperators))
		}
		result.Message = strings.Join(messages, "; ")
	} else if len(progressingOperators) > 0 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("Some operators are progressing: %v", progressingOperators)
	} else {
		result.Status = "Healthy"
		result.Message = "All ClusterOperators are available and not degraded"
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckNodeResources checks node resource allocation (requests, limits, capacity, allocatable)
// and detects overcommit situations. Similar to: oc describe node | grep -A10 Allocated
func (kc *KubernetesChecker) CheckNodeResources(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	// Get node information
	node, err := kc.client.CoreV1().Nodes().Get(ctx, kc.nodeName, metav1.GetOptions{})
	if err != nil {
		result.Status = "Critical"
		result.Message = fmt.Sprintf("Failed to get node information: %v", err)
		result.Details = mapToRawExtension(details)
		return result
	}

	// Get all pods on this node
	pods, err := kc.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", kc.nodeName),
	})
	if err != nil {
		result.Status = "Critical"
		result.Message = fmt.Sprintf("Failed to list pods: %v", err)
		result.Details = mapToRawExtension(details)
		return result
	}

	// Calculate total requests and limits from all pods
	var totalCPURequestsMilli, totalCPULimitsMilli int64
	var totalMemoryRequestsBytes, totalMemoryLimitsBytes int64
	activePodsCount := 0

	for _, pod := range pods.Items {
		// Skip pods that are being terminated
		if pod.DeletionTimestamp != nil {
			continue
		}

		// Skip pods in terminal states (Failed, Succeeded) - same as oc describe node
		// These pods don't consume resources on the node
		if pod.Status.Phase == "Failed" || pod.Status.Phase == "Succeeded" {
			continue
		}

		activePodsCount++

		for _, container := range pod.Spec.Containers {
			// CPU requests and limits
			if cpuRequest := container.Resources.Requests["cpu"]; !cpuRequest.IsZero() {
				totalCPURequestsMilli += cpuRequest.MilliValue()
			}
			if cpuLimit := container.Resources.Limits["cpu"]; !cpuLimit.IsZero() {
				totalCPULimitsMilli += cpuLimit.MilliValue()
			}

			// Memory requests and limits
			if memRequest := container.Resources.Requests["memory"]; !memRequest.IsZero() {
				totalMemoryRequestsBytes += memRequest.Value()
			}
			if memLimit := container.Resources.Limits["memory"]; !memLimit.IsZero() {
				totalMemoryLimitsBytes += memLimit.Value()
			}
		}
	}

	// Get node capacity and allocatable
	nodeCapacityCPU := node.Status.Capacity["cpu"]
	nodeAllocatableCPU := node.Status.Allocatable["cpu"]
	nodeCapacityMemory := node.Status.Capacity["memory"]
	nodeAllocatableMemory := node.Status.Allocatable["memory"]

	// Convert to comparable values
	// Use capacity for percentage calculations (same as oc describe node)
	capacityCPUMilli := nodeCapacityCPU.MilliValue()
	capacityMemoryBytes := nodeCapacityMemory.Value()

	// Calculate percentages based on capacity (same as oc describe node)
	cpuRequestPercent := float64(0)
	cpuLimitPercent := float64(0)
	memoryRequestPercent := float64(0)
	memoryLimitPercent := float64(0)

	if capacityCPUMilli > 0 {
		cpuRequestPercent = float64(totalCPURequestsMilli) / float64(capacityCPUMilli) * 100
		cpuLimitPercent = float64(totalCPULimitsMilli) / float64(capacityCPUMilli) * 100
	}

	if capacityMemoryBytes > 0 {
		memoryRequestPercent = float64(totalMemoryRequestsBytes) / float64(capacityMemoryBytes) * 100
		memoryLimitPercent = float64(totalMemoryLimitsBytes) / float64(capacityMemoryBytes) * 100
	}

	// Build details
	details["node_name"] = node.Name
	details["capacity"] = map[string]interface{}{
		"cpu":    nodeCapacityCPU.String(),
		"memory": nodeCapacityMemory.String(),
	}
	details["allocatable"] = map[string]interface{}{
		"cpu":    nodeAllocatableCPU.String(),
		"memory": nodeAllocatableMemory.String(),
	}
	details["allocated"] = map[string]interface{}{
		"cpu_requests":     fmt.Sprintf("%dm", totalCPURequestsMilli),
		"cpu_limits":       fmt.Sprintf("%dm", totalCPULimitsMilli),
		"memory_requests":  fmt.Sprintf("%d", totalMemoryRequestsBytes),
		"memory_limits":    fmt.Sprintf("%d", totalMemoryLimitsBytes),
	}
	details["percentages"] = map[string]interface{}{
		"cpu_request_percent":    cpuRequestPercent,
		"cpu_limit_percent":      cpuLimitPercent,
		"memory_request_percent": memoryRequestPercent,
		"memory_limit_percent":   memoryLimitPercent,
	}
	details["total_pods"] = activePodsCount

	// Determine status based on overcommit
	hasOvercommit := false
	overcommitMessages := []string{}

	if cpuLimitPercent > 100 {
		hasOvercommit = true
		overcommitMessages = append(overcommitMessages, fmt.Sprintf("CPU limits overcommitted: %.1f%%", cpuLimitPercent))
	}
	if memoryLimitPercent > 100 {
		hasOvercommit = true
		overcommitMessages = append(overcommitMessages, fmt.Sprintf("Memory limits overcommitted: %.1f%%", memoryLimitPercent))
	}

	// Add note to clarify these are allocations, not actual usage
	details["note"] = "These values represent resource ALLOCATIONS (requests/limits) from pod specifications, not actual real-time consumption. Use NodeResourceUsage check for actual consumption metrics."

	// Check if node is underutilized
	isUnderutilized := cpuRequestPercent < 10 && memoryRequestPercent < 10

	if hasOvercommit {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("Resource allocation overcommit detected: %s", strings.Join(overcommitMessages, "; "))
	} else if isUnderutilized {
		result.Status = "Healthy"
		result.Message = fmt.Sprintf("Node allocation is underutilized (CPU requests: %.1f%%, Memory requests: %.1f%%)", cpuRequestPercent, memoryRequestPercent)
	} else if cpuRequestPercent > 90 || memoryRequestPercent > 90 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("High resource allocation (CPU requests: %.1f%%, Memory requests: %.1f%%)", cpuRequestPercent, memoryRequestPercent)
	} else {
		result.Status = "Healthy"
		result.Message = fmt.Sprintf("Resource allocation is normal (CPU requests: %.1f%%, Memory requests: %.1f%%)", cpuRequestPercent, memoryRequestPercent)
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckNodeResourceUsage checks actual real-time CPU and memory consumption using Metrics API
// This provides actual usage metrics, unlike CheckNodeResources which shows allocations
func (kc *KubernetesChecker) CheckNodeResourceUsage(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	// Detect if we're on OpenShift (which has metrics-server built-in)
	isOpenShift := kc.isOpenShiftCluster(ctx)
	details["is_openshift"] = isOpenShift
	
	metricsObj, err := kc.metricsClient.MetricsV1beta1().NodeMetricses().Get(ctx, kc.nodeName, metav1.GetOptions{})
	if err != nil {
		// Metrics API not available
		result.Status = "Warning"
		if isOpenShift {
			result.Message = fmt.Sprintf("Metrics API not available: %v (OpenShift should have metrics-server by default, check if it's running)", err)
			details["note"] = "OpenShift includes metrics-server by default. If this check fails, verify that the metrics-server pods are running in the openshift-monitoring namespace."
		} else {
			result.Message = fmt.Sprintf("Metrics API not available: %v (metrics-server may not be installed or accessible)", err)
			details["note"] = "Node resource usage requires metrics-server to be installed. Install with: kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml"
		}
		details["error"] = err.Error()
		result.Details = mapToRawExtension(details)
		return result
	}

	// Parse CPU and memory from the response
	// Usage is returned as strings like "5895m" for CPU and "43404Mi" for memory
	cpuUsageQuantity, cpuOk := metricsObj.Usage["cpu"]
	memoryUsageQuantity, memoryOk := metricsObj.Usage["memory"]

	if !cpuOk || !memoryOk {
		result.Status = "Warning"
		result.Message = "Failed to parse metrics response (unexpected format)"
		details["error"] = "Metrics API returned unexpected format"
		result.Details = mapToRawExtension(details)
		return result
	}

	cpuUsage := cpuUsageQuantity.DeepCopy()
	memoryUsage := memoryUsageQuantity.DeepCopy()

	// Get node capacity for percentage calculations
	node, err := kc.client.CoreV1().Nodes().Get(ctx, kc.nodeName, metav1.GetOptions{})
	if err != nil {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("Failed to get node information: %v", err)
		details["error"] = err.Error()
		result.Details = mapToRawExtension(details)
		return result
	}

	nodeCapacityCPU := node.Status.Capacity["cpu"]
	nodeCapacityMemory := node.Status.Capacity["memory"]

	// Calculate percentages
	cpuUsageMilli := cpuUsage.MilliValue()
	memoryUsageBytes := memoryUsage.Value()
	capacityCPUMilli := nodeCapacityCPU.MilliValue()
	capacityMemoryBytes := nodeCapacityMemory.Value()

	cpuUsagePercent := float64(0)
	memoryUsagePercent := float64(0)

	if capacityCPUMilli > 0 {
		cpuUsagePercent = float64(cpuUsageMilli) / float64(capacityCPUMilli) * 100
	}
	if capacityMemoryBytes > 0 {
		memoryUsagePercent = float64(memoryUsageBytes) / float64(capacityMemoryBytes) * 100
	}

	// Build details
	details["node_name"] = kc.nodeName
	details["cpu_usage"] = map[string]interface{}{
		"cores":    fmt.Sprintf("%dm", cpuUsageMilli),
		"percent":  cpuUsagePercent,
		"capacity": nodeCapacityCPU.String(),
	}
	details["memory_usage"] = map[string]interface{}{
		"bytes":    memoryUsageBytes,
		"human":    memoryUsage.String(),
		"percent":  memoryUsagePercent,
		"capacity": nodeCapacityMemory.String(),
	}
	details["check_method"] = "Metrics API (metrics-server)"
	details["note"] = "These values represent ACTUAL real-time consumption, not allocations. Compare with NodeResources check to see allocation vs usage."

	// Determine status based on actual usage
	issues := []string{}
	if cpuUsagePercent > 90 {
		issues = append(issues, fmt.Sprintf("High CPU usage: %.1f%%", cpuUsagePercent))
	}
	if memoryUsagePercent > 90 {
		issues = append(issues, fmt.Sprintf("High memory usage: %.1f%%", memoryUsagePercent))
	}

	if len(issues) > 0 {
		result.Status = "Warning"
		result.Message = strings.Join(issues, "; ")
	} else if cpuUsagePercent > 80 || memoryUsagePercent > 80 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("Elevated resource usage (CPU: %.1f%%, Memory: %.1f%%)", cpuUsagePercent, memoryUsagePercent)
	} else {
		result.Status = "Healthy"
		result.Message = fmt.Sprintf("Resource usage is normal (CPU: %.1f%%, Memory: %.1f%%)", cpuUsagePercent, memoryUsagePercent)
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckContainerRuntime checks container runtime health
func (kc *KubernetesChecker) CheckContainerRuntime(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	// Check containerd socket
	containerdSocket := "/run/containerd/containerd.sock"
	output, err := runHostCommand(ctx, fmt.Sprintf("test -S %s && echo exists || echo not_found", containerdSocket))
	if err == nil && strings.Contains(string(output), "exists") {
		details["runtime"] = "containerd"
		details["socket"] = containerdSocket
		result.Status = "Healthy"
		result.Message = "Container runtime (containerd) is available"
		result.Details = mapToRawExtension(details)
		return result
	}

	// Check CRI-O socket
	crioSocket := "/var/run/crio/crio.sock"
	output, err = runHostCommand(ctx, fmt.Sprintf("test -S %s && echo exists || echo not_found", crioSocket))
	if err == nil && strings.Contains(string(output), "exists") {
		details["runtime"] = "crio"
		details["socket"] = crioSocket
		result.Status = "Healthy"
		result.Message = "Container runtime (CRI-O) is available"
		result.Details = mapToRawExtension(details)
		return result
	}

	// Check Docker socket
	dockerSocket := "/var/run/docker.sock"
	output, err = runHostCommand(ctx, fmt.Sprintf("test -S %s && echo exists || echo not_found", dockerSocket))
	if err == nil && strings.Contains(string(output), "exists") {
		details["runtime"] = "docker"
		details["socket"] = dockerSocket
		result.Status = "Warning"
		result.Message = "Docker runtime detected (may be deprecated in newer Kubernetes versions)"
		result.Details = mapToRawExtension(details)
		return result
	}

	result.Status = "Warning"
	result.Message = "Container runtime socket not found or not accessible"
	details["note"] = "Container runtime socket may not be accessible from the container"
	result.Details = mapToRawExtension(details)
	return result
}

// CheckKubeletHealth checks kubelet health endpoint
func (kc *KubernetesChecker) CheckKubeletHealth(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	// Check kubelet health endpoint (usually on port 10248)
	output, err := runHostCommand(ctx, "curl -s -k https://localhost:10248/healthz 2>&1 || curl -s http://localhost:10248/healthz 2>&1")
	if err == nil {
		healthResponse := strings.TrimSpace(string(output))
		details["health_response"] = healthResponse
		
		if healthResponse == "ok" {
			result.Status = "Healthy"
			result.Message = "Kubelet health endpoint is responding"
		} else {
			result.Status = "Warning"
			result.Message = fmt.Sprintf("Kubelet health endpoint returned: %s", healthResponse)
		}
	} else {
		result.Status = "Warning"
		result.Message = "Kubelet health endpoint not accessible"
		details["note"] = "Kubelet health endpoint may not be accessible from the container"
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckCNIPlugin checks CNI plugin status
func (kc *KubernetesChecker) CheckCNIPlugin(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	// Check CNI config directory
	cniConfigDir := "/etc/cni/net.d"
	output, err := runHostCommand(ctx, fmt.Sprintf("ls %s 2>/dev/null | head -5", cniConfigDir))
	if err == nil && len(output) > 0 {
		details["cni_config_dir"] = cniConfigDir
		details["cni_configs"] = strings.Split(strings.TrimSpace(string(output)), "\n")
	}

	// Check CNI bin directory
	cniBinDir := "/opt/cni/bin"
	output, err = runHostCommand(ctx, fmt.Sprintf("ls %s 2>/dev/null | head -10", cniBinDir))
	if err == nil && len(output) > 0 {
		details["cni_bin_dir"] = cniBinDir
		details["cni_binaries"] = strings.Split(strings.TrimSpace(string(output)), "\n")
	}

	if len(details) > 0 {
		result.Status = "Healthy"
		result.Message = "CNI plugin configuration found"
	} else {
		result.Status = "Warning"
		result.Message = "CNI plugin configuration not found or not accessible"
		details["note"] = "CNI directories may not be accessible from the container"
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckNodeConditions checks node conditions in detail
func (kc *KubernetesChecker) CheckNodeConditions(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	node, err := kc.client.CoreV1().Nodes().Get(ctx, kc.nodeName, metav1.GetOptions{})
	if err != nil {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("Failed to get node: %v", err)
		result.Details = mapToRawExtension(details)
		return result
	}

	conditions := make(map[string]interface{})
	unhealthyConditions := []string{}

	for _, condition := range node.Status.Conditions {
		conditionInfo := map[string]interface{}{
			"status":             string(condition.Status),
			"lastTransitionTime": condition.LastTransitionTime.Format("2006-01-02 15:04:05"),
			"reason":             condition.Reason,
			"message":            condition.Message,
		}
		conditions[string(condition.Type)] = conditionInfo

		// Check for unhealthy conditions
		// Note: For pressure conditions (MemoryPressure, DiskPressure, PIDPressure),
		// "False" status means healthy (no pressure), "True" means unhealthy (pressure exists)
		// For Ready condition, "True" means healthy
		if condition.Type == "Ready" {
			// Ready should be True for healthy node
			if condition.Status != "True" {
				unhealthyConditions = append(unhealthyConditions, string(condition.Type))
			}
		} else {
			// For other conditions (pressure conditions), True means unhealthy
			if condition.Status == "True" {
				unhealthyConditions = append(unhealthyConditions, string(condition.Type))
			}
		}
	}

	details["conditions"] = conditions
	details["node_name"] = kc.nodeName

	if len(unhealthyConditions) > 0 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("Unhealthy node conditions: %s", strings.Join(unhealthyConditions, ", "))
	} else {
		result.Status = "Healthy"
		result.Message = "All node conditions are healthy"
	}

	result.Details = mapToRawExtension(details)
	return result
}

