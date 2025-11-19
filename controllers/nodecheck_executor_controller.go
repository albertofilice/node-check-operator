package controllers

import (
	"context"
	"fmt"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"k8s.io/client-go/kubernetes"

	nodecheckv1alpha1 "github.com/albertofilice/node-check-operator/api/v1alpha1"
	"github.com/albertofilice/node-check-operator/pkg/checks"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeCheckExecutorReconciler reconciles NodeCheck objects by executing checks
// This controller runs in DaemonSet mode and only executes checks for NodeChecks
// that match the current node. It does NOT create or manage NodeCheck resources.
type NodeCheckExecutorReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Clientset kubernetes.Interface
}

//+kubebuilder:rbac:groups=nodecheck.openshift.io,resources=nodechecks,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=nodecheck.openshift.io,resources=nodechecks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=resourcequotas,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch
//+kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch

// Reconcile executes checks for NodeCheck resources that match the current node
func (r *NodeCheckExecutorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.Log.WithName("NodeCheckExecutor")

	// Fetch the NodeCheck instance
	var nodeCheck nodecheckv1alpha1.NodeCheck
	if err := r.Get(ctx, req.NamespacedName, &nodeCheck); err != nil {
		log.Error(err, "unable to fetch NodeCheck")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Get the node name where this pod is running
	currentNodeName := os.Getenv("NODE_NAME")
	if currentNodeName == "" {
		log.Error(nil, "NODE_NAME environment variable not set")
		return ctrl.Result{}, fmt.Errorf("NODE_NAME environment variable not set")
	}

	// Handle auto-detection of nodeName: if nodeName is empty, auto-detect and update the NodeCheck
	nodeName := nodeCheck.Spec.NodeName
	if nodeName == "" {
		log.Info("NodeName not specified, auto-detecting from pod node", "currentNode", currentNodeName)
		// Update the NodeCheck spec with the detected node name
		nodeCheck.Spec.NodeName = currentNodeName
		if err := r.Update(ctx, &nodeCheck); err != nil {
			log.Error(err, "unable to update NodeCheck with auto-detected nodeName")
			return ctrl.Result{}, err
		}
		log.Info("Updated NodeCheck with auto-detected nodeName", "nodeName", currentNodeName)
		nodeName = currentNodeName
	}

	// Only process NodeChecks that match this node (skip "*" and "all" as they are handled by main controller)
	if nodeName == "*" || nodeName == "all" {
		log.Info("Skipping NodeCheck - wildcard nodeName handled by main controller", 
			"nodeCheckNode", nodeName)
		return ctrl.Result{}, nil
	}

	if nodeName != currentNodeName {
		// This NodeCheck is not for this node, skip
		log.Info("Skipping NodeCheck - not for this node", 
			"nodeCheckNode", nodeName, 
			"currentNode", currentNodeName)
		return ctrl.Result{}, nil
	}

	// Calculate check interval
	interval := time.Duration(nodeCheck.Spec.CheckInterval) * time.Minute
	if interval == 0 {
		interval = 5 * time.Minute // Default interval
	}

	// Check if enough time has passed since last check
	if !nodeCheck.Status.LastCheckTime.IsZero() {
		timeSinceLastCheck := time.Since(nodeCheck.Status.LastCheckTime.Time)
		if timeSinceLastCheck < interval {
			// Not enough time has passed, requeue for the remaining time
			remainingTime := interval - timeSinceLastCheck
			log.Info("Skipping check, not enough time has passed", 
				"timeSinceLastCheck", timeSinceLastCheck, 
				"interval", interval, 
				"remainingTime", remainingTime)
			return ctrl.Result{RequeueAfter: remainingTime}, nil
		}
	}

	log.Info("Executing checks for NodeCheck", "nodeCheck", req.Name, "node", currentNodeName)

	// Initialize check results for the current node
	systemResults := make(map[string]nodecheckv1alpha1.CheckResult)
	kubernetesResults := make(map[string]nodecheckv1alpha1.CheckResult)

	// Perform system checks for the current node
	if nodeCheck.Spec.SystemChecks.Uptime {
		systemChecker := checks.NewSystemChecker(currentNodeName)
		result := systemChecker.CheckUptime(ctx)
		systemResults["uptime"] = *result
	}

	if nodeCheck.Spec.SystemChecks.Processes {
		systemChecker := checks.NewSystemChecker(currentNodeName)
		result := systemChecker.CheckProcesses(ctx)
		systemResults["processes"] = *result
	}

	if nodeCheck.Spec.SystemChecks.Resources {
		systemChecker := checks.NewSystemChecker(currentNodeName)
		result := systemChecker.CheckResources(ctx)
		systemResults["resources"] = *result
	}

	if nodeCheck.Spec.SystemChecks.Memory {
		systemChecker := checks.NewSystemChecker(currentNodeName)
		result := systemChecker.CheckMemory(ctx)
		systemResults["memory"] = *result
	}

	if nodeCheck.Spec.SystemChecks.UninterruptibleTasks {
		systemChecker := checks.NewSystemChecker(currentNodeName)
		result := systemChecker.CheckUninterruptibleTasks(ctx)
		systemResults["uninterruptible_tasks"] = *result
	}

	if nodeCheck.Spec.SystemChecks.Services {
		systemChecker := checks.NewSystemChecker(currentNodeName)
		result := systemChecker.CheckServices(ctx)
		systemResults["services"] = *result
	}

	if nodeCheck.Spec.SystemChecks.SystemLogs {
		systemChecker := checks.NewSystemChecker(currentNodeName)
		result := systemChecker.CheckSystemLogs(ctx)
		systemResults["system_logs"] = *result
	}

	// New system checks
	systemChecker := checks.NewSystemChecker(currentNodeName)
	if nodeCheck.Spec.SystemChecks.FileDescriptors {
		result := systemChecker.CheckFileDescriptors(ctx)
		systemResults["file_descriptors"] = *result
	}
	if nodeCheck.Spec.SystemChecks.ZombieProcesses {
		result := systemChecker.CheckZombieProcesses(ctx)
		systemResults["zombie_processes"] = *result
	}
	if nodeCheck.Spec.SystemChecks.NTPSync {
		result := systemChecker.CheckNTPSync(ctx)
		systemResults["ntp_sync"] = *result
	}
	if nodeCheck.Spec.SystemChecks.KernelPanics {
		result := systemChecker.CheckKernelPanics(ctx)
		systemResults["kernel_panics"] = *result
	}
	if nodeCheck.Spec.SystemChecks.OOMKiller {
		result := systemChecker.CheckOOMKiller(ctx)
		systemResults["oom_killer"] = *result
	}
	if nodeCheck.Spec.SystemChecks.CPUFrequency {
		result := systemChecker.CheckCPUFrequency(ctx)
		systemResults["cpu_frequency"] = *result
	}
	if nodeCheck.Spec.SystemChecks.InterruptsBalance {
		result := systemChecker.CheckInterruptsBalance(ctx)
		systemResults["interrupts_balance"] = *result
	}
	if nodeCheck.Spec.SystemChecks.CPUStealTime {
		result := systemChecker.CheckCPUStealTime(ctx)
		systemResults["cpu_steal_time"] = *result
	}
	if nodeCheck.Spec.SystemChecks.MemoryFragmentation {
		result := systemChecker.CheckMemoryFragmentation(ctx)
		systemResults["memory_fragmentation"] = *result
	}
	if nodeCheck.Spec.SystemChecks.SwapActivity {
		result := systemChecker.CheckSwapActivity(ctx)
		systemResults["swap_activity"] = *result
	}
	if nodeCheck.Spec.SystemChecks.ContextSwitches {
		result := systemChecker.CheckContextSwitches(ctx)
		systemResults["context_switches"] = *result
	}
	if nodeCheck.Spec.SystemChecks.SELinuxStatus {
		result := systemChecker.CheckSELinuxStatus(ctx)
		systemResults["selinux_status"] = *result
	}
	if nodeCheck.Spec.SystemChecks.SSHAccess {
		result := systemChecker.CheckSSHAccess(ctx)
		systemResults["ssh_access"] = *result
	}
	if nodeCheck.Spec.SystemChecks.KernelModules {
		result := systemChecker.CheckKernelModules(ctx)
		systemResults["kernel_modules"] = *result
	}

	// Perform disk checks for the current node
	if nodeCheck.Spec.SystemChecks.Disks.Space || nodeCheck.Spec.SystemChecks.Disks.SMART || 
	   nodeCheck.Spec.SystemChecks.Disks.Performance || nodeCheck.Spec.SystemChecks.Disks.RAID ||
	   nodeCheck.Spec.SystemChecks.Disks.PVs || nodeCheck.Spec.SystemChecks.Disks.LVM ||
	   nodeCheck.Spec.SystemChecks.Disks.IOWait || nodeCheck.Spec.SystemChecks.Disks.QueueDepth ||
	   nodeCheck.Spec.SystemChecks.Disks.FilesystemErrors || nodeCheck.Spec.SystemChecks.Disks.InodeUsage ||
	   nodeCheck.Spec.SystemChecks.Disks.MountPoints {
		diskChecker := checks.NewDiskChecker(currentNodeName)
		if nodeCheck.Spec.SystemChecks.Disks.Space {
			result := diskChecker.CheckDiskSpace(ctx)
			systemResults["disk_space"] = *result
		}
		if nodeCheck.Spec.SystemChecks.Disks.SMART {
			result := diskChecker.CheckSMART(ctx)
			systemResults["disk_smart"] = *result
		}
		if nodeCheck.Spec.SystemChecks.Disks.Performance {
			result := diskChecker.CheckDiskPerformance(ctx)
			systemResults["disk_performance"] = *result
		}
		if nodeCheck.Spec.SystemChecks.Disks.RAID {
			result := diskChecker.CheckRAID(ctx)
			systemResults["disk_raid"] = *result
		}
		if nodeCheck.Spec.SystemChecks.Disks.PVs {
			result := diskChecker.CheckPVs(ctx)
			systemResults["disk_pvs"] = *result
		}
		if nodeCheck.Spec.SystemChecks.Disks.LVM {
			result := diskChecker.CheckLVM(ctx)
			systemResults["disk_lvm"] = *result
		}
		if nodeCheck.Spec.SystemChecks.Disks.IOWait {
			result := diskChecker.CheckIOWait(ctx)
			systemResults["disk_io_wait"] = *result
		}
		if nodeCheck.Spec.SystemChecks.Disks.QueueDepth {
			result := diskChecker.CheckQueueDepth(ctx)
			systemResults["disk_queue_depth"] = *result
		}
		if nodeCheck.Spec.SystemChecks.Disks.FilesystemErrors {
			result := diskChecker.CheckFilesystemErrors(ctx)
			systemResults["disk_filesystem_errors"] = *result
		}
		if nodeCheck.Spec.SystemChecks.Disks.InodeUsage {
			result := diskChecker.CheckInodeUsage(ctx)
			systemResults["disk_inode_usage"] = *result
		}
		if nodeCheck.Spec.SystemChecks.Disks.MountPoints {
			result := diskChecker.CheckMountPoints(ctx)
			systemResults["disk_mount_points"] = *result
		}
	}

	// Perform hardware checks for the current node
	if nodeCheck.Spec.SystemChecks.Hardware.Temperature || nodeCheck.Spec.SystemChecks.Hardware.IPMI || 
	   nodeCheck.Spec.SystemChecks.Hardware.BMC || nodeCheck.Spec.SystemChecks.Hardware.FanStatus ||
	   nodeCheck.Spec.SystemChecks.Hardware.PowerSupply || nodeCheck.Spec.SystemChecks.Hardware.MemoryErrors ||
	   nodeCheck.Spec.SystemChecks.Hardware.PCIeErrors || nodeCheck.Spec.SystemChecks.Hardware.CPUMicrocode {
		hardwareChecker := checks.NewHardwareChecker(currentNodeName)
		if nodeCheck.Spec.SystemChecks.Hardware.Temperature {
			result := hardwareChecker.CheckTemperature(ctx)
			systemResults["hardware_temperature"] = *result
		}
		if nodeCheck.Spec.SystemChecks.Hardware.IPMI {
			result := hardwareChecker.CheckIPMI(ctx)
			systemResults["hardware_ipmi"] = *result
		}
		if nodeCheck.Spec.SystemChecks.Hardware.BMC {
			result := hardwareChecker.CheckBMC(ctx)
			systemResults["hardware_bmc"] = *result
		}
		if nodeCheck.Spec.SystemChecks.Hardware.FanStatus {
			result := hardwareChecker.CheckFanStatus(ctx)
			systemResults["hardware_fan_status"] = *result
		}
		if nodeCheck.Spec.SystemChecks.Hardware.PowerSupply {
			result := hardwareChecker.CheckPowerSupply(ctx)
			systemResults["hardware_power_supply"] = *result
		}
		if nodeCheck.Spec.SystemChecks.Hardware.MemoryErrors {
			result := hardwareChecker.CheckMemoryErrors(ctx)
			systemResults["hardware_memory_errors"] = *result
		}
		if nodeCheck.Spec.SystemChecks.Hardware.PCIeErrors {
			result := hardwareChecker.CheckPCIeErrors(ctx)
			systemResults["hardware_pcie_errors"] = *result
		}
		if nodeCheck.Spec.SystemChecks.Hardware.CPUMicrocode {
			result := hardwareChecker.CheckCPUMicrocode(ctx)
			systemResults["hardware_cpu_microcode"] = *result
		}
	}

	// Perform network checks for the current node
	if nodeCheck.Spec.SystemChecks.Network.Interfaces || nodeCheck.Spec.SystemChecks.Network.Routing || 
	   nodeCheck.Spec.SystemChecks.Network.Connectivity || nodeCheck.Spec.SystemChecks.Network.Statistics ||
	   nodeCheck.Spec.SystemChecks.Network.Errors || nodeCheck.Spec.SystemChecks.Network.Latency ||
	   nodeCheck.Spec.SystemChecks.Network.DNSResolution || nodeCheck.Spec.SystemChecks.Network.BondingStatus ||
	   nodeCheck.Spec.SystemChecks.Network.FirewallRules {
		networkChecker := checks.NewNetworkChecker(currentNodeName)
		if nodeCheck.Spec.SystemChecks.Network.Interfaces {
			result := networkChecker.CheckInterfaces(ctx)
			systemResults["network_interfaces"] = *result
		}
		if nodeCheck.Spec.SystemChecks.Network.Routing {
			result := networkChecker.CheckRouting(ctx)
			systemResults["network_routing"] = *result
		}
		if nodeCheck.Spec.SystemChecks.Network.Connectivity {
			result := networkChecker.CheckConnectivity(ctx)
			systemResults["network_connectivity"] = *result
		}
		if nodeCheck.Spec.SystemChecks.Network.Statistics {
			result := networkChecker.CheckStatistics(ctx)
			systemResults["network_statistics"] = *result
		}
		if nodeCheck.Spec.SystemChecks.Network.Errors {
			result := networkChecker.CheckErrors(ctx)
			systemResults["network_errors"] = *result
		}
		if nodeCheck.Spec.SystemChecks.Network.Latency {
			result := networkChecker.CheckLatency(ctx)
			systemResults["network_latency"] = *result
		}
		if nodeCheck.Spec.SystemChecks.Network.DNSResolution {
			result := networkChecker.CheckDNSResolution(ctx)
			systemResults["network_dns_resolution"] = *result
		}
		if nodeCheck.Spec.SystemChecks.Network.BondingStatus {
			result := networkChecker.CheckBondingStatus(ctx)
			systemResults["network_bonding_status"] = *result
		}
		if nodeCheck.Spec.SystemChecks.Network.FirewallRules {
			result := networkChecker.CheckFirewallRules(ctx)
			systemResults["network_firewall_rules"] = *result
		}
	}

	// Perform Kubernetes checks
	kubernetesChecker, err := checks.NewKubernetesChecker(nodeCheck.Spec.NodeName)
	if err != nil {
		log.Error(err, "failed to create Kubernetes checker")
	} else {
		if nodeCheck.Spec.KubernetesChecks.NodeStatus {
			result := kubernetesChecker.CheckNodeStatus(ctx)
			kubernetesResults["node_status"] = *result
		}

		if nodeCheck.Spec.KubernetesChecks.Pods {
			result := kubernetesChecker.CheckPods(ctx)
			kubernetesResults["pods"] = *result
		}

		if nodeCheck.Spec.KubernetesChecks.ClusterOperators {
			result := kubernetesChecker.CheckClusterOperators(ctx)
			kubernetesResults["cluster_operators"] = *result
		}

		if nodeCheck.Spec.KubernetesChecks.NodeResources {
			result := kubernetesChecker.CheckNodeResources(ctx)
			kubernetesResults["node_resources"] = *result
		}

		if nodeCheck.Spec.KubernetesChecks.NodeResourceUsage {
			result := kubernetesChecker.CheckNodeResourceUsage(ctx)
			kubernetesResults["node_resource_usage"] = *result
		}
		if nodeCheck.Spec.KubernetesChecks.ContainerRuntime {
			result := kubernetesChecker.CheckContainerRuntime(ctx)
			kubernetesResults["container_runtime"] = *result
		}
		if nodeCheck.Spec.KubernetesChecks.KubeletHealth {
			result := kubernetesChecker.CheckKubeletHealth(ctx)
			kubernetesResults["kubelet_health"] = *result
		}
		if nodeCheck.Spec.KubernetesChecks.CNIPlugin {
			result := kubernetesChecker.CheckCNIPlugin(ctx)
			kubernetesResults["cni_plugin"] = *result
		}
		if nodeCheck.Spec.KubernetesChecks.NodeConditions {
			result := kubernetesChecker.CheckNodeConditions(ctx)
			kubernetesResults["node_conditions"] = *result
		}
	}

	// Determine overall status
	overallStatus := "Healthy"
	overallMessage := fmt.Sprintf("Node %s is healthy", currentNodeName)

	// Check for any critical or warning statuses from system checks
	for _, result := range systemResults {
		if result.Status == "Critical" {
			overallStatus = "Critical"
			overallMessage = result.Message
			break
		} else if result.Status == "Warning" && overallStatus == "Healthy" {
			overallStatus = "Warning"
			overallMessage = result.Message
		}
	}

	// Check for any critical or warning statuses from Kubernetes checks
	for _, result := range kubernetesResults {
		if result.Status == "Critical" {
			overallStatus = "Critical"
			overallMessage = result.Message
			break
		} else if result.Status == "Warning" && overallStatus == "Healthy" {
			overallStatus = "Warning"
			overallMessage = result.Message
		}
	}

	// Build SystemCheckResults struct
	systemCheckResults := nodecheckv1alpha1.SystemCheckResults{}
	if result, ok := systemResults["uptime"]; ok {
		systemCheckResults.Uptime = &result
	}
	if result, ok := systemResults["processes"]; ok {
		systemCheckResults.Processes = &result
	}
	if result, ok := systemResults["resources"]; ok {
		systemCheckResults.Resources = &result
	}
	if result, ok := systemResults["memory"]; ok {
		systemCheckResults.Memory = &result
	}
	if result, ok := systemResults["uninterruptible_tasks"]; ok {
		systemCheckResults.UninterruptibleTasks = &result
	}
	if result, ok := systemResults["services"]; ok {
		systemCheckResults.Services = &result
	}
	if result, ok := systemResults["system_logs"]; ok {
		systemCheckResults.SystemLogs = &result
	}
	if result, ok := systemResults["file_descriptors"]; ok {
		systemCheckResults.FileDescriptors = &result
	}
	if result, ok := systemResults["zombie_processes"]; ok {
		systemCheckResults.ZombieProcesses = &result
	}
	if result, ok := systemResults["ntp_sync"]; ok {
		systemCheckResults.NTPSync = &result
	}
	if result, ok := systemResults["kernel_panics"]; ok {
		systemCheckResults.KernelPanics = &result
	}
	if result, ok := systemResults["oom_killer"]; ok {
		systemCheckResults.OOMKiller = &result
	}
	if result, ok := systemResults["cpu_frequency"]; ok {
		systemCheckResults.CPUFrequency = &result
	}
	if result, ok := systemResults["interrupts_balance"]; ok {
		systemCheckResults.InterruptsBalance = &result
	}
	if result, ok := systemResults["cpu_steal_time"]; ok {
		systemCheckResults.CPUStealTime = &result
	}
	if result, ok := systemResults["memory_fragmentation"]; ok {
		systemCheckResults.MemoryFragmentation = &result
	}
	if result, ok := systemResults["swap_activity"]; ok {
		systemCheckResults.SwapActivity = &result
	}
	if result, ok := systemResults["context_switches"]; ok {
		systemCheckResults.ContextSwitches = &result
	}
	if result, ok := systemResults["selinux_status"]; ok {
		systemCheckResults.SELinuxStatus = &result
	}
	if result, ok := systemResults["ssh_access"]; ok {
		systemCheckResults.SSHAccess = &result
	}
	if result, ok := systemResults["kernel_modules"]; ok {
		systemCheckResults.KernelModules = &result
	}
	
	// Build HardwareCheckResults
	hardwareResults := &nodecheckv1alpha1.HardwareCheckResults{}
	if result, ok := systemResults["hardware_temperature"]; ok {
		hardwareResults.Temperature = &result
	}
	if result, ok := systemResults["hardware_ipmi"]; ok {
		hardwareResults.IPMI = &result
	}
	if result, ok := systemResults["hardware_bmc"]; ok {
		hardwareResults.BMC = &result
	}
	if result, ok := systemResults["hardware_fan_status"]; ok {
		hardwareResults.FanStatus = &result
	}
	if result, ok := systemResults["hardware_power_supply"]; ok {
		hardwareResults.PowerSupply = &result
	}
	if result, ok := systemResults["hardware_memory_errors"]; ok {
		hardwareResults.MemoryErrors = &result
	}
	if result, ok := systemResults["hardware_pcie_errors"]; ok {
		hardwareResults.PCIeErrors = &result
	}
	if result, ok := systemResults["hardware_cpu_microcode"]; ok {
		hardwareResults.CPUMicrocode = &result
	}
	if hardwareResults.Temperature != nil || hardwareResults.IPMI != nil || hardwareResults.BMC != nil ||
	   hardwareResults.FanStatus != nil || hardwareResults.PowerSupply != nil || hardwareResults.MemoryErrors != nil ||
	   hardwareResults.PCIeErrors != nil || hardwareResults.CPUMicrocode != nil {
		systemCheckResults.Hardware = hardwareResults
	}
	
	// Build DiskCheckResults
	diskResults := &nodecheckv1alpha1.DiskCheckResults{}
	if result, ok := systemResults["disk_space"]; ok {
		diskResults.Space = &result
	}
	if result, ok := systemResults["disk_smart"]; ok {
		diskResults.SMART = &result
	}
	if result, ok := systemResults["disk_performance"]; ok {
		diskResults.Performance = &result
	}
	if result, ok := systemResults["disk_raid"]; ok {
		diskResults.RAID = &result
	}
	if result, ok := systemResults["disk_pvs"]; ok {
		diskResults.PVs = &result
	}
	if result, ok := systemResults["disk_lvm"]; ok {
		diskResults.LVM = &result
	}
	if result, ok := systemResults["disk_io_wait"]; ok {
		diskResults.IOWait = &result
	}
	if result, ok := systemResults["disk_queue_depth"]; ok {
		diskResults.QueueDepth = &result
	}
	if result, ok := systemResults["disk_filesystem_errors"]; ok {
		diskResults.FilesystemErrors = &result
	}
	if result, ok := systemResults["disk_inode_usage"]; ok {
		diskResults.InodeUsage = &result
	}
	if result, ok := systemResults["disk_mount_points"]; ok {
		diskResults.MountPoints = &result
	}
	if diskResults.Space != nil || diskResults.SMART != nil || diskResults.Performance != nil || 
	   diskResults.RAID != nil || diskResults.PVs != nil || diskResults.LVM != nil ||
	   diskResults.IOWait != nil || diskResults.QueueDepth != nil || diskResults.FilesystemErrors != nil ||
	   diskResults.InodeUsage != nil || diskResults.MountPoints != nil {
		systemCheckResults.Disks = diskResults
	}
	
	// Build NetworkCheckResults
	networkResults := &nodecheckv1alpha1.NetworkCheckResults{}
	if result, ok := systemResults["network_interfaces"]; ok {
		networkResults.Interfaces = &result
	}
	if result, ok := systemResults["network_routing"]; ok {
		networkResults.Routing = &result
	}
	if result, ok := systemResults["network_connectivity"]; ok {
		networkResults.Connectivity = &result
	}
	if result, ok := systemResults["network_statistics"]; ok {
		networkResults.Statistics = &result
	}
	if result, ok := systemResults["network_errors"]; ok {
		networkResults.Errors = &result
	}
	if result, ok := systemResults["network_latency"]; ok {
		networkResults.Latency = &result
	}
	if result, ok := systemResults["network_dns_resolution"]; ok {
		networkResults.DNSResolution = &result
	}
	if result, ok := systemResults["network_bonding_status"]; ok {
		networkResults.BondingStatus = &result
	}
	if result, ok := systemResults["network_firewall_rules"]; ok {
		networkResults.FirewallRules = &result
	}
	if networkResults.Interfaces != nil || networkResults.Routing != nil || networkResults.Connectivity != nil || networkResults.Statistics != nil ||
	   networkResults.Errors != nil || networkResults.Latency != nil || networkResults.DNSResolution != nil ||
	   networkResults.BondingStatus != nil || networkResults.FirewallRules != nil {
		systemCheckResults.Network = networkResults
	}

	// Build KubernetesCheckResults struct
	kubernetesCheckResults := nodecheckv1alpha1.KubernetesCheckResults{}
	if result, ok := kubernetesResults["node_status"]; ok {
		kubernetesCheckResults.NodeStatus = &result
	}
	if result, ok := kubernetesResults["pods"]; ok {
		kubernetesCheckResults.Pods = &result
	}
	if result, ok := kubernetesResults["cluster_operators"]; ok {
		kubernetesCheckResults.ClusterOperators = &result
	}
	if result, ok := kubernetesResults["node_resources"]; ok {
		kubernetesCheckResults.NodeResources = &result
	}
	if result, ok := kubernetesResults["node_resource_usage"]; ok {
		kubernetesCheckResults.NodeResourceUsage = &result
	}
	if result, ok := kubernetesResults["container_runtime"]; ok {
		kubernetesCheckResults.ContainerRuntime = &result
	}
	if result, ok := kubernetesResults["kubelet_health"]; ok {
		kubernetesCheckResults.KubeletHealth = &result
	}
	if result, ok := kubernetesResults["cni_plugin"]; ok {
		kubernetesCheckResults.CNIPlugin = &result
	}
	if result, ok := kubernetesResults["node_conditions"]; ok {
		kubernetesCheckResults.NodeConditions = &result
	}

	// Update status
	nodeCheck.Status.NodeName = currentNodeName
	nodeCheck.Status.OverallStatus = overallStatus
	nodeCheck.Status.Message = overallMessage
	nodeCheck.Status.LastCheckTime = metav1.Now()
	nodeCheck.Status.CheckResults = nodecheckv1alpha1.CheckResults{
		SystemResults:     systemCheckResults,
		KubernetesResults: kubernetesCheckResults,
	}

	// Update the status with retry logic for conflict errors
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		if err := r.Status().Update(ctx, &nodeCheck); err != nil {
			if errors.IsConflict(err) {
				// Conflict error - fetch latest version and retry
				if i < maxRetries-1 {
					log.Info("Conflict updating status, retrying...", "attempt", i+1, "maxRetries", maxRetries)
					// Fetch latest version
					if err := r.Get(ctx, req.NamespacedName, &nodeCheck); err != nil {
						log.Error(err, "unable to fetch NodeCheck for retry")
						return ctrl.Result{}, err
					}
					// Re-apply status changes
					nodeCheck.Status.NodeName = currentNodeName
					nodeCheck.Status.OverallStatus = overallStatus
					nodeCheck.Status.Message = overallMessage
					nodeCheck.Status.LastCheckTime = metav1.Now()
					nodeCheck.Status.CheckResults = nodecheckv1alpha1.CheckResults{
						SystemResults:     systemCheckResults,
						KubernetesResults: kubernetesCheckResults,
					}
					time.Sleep(time.Millisecond * 100 * time.Duration(i+1)) // Exponential backoff
					continue
				}
			}
			// Non-conflict error or max retries reached
			log.Error(err, "unable to update NodeCheck status")
			return ctrl.Result{}, err
		}
		// Success
		break
	}

	log.Info("NodeCheck checks executed successfully", "node", currentNodeName, "status", overallStatus)

	// Reconcile again after the specified interval
	return ctrl.Result{RequeueAfter: interval}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeCheckExecutorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&nodecheckv1alpha1.NodeCheck{}).
		Complete(r)
}

