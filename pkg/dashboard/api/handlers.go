package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/albertofilice/node-check-operator/api/v1alpha1"
	"github.com/albertofilice/node-check-operator/pkg/metrics"
	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DashboardAPI handles dashboard API requests
type DashboardAPI struct {
	k8sClient    client.Client
	clientset     *kubernetes.Clientset
	namespace    string
}

// NewDashboardAPI creates a new dashboard API
func NewDashboardAPI(k8sClient client.Client, clientset *kubernetes.Clientset, namespace string) *DashboardAPI {
	return &DashboardAPI{
		k8sClient: k8sClient,
		clientset: clientset,
		namespace: namespace,
	}
}

// NodeCheckSummary represents a summary of a NodeCheck
type NodeCheckSummary struct {
	Name          string    `json:"name"`
	Namespace     string    `json:"namespace"`
	NodeName      string    `json:"nodeName"`
	OverallStatus string    `json:"overallStatus"`
	LastCheck     time.Time `json:"lastCheck"`
	Message       string    `json:"message"`
	CheckCount    int       `json:"checkCount"`
	HealthyCount  int       `json:"healthyCount"`
	WarningCount  int       `json:"warningCount"`
	CriticalCount int       `json:"criticalCount"`
}

// CheckResultAPI represents a check result for API responses (with details as object instead of RawExtension)
type CheckResultAPI struct {
	Status    string                 `json:"status"`
	Message   string                 `json:"message,omitempty"`
	Timestamp string                 `json:"timestamp"`
	Command   string                 `json:"command,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// SystemCheckResultsAPI represents system check results for API responses
type SystemCheckResultsAPI struct {
	Uptime              *CheckResultAPI           `json:"uptime,omitempty"`
	Processes           *CheckResultAPI           `json:"processes,omitempty"`
	Resources           *CheckResultAPI           `json:"resources,omitempty"`
	Services            *CheckResultAPI           `json:"services,omitempty"`
	Memory              *CheckResultAPI           `json:"memory,omitempty"`
	UninterruptibleTasks *CheckResultAPI          `json:"uninterruptibleTasks,omitempty"`
	SystemLogs          *CheckResultAPI           `json:"systemLogs,omitempty"`
	FileDescriptors     *CheckResultAPI           `json:"fileDescriptors,omitempty"`
	ZombieProcesses     *CheckResultAPI           `json:"zombieProcesses,omitempty"`
	NTPSync             *CheckResultAPI           `json:"ntpSync,omitempty"`
	KernelPanics        *CheckResultAPI           `json:"kernelPanics,omitempty"`
	OOMKiller           *CheckResultAPI           `json:"oomKiller,omitempty"`
	CPUFrequency        *CheckResultAPI           `json:"cpuFrequency,omitempty"`
	InterruptsBalance   *CheckResultAPI           `json:"interruptsBalance,omitempty"`
	CPUStealTime        *CheckResultAPI           `json:"cpuStealTime,omitempty"`
	MemoryFragmentation *CheckResultAPI           `json:"memoryFragmentation,omitempty"`
	SwapActivity        *CheckResultAPI           `json:"swapActivity,omitempty"`
	ContextSwitches     *CheckResultAPI           `json:"contextSwitches,omitempty"`
	SELinuxStatus       *CheckResultAPI           `json:"selinuxStatus,omitempty"`
	SSHAccess           *CheckResultAPI           `json:"sshAccess,omitempty"`
	KernelModules       *CheckResultAPI           `json:"kernelModules,omitempty"`
	Hardware            *HardwareCheckResultsAPI  `json:"hardware,omitempty"`
	Disks               *DiskCheckResultsAPI      `json:"disks,omitempty"`
	Network             *NetworkCheckResultsAPI   `json:"network,omitempty"`
}

// HardwareCheckResultsAPI represents hardware check results for API responses
type HardwareCheckResultsAPI struct {
	Temperature *CheckResultAPI `json:"temperature,omitempty"`
	IPMI        *CheckResultAPI `json:"ipmi,omitempty"`
	BMC         *CheckResultAPI `json:"bmc,omitempty"`
	FanStatus   *CheckResultAPI `json:"fanStatus,omitempty"`
	PowerSupply *CheckResultAPI `json:"powerSupply,omitempty"`
	MemoryErrors *CheckResultAPI `json:"memoryErrors,omitempty"`
	PCIeErrors  *CheckResultAPI `json:"pcieErrors,omitempty"`
	CPUMicrocode *CheckResultAPI `json:"cpuMicrocode,omitempty"`
}

// DiskCheckResultsAPI represents disk check results for API responses
type DiskCheckResultsAPI struct {
	Space            *CheckResultAPI `json:"space,omitempty"`
	SMART            *CheckResultAPI `json:"smart,omitempty"`
	Performance      *CheckResultAPI `json:"performance,omitempty"`
	RAID             *CheckResultAPI `json:"raid,omitempty"`
	PVs              *CheckResultAPI `json:"pvs,omitempty"`
	LVM              *CheckResultAPI `json:"lvm,omitempty"`
	IOWait           *CheckResultAPI `json:"ioWait,omitempty"`
	QueueDepth       *CheckResultAPI `json:"queueDepth,omitempty"`
	FilesystemErrors *CheckResultAPI `json:"filesystemErrors,omitempty"`
	InodeUsage       *CheckResultAPI `json:"inodeUsage,omitempty"`
	MountPoints      *CheckResultAPI `json:"mountPoints,omitempty"`
}

// NetworkCheckResultsAPI represents network check results for API responses
type NetworkCheckResultsAPI struct {
	Interfaces    *CheckResultAPI `json:"interfaces,omitempty"`
	Routing       *CheckResultAPI `json:"routing,omitempty"`
	Connectivity  *CheckResultAPI `json:"connectivity,omitempty"`
	Statistics    *CheckResultAPI `json:"statistics,omitempty"`
	Errors        *CheckResultAPI `json:"errors,omitempty"`
	Latency       *CheckResultAPI `json:"latency,omitempty"`
	DNSResolution *CheckResultAPI `json:"dnsResolution,omitempty"`
	BondingStatus *CheckResultAPI `json:"bondingStatus,omitempty"`
	FirewallRules *CheckResultAPI `json:"firewallRules,omitempty"`
}

// KubernetesCheckResultsAPI represents Kubernetes check results for API responses
type KubernetesCheckResultsAPI struct {
	NodeStatus         *CheckResultAPI `json:"nodeStatus,omitempty"`
	Pods               *CheckResultAPI `json:"pods,omitempty"`
	ClusterOperators   *CheckResultAPI `json:"clusterOperators,omitempty"`
	NodeResources      *CheckResultAPI `json:"nodeResources,omitempty"`
	NodeResourceUsage  *CheckResultAPI `json:"nodeResourceUsage,omitempty"`
	ContainerRuntime   *CheckResultAPI `json:"containerRuntime,omitempty"`
	KubeletHealth      *CheckResultAPI `json:"kubeletHealth,omitempty"`
	CNIPlugin          *CheckResultAPI `json:"cniPlugin,omitempty"`
	NodeConditions     *CheckResultAPI `json:"nodeConditions,omitempty"`
}

// NodeCheckDetail represents detailed information about a NodeCheck
type NodeCheckDetail struct {
	NodeCheckSummary
	SystemResults     *SystemCheckResultsAPI     `json:"systemResults"`
	KubernetesResults *KubernetesCheckResultsAPI `json:"kubernetesResults"`
}

// CheckSummary represents a summary of a check type across all nodes
type CheckSummary struct {
	Name           string `json:"name"`
	Category       string `json:"category"` // "system" or "kubernetes"
	Enabled        bool   `json:"enabled"`
	HealthyCount   int    `json:"healthyCount"`
	WarningCount   int    `json:"warningCount"`
	CriticalCount  int    `json:"criticalCount"`
	UnknownCount   int    `json:"unknownCount"`
	OverallStatus  string `json:"overallStatus"` // Worst status across all nodes
}

// DashboardStats represents dashboard statistics
type DashboardStats struct {
	TotalNodeChecks int            `json:"totalNodeChecks"`
	HealthyNodes    int            `json:"healthyNodes"`
	WarningNodes    int            `json:"warningNodes"`
	CriticalNodes   int            `json:"criticalNodes"`
	UnknownNodes    int            `json:"unknownNodes"`
	LastUpdate      time.Time      `json:"lastUpdate"`
	Checks          []CheckSummary `json:"checks,omitempty"`
}

// countStatus increments the appropriate counter based on check status
func countStatus(summary *NodeCheckSummary, status string) {
	switch status {
	case "Healthy":
		summary.HealthyCount++
	case "Warning":
		summary.WarningCount++
	case "Critical":
		summary.CriticalCount++
	}
}

// GetDashboardStats returns overall dashboard statistics
func (api *DashboardAPI) GetDashboardStats(c *gin.Context) {
	ctx := context.Background()
	
	// Get all NodeChecks
	var nodeChecks v1alpha1.NodeCheckList
	if err := api.k8sClient.List(ctx, &nodeChecks); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Filter out generic NodeChecks (nodeName == "*")
	filteredNodeChecks := make([]v1alpha1.NodeCheck, 0)
	for _, nc := range nodeChecks.Items {
		if nc.Spec.NodeName != "*" && nc.Spec.NodeName != "all" {
			filteredNodeChecks = append(filteredNodeChecks, nc)
		}
	}

	stats := DashboardStats{
		TotalNodeChecks: len(filteredNodeChecks),
		LastUpdate:      time.Now(),
		Checks:          []CheckSummary{},
	}

	// Map to track check summaries
	checkMap := make(map[string]*CheckSummary)

	// Count by status and collect check information
	for _, nc := range filteredNodeChecks {
		switch nc.Status.OverallStatus {
		case "Healthy":
			stats.HealthyNodes++
		case "Warning":
			stats.WarningNodes++
		case "Critical":
			stats.CriticalNodes++
		default:
			stats.UnknownNodes++
		}

		// Process system checks
		systemResults := nc.Status.CheckResults.SystemResults
		{
			
			// Uptime
			if systemResults.Uptime != nil {
				key := "system:uptime"
				if checkMap[key] == nil {
					checkMap[key] = &CheckSummary{Name: "Uptime", Category: "system", Enabled: true}
				}
				updateCheckSummary(checkMap[key], systemResults.Uptime.Status)
			}

			// Processes
			if systemResults.Processes != nil {
				key := "system:processes"
				if checkMap[key] == nil {
					checkMap[key] = &CheckSummary{Name: "Processes", Category: "system", Enabled: true}
				}
				updateCheckSummary(checkMap[key], systemResults.Processes.Status)
			}

			// Resources
			if systemResults.Resources != nil {
				key := "system:resources"
				if checkMap[key] == nil {
					checkMap[key] = &CheckSummary{Name: "Resources", Category: "system", Enabled: true}
				}
				updateCheckSummary(checkMap[key], systemResults.Resources.Status)
			}

			// Memory
			if systemResults.Memory != nil {
				key := "system:memory"
				if checkMap[key] == nil {
					checkMap[key] = &CheckSummary{Name: "Memory", Category: "system", Enabled: true}
				}
				updateCheckSummary(checkMap[key], systemResults.Memory.Status)
			}

			// UninterruptibleTasks
			if systemResults.UninterruptibleTasks != nil {
				key := "system:uninterruptible_tasks"
				if checkMap[key] == nil {
					checkMap[key] = &CheckSummary{Name: "Uninterruptible Tasks", Category: "system", Enabled: true}
				}
				updateCheckSummary(checkMap[key], systemResults.UninterruptibleTasks.Status)
			}

			// Services
			if systemResults.Services != nil {
				key := "system:services"
				if checkMap[key] == nil {
					checkMap[key] = &CheckSummary{Name: "Services", Category: "system", Enabled: true}
				}
				updateCheckSummary(checkMap[key], systemResults.Services.Status)
			}

			// SystemLogs
			if systemResults.SystemLogs != nil {
				key := "system:system_logs"
				if checkMap[key] == nil {
					checkMap[key] = &CheckSummary{Name: "System Logs", Category: "system", Enabled: true}
				}
				updateCheckSummary(checkMap[key], systemResults.SystemLogs.Status)
			}

			// FileDescriptors
			if systemResults.FileDescriptors != nil {
				key := "system:file_descriptors"
				if checkMap[key] == nil {
					checkMap[key] = &CheckSummary{Name: "File Descriptors", Category: "system", Enabled: true}
				}
				updateCheckSummary(checkMap[key], systemResults.FileDescriptors.Status)
			}

			// ZombieProcesses
			if systemResults.ZombieProcesses != nil {
				key := "system:zombie_processes"
				if checkMap[key] == nil {
					checkMap[key] = &CheckSummary{Name: "Zombie Processes", Category: "system", Enabled: true}
				}
				updateCheckSummary(checkMap[key], systemResults.ZombieProcesses.Status)
			}

			// NTPSync
			if systemResults.NTPSync != nil {
				key := "system:ntp_sync"
				if checkMap[key] == nil {
					checkMap[key] = &CheckSummary{Name: "NTP Sync", Category: "system", Enabled: true}
				}
				updateCheckSummary(checkMap[key], systemResults.NTPSync.Status)
			}

			// KernelPanics
			if systemResults.KernelPanics != nil {
				key := "system:kernel_panics"
				if checkMap[key] == nil {
					checkMap[key] = &CheckSummary{Name: "Kernel Panics", Category: "system", Enabled: true}
				}
				updateCheckSummary(checkMap[key], systemResults.KernelPanics.Status)
			}

			// OOMKiller
			if systemResults.OOMKiller != nil {
				key := "system:oom_killer"
				if checkMap[key] == nil {
					checkMap[key] = &CheckSummary{Name: "OOM Killer", Category: "system", Enabled: true}
				}
				updateCheckSummary(checkMap[key], systemResults.OOMKiller.Status)
			}

			// CPUFrequency
			if systemResults.CPUFrequency != nil {
				key := "system:cpu_frequency"
				if checkMap[key] == nil {
					checkMap[key] = &CheckSummary{Name: "CPU Frequency", Category: "system", Enabled: true}
				}
				updateCheckSummary(checkMap[key], systemResults.CPUFrequency.Status)
			}

			// InterruptsBalance
			if systemResults.InterruptsBalance != nil {
				key := "system:interrupts_balance"
				if checkMap[key] == nil {
					checkMap[key] = &CheckSummary{Name: "Interrupts Balance", Category: "system", Enabled: true}
				}
				updateCheckSummary(checkMap[key], systemResults.InterruptsBalance.Status)
			}

			// CPUStealTime
			if systemResults.CPUStealTime != nil {
				key := "system:cpu_steal_time"
				if checkMap[key] == nil {
					checkMap[key] = &CheckSummary{Name: "CPU Steal Time", Category: "system", Enabled: true}
				}
				updateCheckSummary(checkMap[key], systemResults.CPUStealTime.Status)
			}

			// MemoryFragmentation
			if systemResults.MemoryFragmentation != nil {
				key := "system:memory_fragmentation"
				if checkMap[key] == nil {
					checkMap[key] = &CheckSummary{Name: "Memory Fragmentation", Category: "system", Enabled: true}
				}
				updateCheckSummary(checkMap[key], systemResults.MemoryFragmentation.Status)
			}

			// SwapActivity
			if systemResults.SwapActivity != nil {
				key := "system:swap_activity"
				if checkMap[key] == nil {
					checkMap[key] = &CheckSummary{Name: "Swap Activity", Category: "system", Enabled: true}
				}
				updateCheckSummary(checkMap[key], systemResults.SwapActivity.Status)
			}

			// ContextSwitches
			if systemResults.ContextSwitches != nil {
				key := "system:context_switches"
				if checkMap[key] == nil {
					checkMap[key] = &CheckSummary{Name: "Context Switches", Category: "system", Enabled: true}
				}
				updateCheckSummary(checkMap[key], systemResults.ContextSwitches.Status)
			}

			// SELinuxStatus
			if systemResults.SELinuxStatus != nil {
				key := "system:selinux_status"
				if checkMap[key] == nil {
					checkMap[key] = &CheckSummary{Name: "SELinux Status", Category: "system", Enabled: true}
				}
				updateCheckSummary(checkMap[key], systemResults.SELinuxStatus.Status)
			}

			// SSHAccess
			if systemResults.SSHAccess != nil {
				key := "system:ssh_access"
				if checkMap[key] == nil {
					checkMap[key] = &CheckSummary{Name: "SSH Access", Category: "system", Enabled: true}
				}
				updateCheckSummary(checkMap[key], systemResults.SSHAccess.Status)
			}

			// KernelModules
			if systemResults.KernelModules != nil {
				key := "system:kernel_modules"
				if checkMap[key] == nil {
					checkMap[key] = &CheckSummary{Name: "Kernel Modules", Category: "system", Enabled: true}
				}
				updateCheckSummary(checkMap[key], systemResults.KernelModules.Status)
			}

			// Disks
			if systemResults.Disks != nil {
				if systemResults.Disks.Space != nil {
					key := "system:disk_space"
					if checkMap[key] == nil {
						checkMap[key] = &CheckSummary{Name: "Disk Space", Category: "system", Enabled: true}
					}
					updateCheckSummary(checkMap[key], systemResults.Disks.Space.Status)
				}
				if systemResults.Disks.SMART != nil {
					key := "system:disk_smart"
					if checkMap[key] == nil {
						checkMap[key] = &CheckSummary{Name: "Disk SMART", Category: "system", Enabled: true}
					}
					updateCheckSummary(checkMap[key], systemResults.Disks.SMART.Status)
				}
				if systemResults.Disks.Performance != nil {
					key := "system:disk_performance"
					if checkMap[key] == nil {
						checkMap[key] = &CheckSummary{Name: "Disk Performance", Category: "system", Enabled: true}
					}
					updateCheckSummary(checkMap[key], systemResults.Disks.Performance.Status)
				}
				if systemResults.Disks.RAID != nil {
					key := "system:disk_raid"
					if checkMap[key] == nil {
						checkMap[key] = &CheckSummary{Name: "RAID", Category: "system", Enabled: true}
					}
					updateCheckSummary(checkMap[key], systemResults.Disks.RAID.Status)
				}
				if systemResults.Disks.PVs != nil {
					key := "system:disk_pvs"
					if checkMap[key] == nil {
						checkMap[key] = &CheckSummary{Name: "LVM PVs", Category: "system", Enabled: true}
					}
					updateCheckSummary(checkMap[key], systemResults.Disks.PVs.Status)
				}
				if systemResults.Disks.LVM != nil {
					key := "system:disk_lvm"
					if checkMap[key] == nil {
						checkMap[key] = &CheckSummary{Name: "LVM", Category: "system", Enabled: true}
					}
					updateCheckSummary(checkMap[key], systemResults.Disks.LVM.Status)
				}
				if systemResults.Disks.IOWait != nil {
					key := "system:disk_io_wait"
					if checkMap[key] == nil {
						checkMap[key] = &CheckSummary{Name: "I/O Wait", Category: "system", Enabled: true}
					}
					updateCheckSummary(checkMap[key], systemResults.Disks.IOWait.Status)
				}
				if systemResults.Disks.QueueDepth != nil {
					key := "system:disk_queue_depth"
					if checkMap[key] == nil {
						checkMap[key] = &CheckSummary{Name: "Queue Depth", Category: "system", Enabled: true}
					}
					updateCheckSummary(checkMap[key], systemResults.Disks.QueueDepth.Status)
				}
				if systemResults.Disks.FilesystemErrors != nil {
					key := "system:disk_filesystem_errors"
					if checkMap[key] == nil {
						checkMap[key] = &CheckSummary{Name: "Filesystem Errors", Category: "system", Enabled: true}
					}
					updateCheckSummary(checkMap[key], systemResults.Disks.FilesystemErrors.Status)
				}
				if systemResults.Disks.InodeUsage != nil {
					key := "system:disk_inode_usage"
					if checkMap[key] == nil {
						checkMap[key] = &CheckSummary{Name: "Inode Usage", Category: "system", Enabled: true}
					}
					updateCheckSummary(checkMap[key], systemResults.Disks.InodeUsage.Status)
				}
				if systemResults.Disks.MountPoints != nil {
					key := "system:disk_mount_points"
					if checkMap[key] == nil {
						checkMap[key] = &CheckSummary{Name: "Mount Points", Category: "system", Enabled: true}
					}
					updateCheckSummary(checkMap[key], systemResults.Disks.MountPoints.Status)
				}
			}

			// Network
			if systemResults.Network != nil {
				if systemResults.Network.Interfaces != nil {
					key := "system:network_interfaces"
					if checkMap[key] == nil {
						checkMap[key] = &CheckSummary{Name: "Network Interfaces", Category: "system", Enabled: true}
					}
					updateCheckSummary(checkMap[key], systemResults.Network.Interfaces.Status)
				}
				if systemResults.Network.Routing != nil {
					key := "system:network_routing"
					if checkMap[key] == nil {
						checkMap[key] = &CheckSummary{Name: "Network Routing", Category: "system", Enabled: true}
					}
					updateCheckSummary(checkMap[key], systemResults.Network.Routing.Status)
				}
				if systemResults.Network.Connectivity != nil {
					key := "system:network_connectivity"
					if checkMap[key] == nil {
						checkMap[key] = &CheckSummary{Name: "Network Connectivity", Category: "system", Enabled: true}
					}
					updateCheckSummary(checkMap[key], systemResults.Network.Connectivity.Status)
				}
				if systemResults.Network.Statistics != nil {
					key := "system:network_statistics"
					if checkMap[key] == nil {
						checkMap[key] = &CheckSummary{Name: "Network Statistics", Category: "system", Enabled: true}
					}
					updateCheckSummary(checkMap[key], systemResults.Network.Statistics.Status)
				}
				if systemResults.Network.Errors != nil {
					key := "system:network_errors"
					if checkMap[key] == nil {
						checkMap[key] = &CheckSummary{Name: "Network Errors", Category: "system", Enabled: true}
					}
					updateCheckSummary(checkMap[key], systemResults.Network.Errors.Status)
				}
				if systemResults.Network.Latency != nil {
					key := "system:network_latency"
					if checkMap[key] == nil {
						checkMap[key] = &CheckSummary{Name: "Network Latency", Category: "system", Enabled: true}
					}
					updateCheckSummary(checkMap[key], systemResults.Network.Latency.Status)
				}
				if systemResults.Network.DNSResolution != nil {
					key := "system:network_dns_resolution"
					if checkMap[key] == nil {
						checkMap[key] = &CheckSummary{Name: "DNS Resolution", Category: "system", Enabled: true}
					}
					updateCheckSummary(checkMap[key], systemResults.Network.DNSResolution.Status)
				}
				if systemResults.Network.BondingStatus != nil {
					key := "system:network_bonding_status"
					if checkMap[key] == nil {
						checkMap[key] = &CheckSummary{Name: "Bonding Status", Category: "system", Enabled: true}
					}
					updateCheckSummary(checkMap[key], systemResults.Network.BondingStatus.Status)
				}
				if systemResults.Network.FirewallRules != nil {
					key := "system:network_firewall_rules"
					if checkMap[key] == nil {
						checkMap[key] = &CheckSummary{Name: "Firewall Rules", Category: "system", Enabled: true}
					}
					updateCheckSummary(checkMap[key], systemResults.Network.FirewallRules.Status)
				}
			}

			// Hardware
			if systemResults.Hardware != nil {
				if systemResults.Hardware.Temperature != nil {
					key := "system:temperature"
					if checkMap[key] == nil {
						checkMap[key] = &CheckSummary{Name: "Temperature", Category: "system", Enabled: true}
					}
					updateCheckSummary(checkMap[key], systemResults.Hardware.Temperature.Status)
				}
				if systemResults.Hardware.IPMI != nil {
					key := "system:ipmi"
					if checkMap[key] == nil {
						checkMap[key] = &CheckSummary{Name: "IPMI", Category: "system", Enabled: true}
					}
					updateCheckSummary(checkMap[key], systemResults.Hardware.IPMI.Status)
				}
				if systemResults.Hardware.BMC != nil {
					key := "system:bmc"
					if checkMap[key] == nil {
						checkMap[key] = &CheckSummary{Name: "BMC", Category: "system", Enabled: true}
					}
					updateCheckSummary(checkMap[key], systemResults.Hardware.BMC.Status)
				}
				if systemResults.Hardware.FanStatus != nil {
					key := "system:fan_status"
					if checkMap[key] == nil {
						checkMap[key] = &CheckSummary{Name: "Fan Status", Category: "system", Enabled: true}
					}
					updateCheckSummary(checkMap[key], systemResults.Hardware.FanStatus.Status)
				}
				if systemResults.Hardware.PowerSupply != nil {
					key := "system:power_supply"
					if checkMap[key] == nil {
						checkMap[key] = &CheckSummary{Name: "Power Supply", Category: "system", Enabled: true}
					}
					updateCheckSummary(checkMap[key], systemResults.Hardware.PowerSupply.Status)
				}
				if systemResults.Hardware.MemoryErrors != nil {
					key := "system:memory_errors"
					if checkMap[key] == nil {
						checkMap[key] = &CheckSummary{Name: "Memory Errors", Category: "system", Enabled: true}
					}
					updateCheckSummary(checkMap[key], systemResults.Hardware.MemoryErrors.Status)
				}
				if systemResults.Hardware.PCIeErrors != nil {
					key := "system:pcie_errors"
					if checkMap[key] == nil {
						checkMap[key] = &CheckSummary{Name: "PCIe Errors", Category: "system", Enabled: true}
					}
					updateCheckSummary(checkMap[key], systemResults.Hardware.PCIeErrors.Status)
				}
				if systemResults.Hardware.CPUMicrocode != nil {
					key := "system:cpu_microcode"
					if checkMap[key] == nil {
						checkMap[key] = &CheckSummary{Name: "CPU Microcode", Category: "system", Enabled: true}
					}
					updateCheckSummary(checkMap[key], systemResults.Hardware.CPUMicrocode.Status)
				}
			}
		}

		// Process Kubernetes checks
		k8sResults := nc.Status.CheckResults.KubernetesResults
		{

			if k8sResults.NodeStatus != nil {
				key := "kubernetes:node_status"
				if checkMap[key] == nil {
					checkMap[key] = &CheckSummary{Name: "Node Status", Category: "kubernetes", Enabled: true}
				}
				updateCheckSummary(checkMap[key], k8sResults.NodeStatus.Status)
			}

			if k8sResults.Pods != nil {
				key := "kubernetes:pods"
				if checkMap[key] == nil {
					checkMap[key] = &CheckSummary{Name: "Pods", Category: "kubernetes", Enabled: true}
				}
				updateCheckSummary(checkMap[key], k8sResults.Pods.Status)
			}

			if k8sResults.ClusterOperators != nil {
				key := "kubernetes:cluster_operators"
				if checkMap[key] == nil {
					checkMap[key] = &CheckSummary{Name: "Cluster Operators", Category: "kubernetes", Enabled: true}
				}
				updateCheckSummary(checkMap[key], k8sResults.ClusterOperators.Status)
			}

			if k8sResults.NodeResources != nil {
				key := "kubernetes:node_resources"
				if checkMap[key] == nil {
					checkMap[key] = &CheckSummary{Name: "Node Resources", Category: "kubernetes", Enabled: true}
				}
				updateCheckSummary(checkMap[key], k8sResults.NodeResources.Status)
			}

			if k8sResults.NodeResourceUsage != nil {
				key := "kubernetes:node_resource_usage"
				if checkMap[key] == nil {
					checkMap[key] = &CheckSummary{Name: "Node Resource Usage", Category: "kubernetes", Enabled: true}
				}
				updateCheckSummary(checkMap[key], k8sResults.NodeResourceUsage.Status)
			}

			if k8sResults.ContainerRuntime != nil {
				key := "kubernetes:container_runtime"
				if checkMap[key] == nil {
					checkMap[key] = &CheckSummary{Name: "Container Runtime", Category: "kubernetes", Enabled: true}
				}
				updateCheckSummary(checkMap[key], k8sResults.ContainerRuntime.Status)
			}

			if k8sResults.KubeletHealth != nil {
				key := "kubernetes:kubelet_health"
				if checkMap[key] == nil {
					checkMap[key] = &CheckSummary{Name: "Kubelet Health", Category: "kubernetes", Enabled: true}
				}
				updateCheckSummary(checkMap[key], k8sResults.KubeletHealth.Status)
			}

			if k8sResults.CNIPlugin != nil {
				key := "kubernetes:cni_plugin"
				if checkMap[key] == nil {
					checkMap[key] = &CheckSummary{Name: "CNI Plugin", Category: "kubernetes", Enabled: true}
				}
				updateCheckSummary(checkMap[key], k8sResults.CNIPlugin.Status)
			}

			if k8sResults.NodeConditions != nil {
				key := "kubernetes:node_conditions"
				if checkMap[key] == nil {
					checkMap[key] = &CheckSummary{Name: "Node Conditions", Category: "kubernetes", Enabled: true}
				}
				updateCheckSummary(checkMap[key], k8sResults.NodeConditions.Status)
			}

		}
	}

	// Convert map to slice and calculate overall status
	for _, check := range checkMap {
		check.OverallStatus = calculateOverallStatus(check.HealthyCount, check.WarningCount, check.CriticalCount, check.UnknownCount)
		stats.Checks = append(stats.Checks, *check)
	}

	// Push metrics snapshot to Prometheus
	metricsSnapshot := metrics.DashboardSnapshot{
		TotalNodeChecks: stats.TotalNodeChecks,
		LastUpdate:      stats.LastUpdate,
		NodeStatus: map[string]int{
			"Healthy":  stats.HealthyNodes,
			"Warning":  stats.WarningNodes,
			"Critical": stats.CriticalNodes,
			"Unknown":  stats.UnknownNodes,
		},
	}

	for _, check := range stats.Checks {
		metricsSnapshot.Checks = append(metricsSnapshot.Checks, metrics.CheckStatusSnapshot{
			Name:     check.Name,
			Category: check.Category,
			Statuses: map[string]int{
				"Healthy":  check.HealthyCount,
				"Warning":  check.WarningCount,
				"Critical": check.CriticalCount,
				"Unknown":  check.UnknownCount,
			},
		})
	}

	// Helper function to deserialize RawExtension details to map
	deserializeDetails := func(cr *v1alpha1.CheckResult) map[string]interface{} {
		if cr == nil || len(cr.Details.Raw) == 0 {
			return nil
		}
		var details map[string]interface{}
		if err := json.Unmarshal(cr.Details.Raw, &details); err == nil {
			// Parse JSON strings back to objects where needed
			for k, v := range details {
				if str, ok := v.(string); ok {
					// Try to parse as JSON if it looks like JSON
					if (strings.HasPrefix(str, "{") && strings.HasSuffix(str, "}")) ||
					   (strings.HasPrefix(str, "[") && strings.HasSuffix(str, "]")) {
						var parsed interface{}
						if err := json.Unmarshal([]byte(str), &parsed); err == nil {
							details[k] = parsed
						}
					}
				}
			}
			return details
		}
		return nil
	}

	// Extract node-level metrics from NodeChecks
	nodeMetricsMap := make(map[string]*metrics.NodeMetricsSnapshot)
	for _, nc := range filteredNodeChecks {
		nodeName := nc.Spec.NodeName
		if nodeName == "" || nodeName == "*" || nodeName == "all" {
			continue
		}

		if nodeMetricsMap[nodeName] == nil {
			nodeMetricsMap[nodeName] = &metrics.NodeMetricsSnapshot{
				NodeName: nodeName,
			}
		}
		nodeMetrics := nodeMetricsMap[nodeName]

		systemResults := nc.Status.CheckResults.SystemResults

		// Extract temperature (average from all sensors)
		if systemResults.Hardware != nil && systemResults.Hardware.Temperature != nil {
			details := deserializeDetails(systemResults.Hardware.Temperature)
			if details != nil {
				if temps, ok := details["temperatures"].(map[string]interface{}); ok {
					sumTemp := 0.0
					count := 0
					for _, tempVal := range temps {
						if temp, ok := tempVal.(float64); ok && temp > 0 {
							sumTemp += temp
							count++
						}
					}
					if count > 0 {
						avgTemp := sumTemp / float64(count)
						nodeMetrics.Temperature = &avgTemp
					}
				}
			}
		}

		// Extract CPU usage
		if systemResults.Resources != nil {
			details := deserializeDetails(systemResults.Resources)
			if details != nil {
				var cpuValue float64
				if cpu, ok := details["cpu_usage"].(float64); ok {
					cpuValue = cpu
				} else if cpu, ok := details["cpuUsage"].(float64); ok {
					cpuValue = cpu
				} else if cpu, ok := details["cpu"].(float64); ok {
					cpuValue = cpu
				} else if cpuIdle, ok := details["cpu_idle_percent"].(float64); ok {
					cpuValue = 100 - cpuIdle
				} else if cpuUser, ok := details["cpu_user_percent"].(float64); ok {
					cpuSys := 0.0
					if cpuSysVal, ok := details["cpu_system_percent"].(float64); ok {
						cpuSys = cpuSysVal
					}
					cpuValue = cpuUser + cpuSys
				}
				if cpuValue > 0 {
					nodeMetrics.CPUUsage = &cpuValue
				}
			}
		}

		// Extract memory usage
		if systemResults.Memory != nil {
			details := deserializeDetails(systemResults.Memory)
			if details != nil {
				var memValue float64
				if mem, ok := details["memory_usage_percent"].(float64); ok {
					memValue = mem
				} else if mem, ok := details["memoryUsage"].(float64); ok {
					memValue = mem
				} else if mem, ok := details["used_percent"].(float64); ok {
					memValue = mem
				} else if usedKB, ok := details["memory_used_kb"].(float64); ok {
					if totalKB, ok := details["memory_total_kb"].(float64); ok && totalKB > 0 {
						memValue = (usedKB / totalKB) * 100
					}
				}
				if memValue > 0 {
					nodeMetrics.MemoryUsage = &memValue
				}
			}
		}

		// Extract uptime and load averages
		if systemResults.Uptime != nil {
			details := deserializeDetails(systemResults.Uptime)
			if details != nil {
				if load1m, ok := details["load_1min"].(float64); ok {
					nodeMetrics.LoadAverage1m = &load1m
				}
				if load5m, ok := details["load_5min"].(float64); ok {
					nodeMetrics.LoadAverage5m = &load5m
				}
				if load15m, ok := details["load_15min"].(float64); ok {
					nodeMetrics.LoadAverage15m = &load15m
				}
				// Parse uptime string to seconds (simplified - would need proper parsing)
				// For now, we'll skip uptime parsing as it requires parsing the uptime string
			}
		}
	}

	// Convert map to slice
	for _, nodeMetrics := range nodeMetricsMap {
		metricsSnapshot.Nodes = append(metricsSnapshot.Nodes, *nodeMetrics)
	}

	metrics.UpdateDashboardMetrics(metricsSnapshot)

	c.JSON(http.StatusOK, stats)
}

// updateCheckSummary updates a check summary with a status
func updateCheckSummary(summary *CheckSummary, status string) {
	switch status {
	case "Healthy":
		summary.HealthyCount++
	case "Warning":
		summary.WarningCount++
	case "Critical":
		summary.CriticalCount++
	default:
		summary.UnknownCount++
	}
}

// calculateOverallStatus determines the worst status
func calculateOverallStatus(healthy, warning, critical, unknown int) string {
	if critical > 0 {
		return "Critical"
	}
	if warning > 0 {
		return "Warning"
	}
	if unknown > 0 {
		return "Unknown"
	}
	return "Healthy"
}

// GetNodeChecks returns a list of all NodeChecks
func (api *DashboardAPI) GetNodeChecks(c *gin.Context) {
	ctx := context.Background()
	
	var nodeChecks v1alpha1.NodeCheckList
	if err := api.k8sClient.List(ctx, &nodeChecks); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	summaries := make([]NodeCheckSummary, len(nodeChecks.Items))
	for i, nc := range nodeChecks.Items {
		summary := NodeCheckSummary{
			Name:          nc.Name,
			Namespace:     nc.Namespace,
			NodeName:      nc.Spec.NodeName,
			OverallStatus: nc.Status.OverallStatus,
			LastCheck:     nc.Status.LastCheckTime.Time,
			Message:       nc.Status.Message,
		}

		// Count all check results
		// System checks
		if nc.Status.CheckResults.SystemResults.Uptime != nil {
			summary.CheckCount++
			countStatus(&summary, nc.Status.CheckResults.SystemResults.Uptime.Status)
		}
		if nc.Status.CheckResults.SystemResults.Processes != nil {
			summary.CheckCount++
			countStatus(&summary, nc.Status.CheckResults.SystemResults.Processes.Status)
		}
		if nc.Status.CheckResults.SystemResults.Resources != nil {
			summary.CheckCount++
			countStatus(&summary, nc.Status.CheckResults.SystemResults.Resources.Status)
		}
		if nc.Status.CheckResults.SystemResults.Memory != nil {
			summary.CheckCount++
			countStatus(&summary, nc.Status.CheckResults.SystemResults.Memory.Status)
		}
		if nc.Status.CheckResults.SystemResults.UninterruptibleTasks != nil {
			summary.CheckCount++
			countStatus(&summary, nc.Status.CheckResults.SystemResults.UninterruptibleTasks.Status)
		}
		if nc.Status.CheckResults.SystemResults.Services != nil {
			summary.CheckCount++
			countStatus(&summary, nc.Status.CheckResults.SystemResults.Services.Status)
		}
		if nc.Status.CheckResults.SystemResults.SystemLogs != nil {
			summary.CheckCount++
			countStatus(&summary, nc.Status.CheckResults.SystemResults.SystemLogs.Status)
		}
		if nc.Status.CheckResults.SystemResults.FileDescriptors != nil {
			summary.CheckCount++
			countStatus(&summary, nc.Status.CheckResults.SystemResults.FileDescriptors.Status)
		}
		if nc.Status.CheckResults.SystemResults.ZombieProcesses != nil {
			summary.CheckCount++
			countStatus(&summary, nc.Status.CheckResults.SystemResults.ZombieProcesses.Status)
		}
		if nc.Status.CheckResults.SystemResults.NTPSync != nil {
			summary.CheckCount++
			countStatus(&summary, nc.Status.CheckResults.SystemResults.NTPSync.Status)
		}
		if nc.Status.CheckResults.SystemResults.KernelPanics != nil {
			summary.CheckCount++
			countStatus(&summary, nc.Status.CheckResults.SystemResults.KernelPanics.Status)
		}
		if nc.Status.CheckResults.SystemResults.OOMKiller != nil {
			summary.CheckCount++
			countStatus(&summary, nc.Status.CheckResults.SystemResults.OOMKiller.Status)
		}
		if nc.Status.CheckResults.SystemResults.CPUFrequency != nil {
			summary.CheckCount++
			countStatus(&summary, nc.Status.CheckResults.SystemResults.CPUFrequency.Status)
		}
		if nc.Status.CheckResults.SystemResults.InterruptsBalance != nil {
			summary.CheckCount++
			countStatus(&summary, nc.Status.CheckResults.SystemResults.InterruptsBalance.Status)
		}
		if nc.Status.CheckResults.SystemResults.CPUStealTime != nil {
			summary.CheckCount++
			countStatus(&summary, nc.Status.CheckResults.SystemResults.CPUStealTime.Status)
		}
		if nc.Status.CheckResults.SystemResults.MemoryFragmentation != nil {
			summary.CheckCount++
			countStatus(&summary, nc.Status.CheckResults.SystemResults.MemoryFragmentation.Status)
		}
		if nc.Status.CheckResults.SystemResults.SwapActivity != nil {
			summary.CheckCount++
			countStatus(&summary, nc.Status.CheckResults.SystemResults.SwapActivity.Status)
		}
		if nc.Status.CheckResults.SystemResults.ContextSwitches != nil {
			summary.CheckCount++
			countStatus(&summary, nc.Status.CheckResults.SystemResults.ContextSwitches.Status)
		}
		if nc.Status.CheckResults.SystemResults.SELinuxStatus != nil {
			summary.CheckCount++
			countStatus(&summary, nc.Status.CheckResults.SystemResults.SELinuxStatus.Status)
		}
		if nc.Status.CheckResults.SystemResults.SSHAccess != nil {
			summary.CheckCount++
			countStatus(&summary, nc.Status.CheckResults.SystemResults.SSHAccess.Status)
		}
		if nc.Status.CheckResults.SystemResults.KernelModules != nil {
			summary.CheckCount++
			countStatus(&summary, nc.Status.CheckResults.SystemResults.KernelModules.Status)
		}
		
		// Hardware checks
		if nc.Status.CheckResults.SystemResults.Hardware != nil {
			if nc.Status.CheckResults.SystemResults.Hardware.Temperature != nil {
				summary.CheckCount++
				countStatus(&summary, nc.Status.CheckResults.SystemResults.Hardware.Temperature.Status)
			}
			if nc.Status.CheckResults.SystemResults.Hardware.IPMI != nil {
				summary.CheckCount++
				countStatus(&summary, nc.Status.CheckResults.SystemResults.Hardware.IPMI.Status)
			}
			if nc.Status.CheckResults.SystemResults.Hardware.BMC != nil {
				summary.CheckCount++
				countStatus(&summary, nc.Status.CheckResults.SystemResults.Hardware.BMC.Status)
			}
			if nc.Status.CheckResults.SystemResults.Hardware.FanStatus != nil {
				summary.CheckCount++
				countStatus(&summary, nc.Status.CheckResults.SystemResults.Hardware.FanStatus.Status)
			}
			if nc.Status.CheckResults.SystemResults.Hardware.PowerSupply != nil {
				summary.CheckCount++
				countStatus(&summary, nc.Status.CheckResults.SystemResults.Hardware.PowerSupply.Status)
			}
			if nc.Status.CheckResults.SystemResults.Hardware.MemoryErrors != nil {
				summary.CheckCount++
				countStatus(&summary, nc.Status.CheckResults.SystemResults.Hardware.MemoryErrors.Status)
			}
			if nc.Status.CheckResults.SystemResults.Hardware.PCIeErrors != nil {
				summary.CheckCount++
				countStatus(&summary, nc.Status.CheckResults.SystemResults.Hardware.PCIeErrors.Status)
			}
			if nc.Status.CheckResults.SystemResults.Hardware.CPUMicrocode != nil {
				summary.CheckCount++
				countStatus(&summary, nc.Status.CheckResults.SystemResults.Hardware.CPUMicrocode.Status)
			}
		}
		
		// Disk checks
		if nc.Status.CheckResults.SystemResults.Disks != nil {
			if nc.Status.CheckResults.SystemResults.Disks.Space != nil {
				summary.CheckCount++
				countStatus(&summary, nc.Status.CheckResults.SystemResults.Disks.Space.Status)
			}
			if nc.Status.CheckResults.SystemResults.Disks.SMART != nil {
				summary.CheckCount++
				countStatus(&summary, nc.Status.CheckResults.SystemResults.Disks.SMART.Status)
			}
			if nc.Status.CheckResults.SystemResults.Disks.Performance != nil {
				summary.CheckCount++
				countStatus(&summary, nc.Status.CheckResults.SystemResults.Disks.Performance.Status)
			}
			if nc.Status.CheckResults.SystemResults.Disks.RAID != nil {
				summary.CheckCount++
				countStatus(&summary, nc.Status.CheckResults.SystemResults.Disks.RAID.Status)
			}
			if nc.Status.CheckResults.SystemResults.Disks.PVs != nil {
				summary.CheckCount++
				countStatus(&summary, nc.Status.CheckResults.SystemResults.Disks.PVs.Status)
			}
			if nc.Status.CheckResults.SystemResults.Disks.LVM != nil {
				summary.CheckCount++
				countStatus(&summary, nc.Status.CheckResults.SystemResults.Disks.LVM.Status)
			}
			if nc.Status.CheckResults.SystemResults.Disks.IOWait != nil {
				summary.CheckCount++
				countStatus(&summary, nc.Status.CheckResults.SystemResults.Disks.IOWait.Status)
			}
			if nc.Status.CheckResults.SystemResults.Disks.QueueDepth != nil {
				summary.CheckCount++
				countStatus(&summary, nc.Status.CheckResults.SystemResults.Disks.QueueDepth.Status)
			}
			if nc.Status.CheckResults.SystemResults.Disks.FilesystemErrors != nil {
				summary.CheckCount++
				countStatus(&summary, nc.Status.CheckResults.SystemResults.Disks.FilesystemErrors.Status)
			}
			if nc.Status.CheckResults.SystemResults.Disks.InodeUsage != nil {
				summary.CheckCount++
				countStatus(&summary, nc.Status.CheckResults.SystemResults.Disks.InodeUsage.Status)
			}
			if nc.Status.CheckResults.SystemResults.Disks.MountPoints != nil {
				summary.CheckCount++
				countStatus(&summary, nc.Status.CheckResults.SystemResults.Disks.MountPoints.Status)
			}
		}
		
		// Network checks
		if nc.Status.CheckResults.SystemResults.Network != nil {
			if nc.Status.CheckResults.SystemResults.Network.Interfaces != nil {
				summary.CheckCount++
				countStatus(&summary, nc.Status.CheckResults.SystemResults.Network.Interfaces.Status)
			}
			if nc.Status.CheckResults.SystemResults.Network.Routing != nil {
				summary.CheckCount++
				countStatus(&summary, nc.Status.CheckResults.SystemResults.Network.Routing.Status)
			}
			if nc.Status.CheckResults.SystemResults.Network.Connectivity != nil {
				summary.CheckCount++
				countStatus(&summary, nc.Status.CheckResults.SystemResults.Network.Connectivity.Status)
			}
			if nc.Status.CheckResults.SystemResults.Network.Statistics != nil {
				summary.CheckCount++
				countStatus(&summary, nc.Status.CheckResults.SystemResults.Network.Statistics.Status)
			}
			if nc.Status.CheckResults.SystemResults.Network.Errors != nil {
				summary.CheckCount++
				countStatus(&summary, nc.Status.CheckResults.SystemResults.Network.Errors.Status)
			}
			if nc.Status.CheckResults.SystemResults.Network.Latency != nil {
				summary.CheckCount++
				countStatus(&summary, nc.Status.CheckResults.SystemResults.Network.Latency.Status)
			}
			if nc.Status.CheckResults.SystemResults.Network.DNSResolution != nil {
				summary.CheckCount++
				countStatus(&summary, nc.Status.CheckResults.SystemResults.Network.DNSResolution.Status)
			}
			if nc.Status.CheckResults.SystemResults.Network.BondingStatus != nil {
				summary.CheckCount++
				countStatus(&summary, nc.Status.CheckResults.SystemResults.Network.BondingStatus.Status)
			}
			if nc.Status.CheckResults.SystemResults.Network.FirewallRules != nil {
				summary.CheckCount++
				countStatus(&summary, nc.Status.CheckResults.SystemResults.Network.FirewallRules.Status)
			}
		}
		
		// Kubernetes checks
		if nc.Status.CheckResults.KubernetesResults.NodeStatus != nil {
			summary.CheckCount++
			countStatus(&summary, nc.Status.CheckResults.KubernetesResults.NodeStatus.Status)
		}
		if nc.Status.CheckResults.KubernetesResults.Pods != nil {
			summary.CheckCount++
			countStatus(&summary, nc.Status.CheckResults.KubernetesResults.Pods.Status)
		}
		if nc.Status.CheckResults.KubernetesResults.ClusterOperators != nil {
			summary.CheckCount++
			countStatus(&summary, nc.Status.CheckResults.KubernetesResults.ClusterOperators.Status)
		}
		if nc.Status.CheckResults.KubernetesResults.NodeResources != nil {
			summary.CheckCount++
			countStatus(&summary, nc.Status.CheckResults.KubernetesResults.NodeResources.Status)
		}
		if nc.Status.CheckResults.KubernetesResults.NodeResourceUsage != nil {
			summary.CheckCount++
			countStatus(&summary, nc.Status.CheckResults.KubernetesResults.NodeResourceUsage.Status)
		}
		if nc.Status.CheckResults.KubernetesResults.ContainerRuntime != nil {
			summary.CheckCount++
			countStatus(&summary, nc.Status.CheckResults.KubernetesResults.ContainerRuntime.Status)
		}
		if nc.Status.CheckResults.KubernetesResults.KubeletHealth != nil {
			summary.CheckCount++
			countStatus(&summary, nc.Status.CheckResults.KubernetesResults.KubeletHealth.Status)
		}
		if nc.Status.CheckResults.KubernetesResults.CNIPlugin != nil {
			summary.CheckCount++
			countStatus(&summary, nc.Status.CheckResults.KubernetesResults.CNIPlugin.Status)
		}
		if nc.Status.CheckResults.KubernetesResults.NodeConditions != nil {
			summary.CheckCount++
			countStatus(&summary, nc.Status.CheckResults.KubernetesResults.NodeConditions.Status)
		}

		summaries[i] = summary
	}

	c.JSON(http.StatusOK, summaries)
}

// GetNodeCheckDetail returns detailed information about a specific NodeCheck
func (api *DashboardAPI) GetNodeCheckDetail(c *gin.Context) {
	ctx := context.Background()
	
	name := c.Param("name")
	// Usa node-check-operator-system come namespace di default invece di "default"
	namespace := c.DefaultQuery("namespace", "node-check-operator-system")

	var nodeCheck v1alpha1.NodeCheck
	key := client.ObjectKey{Name: name, Namespace: namespace}
	
	if err := api.k8sClient.Get(ctx, key, &nodeCheck); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "NodeCheck not found"})
		return
	}

	summary := NodeCheckSummary{
			Name:          nodeCheck.Name,
			Namespace:     nodeCheck.Namespace,
			NodeName:      nodeCheck.Spec.NodeName,
			OverallStatus: nodeCheck.Status.OverallStatus,
			LastCheck:     nodeCheck.Status.LastCheckTime.Time,
			Message:       nodeCheck.Status.Message,
	}

	// Count all check results (same logic as GetNodeChecks)
	// System checks
	if nodeCheck.Status.CheckResults.SystemResults.Uptime != nil {
		summary.CheckCount++
		countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.Uptime.Status)
	}
	if nodeCheck.Status.CheckResults.SystemResults.Processes != nil {
		summary.CheckCount++
		countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.Processes.Status)
	}
	if nodeCheck.Status.CheckResults.SystemResults.Resources != nil {
		summary.CheckCount++
		countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.Resources.Status)
	}
	if nodeCheck.Status.CheckResults.SystemResults.Memory != nil {
		summary.CheckCount++
		countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.Memory.Status)
	}
	if nodeCheck.Status.CheckResults.SystemResults.UninterruptibleTasks != nil {
		summary.CheckCount++
		countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.UninterruptibleTasks.Status)
	}
	if nodeCheck.Status.CheckResults.SystemResults.Services != nil {
		summary.CheckCount++
		countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.Services.Status)
	}
	if nodeCheck.Status.CheckResults.SystemResults.SystemLogs != nil {
		summary.CheckCount++
		countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.SystemLogs.Status)
	}
	if nodeCheck.Status.CheckResults.SystemResults.FileDescriptors != nil {
		summary.CheckCount++
		countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.FileDescriptors.Status)
	}
	if nodeCheck.Status.CheckResults.SystemResults.ZombieProcesses != nil {
		summary.CheckCount++
		countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.ZombieProcesses.Status)
	}
	if nodeCheck.Status.CheckResults.SystemResults.NTPSync != nil {
		summary.CheckCount++
		countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.NTPSync.Status)
	}
	if nodeCheck.Status.CheckResults.SystemResults.KernelPanics != nil {
		summary.CheckCount++
		countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.KernelPanics.Status)
	}
	if nodeCheck.Status.CheckResults.SystemResults.OOMKiller != nil {
		summary.CheckCount++
		countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.OOMKiller.Status)
	}
	if nodeCheck.Status.CheckResults.SystemResults.CPUFrequency != nil {
		summary.CheckCount++
		countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.CPUFrequency.Status)
	}
	if nodeCheck.Status.CheckResults.SystemResults.InterruptsBalance != nil {
		summary.CheckCount++
		countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.InterruptsBalance.Status)
	}
	if nodeCheck.Status.CheckResults.SystemResults.CPUStealTime != nil {
		summary.CheckCount++
		countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.CPUStealTime.Status)
	}
	if nodeCheck.Status.CheckResults.SystemResults.MemoryFragmentation != nil {
		summary.CheckCount++
		countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.MemoryFragmentation.Status)
	}
	if nodeCheck.Status.CheckResults.SystemResults.SwapActivity != nil {
		summary.CheckCount++
		countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.SwapActivity.Status)
	}
	if nodeCheck.Status.CheckResults.SystemResults.ContextSwitches != nil {
		summary.CheckCount++
		countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.ContextSwitches.Status)
	}
	if nodeCheck.Status.CheckResults.SystemResults.SELinuxStatus != nil {
		summary.CheckCount++
		countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.SELinuxStatus.Status)
	}
	if nodeCheck.Status.CheckResults.SystemResults.SSHAccess != nil {
		summary.CheckCount++
		countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.SSHAccess.Status)
	}
	if nodeCheck.Status.CheckResults.SystemResults.KernelModules != nil {
		summary.CheckCount++
		countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.KernelModules.Status)
	}
	
	// Hardware checks
	if nodeCheck.Status.CheckResults.SystemResults.Hardware != nil {
		if nodeCheck.Status.CheckResults.SystemResults.Hardware.Temperature != nil {
			summary.CheckCount++
			countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.Hardware.Temperature.Status)
		}
		if nodeCheck.Status.CheckResults.SystemResults.Hardware.IPMI != nil {
			summary.CheckCount++
			countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.Hardware.IPMI.Status)
		}
		if nodeCheck.Status.CheckResults.SystemResults.Hardware.BMC != nil {
			summary.CheckCount++
			countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.Hardware.BMC.Status)
		}
		if nodeCheck.Status.CheckResults.SystemResults.Hardware.FanStatus != nil {
			summary.CheckCount++
			countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.Hardware.FanStatus.Status)
		}
		if nodeCheck.Status.CheckResults.SystemResults.Hardware.PowerSupply != nil {
			summary.CheckCount++
			countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.Hardware.PowerSupply.Status)
		}
		if nodeCheck.Status.CheckResults.SystemResults.Hardware.MemoryErrors != nil {
			summary.CheckCount++
			countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.Hardware.MemoryErrors.Status)
		}
		if nodeCheck.Status.CheckResults.SystemResults.Hardware.PCIeErrors != nil {
			summary.CheckCount++
			countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.Hardware.PCIeErrors.Status)
		}
		if nodeCheck.Status.CheckResults.SystemResults.Hardware.CPUMicrocode != nil {
			summary.CheckCount++
			countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.Hardware.CPUMicrocode.Status)
		}
	}
	
	// Disk checks
	if nodeCheck.Status.CheckResults.SystemResults.Disks != nil {
		if nodeCheck.Status.CheckResults.SystemResults.Disks.Space != nil {
			summary.CheckCount++
			countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.Disks.Space.Status)
		}
		if nodeCheck.Status.CheckResults.SystemResults.Disks.SMART != nil {
			summary.CheckCount++
			countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.Disks.SMART.Status)
		}
		if nodeCheck.Status.CheckResults.SystemResults.Disks.Performance != nil {
			summary.CheckCount++
			countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.Disks.Performance.Status)
		}
		if nodeCheck.Status.CheckResults.SystemResults.Disks.RAID != nil {
			summary.CheckCount++
			countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.Disks.RAID.Status)
		}
		if nodeCheck.Status.CheckResults.SystemResults.Disks.PVs != nil {
			summary.CheckCount++
			countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.Disks.PVs.Status)
		}
		if nodeCheck.Status.CheckResults.SystemResults.Disks.LVM != nil {
			summary.CheckCount++
			countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.Disks.LVM.Status)
		}
		if nodeCheck.Status.CheckResults.SystemResults.Disks.IOWait != nil {
			summary.CheckCount++
			countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.Disks.IOWait.Status)
		}
		if nodeCheck.Status.CheckResults.SystemResults.Disks.QueueDepth != nil {
			summary.CheckCount++
			countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.Disks.QueueDepth.Status)
		}
		if nodeCheck.Status.CheckResults.SystemResults.Disks.FilesystemErrors != nil {
			summary.CheckCount++
			countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.Disks.FilesystemErrors.Status)
		}
		if nodeCheck.Status.CheckResults.SystemResults.Disks.InodeUsage != nil {
			summary.CheckCount++
			countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.Disks.InodeUsage.Status)
		}
		if nodeCheck.Status.CheckResults.SystemResults.Disks.MountPoints != nil {
			summary.CheckCount++
			countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.Disks.MountPoints.Status)
		}
	}
	
	// Network checks
	if nodeCheck.Status.CheckResults.SystemResults.Network != nil {
		if nodeCheck.Status.CheckResults.SystemResults.Network.Interfaces != nil {
			summary.CheckCount++
			countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.Network.Interfaces.Status)
		}
		if nodeCheck.Status.CheckResults.SystemResults.Network.Routing != nil {
			summary.CheckCount++
			countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.Network.Routing.Status)
		}
		if nodeCheck.Status.CheckResults.SystemResults.Network.Connectivity != nil {
			summary.CheckCount++
			countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.Network.Connectivity.Status)
		}
		if nodeCheck.Status.CheckResults.SystemResults.Network.Statistics != nil {
			summary.CheckCount++
			countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.Network.Statistics.Status)
		}
		if nodeCheck.Status.CheckResults.SystemResults.Network.Errors != nil {
			summary.CheckCount++
			countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.Network.Errors.Status)
		}
		if nodeCheck.Status.CheckResults.SystemResults.Network.Latency != nil {
			summary.CheckCount++
			countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.Network.Latency.Status)
		}
		if nodeCheck.Status.CheckResults.SystemResults.Network.DNSResolution != nil {
			summary.CheckCount++
			countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.Network.DNSResolution.Status)
		}
		if nodeCheck.Status.CheckResults.SystemResults.Network.BondingStatus != nil {
			summary.CheckCount++
			countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.Network.BondingStatus.Status)
		}
		if nodeCheck.Status.CheckResults.SystemResults.Network.FirewallRules != nil {
			summary.CheckCount++
			countStatus(&summary, nodeCheck.Status.CheckResults.SystemResults.Network.FirewallRules.Status)
		}
	}
	
	// Kubernetes checks
	if nodeCheck.Status.CheckResults.KubernetesResults.NodeStatus != nil {
		summary.CheckCount++
		countStatus(&summary, nodeCheck.Status.CheckResults.KubernetesResults.NodeStatus.Status)
	}
	if nodeCheck.Status.CheckResults.KubernetesResults.Pods != nil {
		summary.CheckCount++
		countStatus(&summary, nodeCheck.Status.CheckResults.KubernetesResults.Pods.Status)
	}
	if nodeCheck.Status.CheckResults.KubernetesResults.ClusterOperators != nil {
		summary.CheckCount++
		countStatus(&summary, nodeCheck.Status.CheckResults.KubernetesResults.ClusterOperators.Status)
	}
	if nodeCheck.Status.CheckResults.KubernetesResults.NodeResources != nil {
		summary.CheckCount++
		countStatus(&summary, nodeCheck.Status.CheckResults.KubernetesResults.NodeResources.Status)
	}
	if nodeCheck.Status.CheckResults.KubernetesResults.NodeResourceUsage != nil {
		summary.CheckCount++
		countStatus(&summary, nodeCheck.Status.CheckResults.KubernetesResults.NodeResourceUsage.Status)
	}
	if nodeCheck.Status.CheckResults.KubernetesResults.ContainerRuntime != nil {
		summary.CheckCount++
		countStatus(&summary, nodeCheck.Status.CheckResults.KubernetesResults.ContainerRuntime.Status)
	}
	if nodeCheck.Status.CheckResults.KubernetesResults.KubeletHealth != nil {
		summary.CheckCount++
		countStatus(&summary, nodeCheck.Status.CheckResults.KubernetesResults.KubeletHealth.Status)
	}
	if nodeCheck.Status.CheckResults.KubernetesResults.CNIPlugin != nil {
		summary.CheckCount++
		countStatus(&summary, nodeCheck.Status.CheckResults.KubernetesResults.CNIPlugin.Status)
	}
	if nodeCheck.Status.CheckResults.KubernetesResults.NodeConditions != nil {
		summary.CheckCount++
		countStatus(&summary, nodeCheck.Status.CheckResults.KubernetesResults.NodeConditions.Status)
	}

	// Convert CheckResult to CheckResultAPI (deserialize RawExtension details)
	convertCheckResult := func(cr *v1alpha1.CheckResult) *CheckResultAPI {
		if cr == nil {
			return nil
		}
		result := &CheckResultAPI{
			Status:    cr.Status,
			Message:   cr.Message,
			Timestamp: cr.Timestamp.Format(time.RFC3339),
			Command:   cr.Command,
		}
		
		// Deserialize RawExtension details to map
		if len(cr.Details.Raw) > 0 {
			var details map[string]interface{}
			if err := json.Unmarshal(cr.Details.Raw, &details); err == nil {
				// Parse JSON strings back to objects where needed
				for k, v := range details {
					if str, ok := v.(string); ok {
						// Try to parse as JSON if it looks like JSON
						if (strings.HasPrefix(str, "{") && strings.HasSuffix(str, "}")) ||
						   (strings.HasPrefix(str, "[") && strings.HasSuffix(str, "]")) {
							var parsed interface{}
							if err := json.Unmarshal([]byte(str), &parsed); err == nil {
								details[k] = parsed
							}
						}
					}
				}
				result.Details = details
			}
		}
		return result
	}

	// Convert SystemCheckResults
	var systemResultsAPI *SystemCheckResultsAPI
	if nodeCheck.Status.CheckResults.SystemResults.Uptime != nil ||
		nodeCheck.Status.CheckResults.SystemResults.Processes != nil ||
		nodeCheck.Status.CheckResults.SystemResults.Memory != nil ||
		nodeCheck.Status.CheckResults.SystemResults.UninterruptibleTasks != nil ||
		nodeCheck.Status.CheckResults.SystemResults.FileDescriptors != nil ||
		nodeCheck.Status.CheckResults.SystemResults.ZombieProcesses != nil ||
		nodeCheck.Status.CheckResults.SystemResults.NTPSync != nil ||
		nodeCheck.Status.CheckResults.SystemResults.KernelPanics != nil ||
		nodeCheck.Status.CheckResults.SystemResults.OOMKiller != nil ||
		nodeCheck.Status.CheckResults.SystemResults.CPUFrequency != nil ||
		nodeCheck.Status.CheckResults.SystemResults.InterruptsBalance != nil ||
		nodeCheck.Status.CheckResults.SystemResults.CPUStealTime != nil ||
		nodeCheck.Status.CheckResults.SystemResults.MemoryFragmentation != nil ||
		nodeCheck.Status.CheckResults.SystemResults.SwapActivity != nil ||
		nodeCheck.Status.CheckResults.SystemResults.ContextSwitches != nil ||
		nodeCheck.Status.CheckResults.SystemResults.SELinuxStatus != nil ||
		nodeCheck.Status.CheckResults.SystemResults.SSHAccess != nil ||
		nodeCheck.Status.CheckResults.SystemResults.KernelModules != nil ||
		nodeCheck.Status.CheckResults.SystemResults.Hardware != nil ||
		nodeCheck.Status.CheckResults.SystemResults.Disks != nil ||
		nodeCheck.Status.CheckResults.SystemResults.Network != nil {
		systemResultsAPI = &SystemCheckResultsAPI{
			Uptime:              convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.Uptime),
			Processes:           convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.Processes),
			Resources:           convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.Resources),
			Services:            convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.Services),
			Memory:              convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.Memory),
			UninterruptibleTasks: convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.UninterruptibleTasks),
			SystemLogs:          convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.SystemLogs),
			FileDescriptors:     convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.FileDescriptors),
			ZombieProcesses:     convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.ZombieProcesses),
			NTPSync:             convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.NTPSync),
			KernelPanics:        convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.KernelPanics),
			OOMKiller:           convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.OOMKiller),
			CPUFrequency:        convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.CPUFrequency),
			InterruptsBalance:   convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.InterruptsBalance),
			CPUStealTime:        convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.CPUStealTime),
			MemoryFragmentation: convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.MemoryFragmentation),
			SwapActivity:        convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.SwapActivity),
			ContextSwitches:     convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.ContextSwitches),
			SELinuxStatus:       convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.SELinuxStatus),
			SSHAccess:           convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.SSHAccess),
			KernelModules:       convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.KernelModules),
		}
		
		if nodeCheck.Status.CheckResults.SystemResults.Hardware != nil {
			systemResultsAPI.Hardware = &HardwareCheckResultsAPI{
				Temperature: convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.Hardware.Temperature),
				IPMI:        convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.Hardware.IPMI),
				BMC:         convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.Hardware.BMC),
				FanStatus:   convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.Hardware.FanStatus),
				PowerSupply: convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.Hardware.PowerSupply),
				MemoryErrors: convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.Hardware.MemoryErrors),
				PCIeErrors:  convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.Hardware.PCIeErrors),
				CPUMicrocode: convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.Hardware.CPUMicrocode),
			}
		}
		
		if nodeCheck.Status.CheckResults.SystemResults.Disks != nil {
			systemResultsAPI.Disks = &DiskCheckResultsAPI{
				Space:            convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.Disks.Space),
				SMART:            convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.Disks.SMART),
				Performance:      convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.Disks.Performance),
				RAID:             convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.Disks.RAID),
				PVs:              convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.Disks.PVs),
				LVM:              convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.Disks.LVM),
				IOWait:           convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.Disks.IOWait),
				QueueDepth:       convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.Disks.QueueDepth),
				FilesystemErrors: convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.Disks.FilesystemErrors),
				InodeUsage:       convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.Disks.InodeUsage),
				MountPoints:      convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.Disks.MountPoints),
			}
		}
		
		if nodeCheck.Status.CheckResults.SystemResults.Network != nil {
			systemResultsAPI.Network = &NetworkCheckResultsAPI{
				Interfaces:    convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.Network.Interfaces),
				Routing:       convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.Network.Routing),
				Connectivity:  convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.Network.Connectivity),
				Statistics:    convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.Network.Statistics),
				Errors:        convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.Network.Errors),
				Latency:       convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.Network.Latency),
				DNSResolution: convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.Network.DNSResolution),
				BondingStatus: convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.Network.BondingStatus),
				FirewallRules: convertCheckResult(nodeCheck.Status.CheckResults.SystemResults.Network.FirewallRules),
			}
		}
	}

	// Convert KubernetesCheckResults
	var kubernetesResultsAPI *KubernetesCheckResultsAPI
	if nodeCheck.Status.CheckResults.KubernetesResults.NodeStatus != nil ||
		nodeCheck.Status.CheckResults.KubernetesResults.Pods != nil ||
		nodeCheck.Status.CheckResults.KubernetesResults.ClusterOperators != nil ||
		nodeCheck.Status.CheckResults.KubernetesResults.NodeResources != nil ||
		nodeCheck.Status.CheckResults.KubernetesResults.NodeResourceUsage != nil ||
		nodeCheck.Status.CheckResults.KubernetesResults.ContainerRuntime != nil ||
		nodeCheck.Status.CheckResults.KubernetesResults.KubeletHealth != nil ||
		nodeCheck.Status.CheckResults.KubernetesResults.CNIPlugin != nil ||
		nodeCheck.Status.CheckResults.KubernetesResults.NodeConditions != nil {
		kubernetesResultsAPI = &KubernetesCheckResultsAPI{
			NodeStatus:         convertCheckResult(nodeCheck.Status.CheckResults.KubernetesResults.NodeStatus),
			Pods:               convertCheckResult(nodeCheck.Status.CheckResults.KubernetesResults.Pods),
			ClusterOperators:   convertCheckResult(nodeCheck.Status.CheckResults.KubernetesResults.ClusterOperators),
			NodeResources:      convertCheckResult(nodeCheck.Status.CheckResults.KubernetesResults.NodeResources),
			NodeResourceUsage:  convertCheckResult(nodeCheck.Status.CheckResults.KubernetesResults.NodeResourceUsage),
			ContainerRuntime:   convertCheckResult(nodeCheck.Status.CheckResults.KubernetesResults.ContainerRuntime),
			KubeletHealth:      convertCheckResult(nodeCheck.Status.CheckResults.KubernetesResults.KubeletHealth),
			CNIPlugin:          convertCheckResult(nodeCheck.Status.CheckResults.KubernetesResults.CNIPlugin),
			NodeConditions:     convertCheckResult(nodeCheck.Status.CheckResults.KubernetesResults.NodeConditions),
		}
	}

	detail := NodeCheckDetail{
		NodeCheckSummary:  summary,
		SystemResults:     systemResultsAPI,
		KubernetesResults: kubernetesResultsAPI,
	}

	// Debug: log per verificare che i dati siano presenti
	if detail.SystemResults != nil {
		fmt.Printf("DEBUG: SystemResults presente, Uptime: %v, Processes: %v, Memory: %v\n",
			detail.SystemResults.Uptime != nil,
			detail.SystemResults.Processes != nil,
			detail.SystemResults.Memory != nil)
		if detail.SystemResults.Disks != nil {
			fmt.Printf("DEBUG: Disks presente, Space: %v, SMART: %v, LVM: %v\n",
				detail.SystemResults.Disks.Space != nil,
				detail.SystemResults.Disks.SMART != nil,
				detail.SystemResults.Disks.LVM != nil)
		}
		if detail.SystemResults.Network != nil {
			fmt.Printf("DEBUG: Network presente, Interfaces: %v\n",
				detail.SystemResults.Network.Interfaces != nil)
		}
		if detail.SystemResults.Hardware != nil {
			fmt.Printf("DEBUG: Hardware presente, Temperature: %v\n",
				detail.SystemResults.Hardware.Temperature != nil)
		}
	}

	// Debug: serializza in JSON per vedere cosa viene restituito
	jsonData, err := json.Marshal(detail)
	if err != nil {
		fmt.Printf("DEBUG: Errore serializzazione JSON: %v\n", err)
	} else {
		// Mostra solo i primi 2000 caratteri per non intasare i log
		jsonStr := string(jsonData)
		if len(jsonStr) > 2000 {
			fmt.Printf("DEBUG: JSON restituito (primi 2000 caratteri): %s...\n", jsonStr[:2000])
		} else {
			fmt.Printf("DEBUG: JSON restituito: %s\n", jsonStr)
		}
	}

	c.JSON(http.StatusOK, detail)
}

// GetNodeCheckHistory returns historical data for a NodeCheck
func (api *DashboardAPI) GetNodeCheckHistory(c *gin.Context) {
	hours := c.DefaultQuery("hours", "24")
	
	hoursInt, err := strconv.Atoi(hours)
	if err != nil {
		hoursInt = 24
	}

	// This would typically query a time-series database
	// For now, we'll return mock data
	history := []map[string]interface{}{
		{
			"timestamp": time.Now().Add(-time.Duration(hoursInt) * time.Hour),
			"status":    "Healthy",
			"uptime":    99.5,
			"cpu":       45.2,
			"memory":    67.8,
		},
		{
			"timestamp": time.Now().Add(-time.Duration(hoursInt-1) * time.Hour),
			"status":    "Healthy",
			"uptime":    99.2,
			"cpu":       52.1,
			"memory":    71.3,
		},
		{
			"timestamp": time.Now().Add(-time.Duration(hoursInt-2) * time.Hour),
			"status":    "Warning",
			"uptime":    98.8,
			"cpu":       78.9,
			"memory":    85.2,
		},
	}

	c.JSON(http.StatusOK, history)
}

// GetNodeInfo returns information about a specific node
func (api *DashboardAPI) GetNodeInfo(c *gin.Context) {
	ctx := context.Background()
	
	nodeName := c.Param("nodeName")
	
	node, err := api.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
		return
	}

	nodeInfo := map[string]interface{}{
		"name":           node.Name,
		"creationTime":   node.CreationTimestamp,
		"labels":         node.Labels,
		"annotations":    node.Annotations,
		"taints":         node.Spec.Taints,
		"unschedulable": node.Spec.Unschedulable,
		"conditions":     node.Status.Conditions,
		"capacity":       node.Status.Capacity,
		"allocatable":   node.Status.Allocatable,
	}

	c.JSON(http.StatusOK, nodeInfo)
}

// GetPodsOnNode returns pods running on a specific node
func (api *DashboardAPI) GetPodsOnNode(c *gin.Context) {
	ctx := context.Background()
	
	nodeName := c.Param("nodeName")
	
	pods, err := api.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	podList := make([]map[string]interface{}, len(pods.Items))
	for i, pod := range pods.Items {
		podList[i] = map[string]interface{}{
			"name":         pod.Name,
			"namespace":    pod.Namespace,
			"phase":        pod.Status.Phase,
			"creationTime": pod.CreationTimestamp,
			"restartCount": 0, // Calculate from container statuses
		}
	}

	c.JSON(http.StatusOK, podList)
}

// SetupRoutes sets up the API routes
func (api *DashboardAPI) SetupRoutes(r *gin.Engine) {
	// Main API group with /api/v1 prefix
	// This is the standard route structure for the dashboard API
	apiGroup := r.Group("/api/v1")
	{
		apiGroup.GET("/stats", api.GetDashboardStats)
		apiGroup.GET("/nodechecks", api.GetNodeChecks)
		apiGroup.GET("/nodechecks/:name", api.GetNodeCheckDetail)
		apiGroup.GET("/nodechecks/:name/history", api.GetNodeCheckHistory)
		apiGroup.GET("/nodes/:nodeName", api.GetNodeInfo)
		apiGroup.GET("/nodes/:nodeName/pods", api.GetPodsOnNode)
	}
	
	// Fallback routes without /api/v1/ prefix
	// These handle cases where the proxy might strip the prefix
	// (though with correct plugin configuration, this shouldn't be needed)
	fallbackGroup := r.Group("")
	{
		fallbackGroup.GET("/stats", api.GetDashboardStats)
		fallbackGroup.GET("/nodechecks", api.GetNodeChecks)
		fallbackGroup.GET("/nodechecks/:name", api.GetNodeCheckDetail)
		fallbackGroup.GET("/nodechecks/:name/history", api.GetNodeCheckHistory)
		fallbackGroup.GET("/nodes/:nodeName", api.GetNodeInfo)
		fallbackGroup.GET("/nodes/:nodeName/pods", api.GetPodsOnNode)
	}
}
