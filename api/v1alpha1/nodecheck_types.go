package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// CheckResult represents the result of a single check
type CheckResult struct {
	// Status indicates the overall status of the check
	Status string `json:"status"`

	// Message provides details about the check result
	Message string `json:"message,omitempty"`

	// Timestamp when the check was performed
	Timestamp metav1.Time `json:"timestamp"`

	// Details provides additional information about the check
	Details runtime.RawExtension `json:"details,omitempty"`
}

// NodeCheckSpec defines the desired state of NodeCheck
type NodeCheckSpec struct {
	// NodeName is the name of the node to check.
	// - If empty or not specified, each executor pod will automatically detect the node where it's running
	//   and create/update a NodeCheck for that specific node.
	// - Use "*" or "all" to check all nodes in the cluster (creates child NodeChecks for each node).
	// - Use a specific node name to check only that node.
	NodeName string `json:"nodeName,omitempty"`

	// CheckInterval is the interval between checks in minutes
	CheckInterval int `json:"checkInterval,omitempty"`

	// NodeSelector is a label query over nodes that determines which nodes the executor DaemonSet
	// should be scheduled on. If specified, the DaemonSet will only run on nodes matching the selector.
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations allow the executor DaemonSet to be scheduled on nodes with matching taints.
	// If specified, the DaemonSet pods will tolerate the listed taints.
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// SystemChecks defines which system-level checks to perform
	SystemChecks SystemChecks `json:"systemChecks,omitempty"`

	// KubernetesChecks defines which Kubernetes-level checks to perform
	KubernetesChecks KubernetesChecks `json:"kubernetesChecks,omitempty"`
}

// SystemChecks defines system-level checks
type SystemChecks struct {
	Uptime              bool           `json:"uptime,omitempty"`
	Processes           bool           `json:"processes,omitempty"`
	Resources           bool           `json:"resources,omitempty"`
	Services            bool           `json:"services,omitempty"`
	Memory              bool           `json:"memory,omitempty"`
	UninterruptibleTasks bool          `json:"uninterruptibleTasks,omitempty"`
	SystemLogs          bool           `json:"systemLogs,omitempty"`
	FileDescriptors     bool           `json:"fileDescriptors,omitempty"`
	ZombieProcesses     bool           `json:"zombieProcesses,omitempty"`
	NTPSync             bool           `json:"ntpSync,omitempty"`
	KernelPanics        bool           `json:"kernelPanics,omitempty"`
	OOMKiller           bool           `json:"oomKiller,omitempty"`
	CPUFrequency        bool           `json:"cpuFrequency,omitempty"`
	InterruptsBalance   bool           `json:"interruptsBalance,omitempty"`
	CPUStealTime        bool           `json:"cpuStealTime,omitempty"`
	MemoryFragmentation bool           `json:"memoryFragmentation,omitempty"`
	SwapActivity        bool           `json:"swapActivity,omitempty"`
	ContextSwitches     bool           `json:"contextSwitches,omitempty"`
	SELinuxStatus       bool           `json:"selinuxStatus,omitempty"`
	SSHAccess           bool           `json:"sshAccess,omitempty"`
	KernelModules       bool           `json:"kernelModules,omitempty"`
	Hardware            HardwareChecks `json:"hardware,omitempty"`
	Disks               DiskChecks     `json:"disks,omitempty"`
	Network             NetworkChecks  `json:"network,omitempty"`
}

// DiskChecks defines disk-related checks
type DiskChecks struct {
	Space           bool `json:"space,omitempty"`
	SMART           bool `json:"smart,omitempty"`
	Performance     bool `json:"performance,omitempty"`
	RAID            bool `json:"raid,omitempty"`
	PVs             bool `json:"pvs,omitempty"`  // Physical Volumes (LVM - pvs command)
	LVM             bool `json:"lvm,omitempty"`  // Logical Volumes and Volume Groups (LVM - lvs/vgs commands)
	IOWait          bool `json:"ioWait,omitempty"`
	QueueDepth      bool `json:"queueDepth,omitempty"`
	FilesystemErrors bool `json:"filesystemErrors,omitempty"`
	InodeUsage      bool `json:"inodeUsage,omitempty"`
	MountPoints     bool `json:"mountPoints,omitempty"`
}

// HardwareChecks defines hardware-related checks
type HardwareChecks struct {
	Temperature  bool `json:"temperature,omitempty"`
	IPMI         bool `json:"ipmi,omitempty"`
	BMC          bool `json:"bmc,omitempty"`
	FanStatus    bool `json:"fanStatus,omitempty"`
	PowerSupply  bool `json:"powerSupply,omitempty"`
	MemoryErrors bool `json:"memoryErrors,omitempty"`
	PCIeErrors   bool `json:"pcieErrors,omitempty"`
	CPUMicrocode bool `json:"cpuMicrocode,omitempty"`
}

// NetworkChecks defines network-related checks
type NetworkChecks struct {
	Interfaces      bool `json:"interfaces,omitempty"`
	Routing         bool `json:"routing,omitempty"`
	Connectivity    bool `json:"connectivity,omitempty"`
	Statistics      bool `json:"statistics,omitempty"`
	Errors          bool `json:"errors,omitempty"`
	Latency         bool `json:"latency,omitempty"`
	DNSResolution   bool `json:"dnsResolution,omitempty"`
	BondingStatus   bool `json:"bondingStatus,omitempty"`
	FirewallRules   bool `json:"firewallRules,omitempty"`
}

// KubernetesChecks defines Kubernetes-level checks
type KubernetesChecks struct {
	NodeStatus        bool `json:"nodeStatus,omitempty"`
	Pods              bool `json:"pods,omitempty"`
	ClusterOperators  bool `json:"clusterOperators,omitempty"`
	NodeResources     bool `json:"nodeResources,omitempty"`
	NodeResourceUsage bool `json:"nodeResourceUsage,omitempty"`
	ContainerRuntime  bool `json:"containerRuntime,omitempty"`
	KubeletHealth     bool `json:"kubeletHealth,omitempty"`
	CNIPlugin         bool `json:"cniPlugin,omitempty"`
	NodeConditions    bool `json:"nodeConditions,omitempty"`
}

// SystemCheckResults contains the results of system-level checks
type SystemCheckResults struct {
	Uptime              *CheckResult           `json:"uptime,omitempty"`
	Processes           *CheckResult           `json:"processes,omitempty"`
	Resources           *CheckResult           `json:"resources,omitempty"`
	Services            *CheckResult           `json:"services,omitempty"`
	Memory              *CheckResult           `json:"memory,omitempty"`
	UninterruptibleTasks *CheckResult          `json:"uninterruptibleTasks,omitempty"`
	SystemLogs          *CheckResult           `json:"systemLogs,omitempty"`
	FileDescriptors     *CheckResult           `json:"fileDescriptors,omitempty"`
	ZombieProcesses     *CheckResult           `json:"zombieProcesses,omitempty"`
	NTPSync             *CheckResult           `json:"ntpSync,omitempty"`
	KernelPanics        *CheckResult           `json:"kernelPanics,omitempty"`
	OOMKiller           *CheckResult           `json:"oomKiller,omitempty"`
	CPUFrequency        *CheckResult           `json:"cpuFrequency,omitempty"`
	InterruptsBalance   *CheckResult           `json:"interruptsBalance,omitempty"`
	CPUStealTime        *CheckResult           `json:"cpuStealTime,omitempty"`
	MemoryFragmentation *CheckResult           `json:"memoryFragmentation,omitempty"`
	SwapActivity        *CheckResult           `json:"swapActivity,omitempty"`
	ContextSwitches     *CheckResult           `json:"contextSwitches,omitempty"`
	SELinuxStatus       *CheckResult           `json:"selinuxStatus,omitempty"`
	SSHAccess           *CheckResult           `json:"sshAccess,omitempty"`
	KernelModules       *CheckResult           `json:"kernelModules,omitempty"`
	Hardware            *HardwareCheckResults  `json:"hardware,omitempty"`
	Disks               *DiskCheckResults      `json:"disks,omitempty"`
	Network             *NetworkCheckResults   `json:"network,omitempty"`
}

// HardwareCheckResults contains hardware check results
type HardwareCheckResults struct {
	Temperature  *CheckResult `json:"temperature,omitempty"`
	IPMI         *CheckResult `json:"ipmi,omitempty"`
	BMC          *CheckResult `json:"bmc,omitempty"`
	FanStatus    *CheckResult `json:"fanStatus,omitempty"`
	PowerSupply  *CheckResult `json:"powerSupply,omitempty"`
	MemoryErrors *CheckResult `json:"memoryErrors,omitempty"`
	PCIeErrors   *CheckResult `json:"pcieErrors,omitempty"`
	CPUMicrocode *CheckResult `json:"cpuMicrocode,omitempty"`
}

// DiskCheckResults contains disk check results
type DiskCheckResults struct {
	Space            *CheckResult `json:"space,omitempty"`
	SMART            *CheckResult `json:"smart,omitempty"`
	Performance      *CheckResult `json:"performance,omitempty"`
	RAID             *CheckResult `json:"raid,omitempty"`
	PVs              *CheckResult `json:"pvs,omitempty"`  // Physical Volumes (LVM - pvs command)
	LVM              *CheckResult `json:"lvm,omitempty"`  // Logical Volumes and Volume Groups (LVM - lvs/vgs commands)
	IOWait           *CheckResult `json:"ioWait,omitempty"`
	QueueDepth       *CheckResult `json:"queueDepth,omitempty"`
	FilesystemErrors *CheckResult `json:"filesystemErrors,omitempty"`
	InodeUsage       *CheckResult `json:"inodeUsage,omitempty"`
	MountPoints      *CheckResult `json:"mountPoints,omitempty"`
}

// NetworkCheckResults contains network check results
type NetworkCheckResults struct {
	Interfaces    *CheckResult `json:"interfaces,omitempty"`
	Routing       *CheckResult `json:"routing,omitempty"`
	Connectivity  *CheckResult `json:"connectivity,omitempty"`
	Statistics    *CheckResult `json:"statistics,omitempty"`
	Errors        *CheckResult `json:"errors,omitempty"`
	Latency       *CheckResult `json:"latency,omitempty"`
	DNSResolution *CheckResult `json:"dnsResolution,omitempty"`
	BondingStatus *CheckResult `json:"bondingStatus,omitempty"`
	FirewallRules *CheckResult `json:"firewallRules,omitempty"`
}

// KubernetesCheckResults contains the results of Kubernetes-level checks
type KubernetesCheckResults struct {
	NodeStatus         *CheckResult `json:"nodeStatus,omitempty"`
	Pods               *CheckResult `json:"pods,omitempty"`
	ClusterOperators   *CheckResult `json:"clusterOperators,omitempty"`
	NodeResources      *CheckResult `json:"nodeResources,omitempty"`
	NodeResourceUsage  *CheckResult `json:"nodeResourceUsage,omitempty"`
	ContainerRuntime   *CheckResult `json:"containerRuntime,omitempty"`
	KubeletHealth      *CheckResult `json:"kubeletHealth,omitempty"`
	CNIPlugin          *CheckResult `json:"cniPlugin,omitempty"`
	NodeConditions     *CheckResult `json:"nodeConditions,omitempty"`
}

// CheckResults contains all check results
type CheckResults struct {
	SystemResults     SystemCheckResults     `json:"systemResults,omitempty"`
	KubernetesResults KubernetesCheckResults `json:"kubernetesResults,omitempty"`
}

// NodeCheckStatus defines the observed state of NodeCheck
type NodeCheckStatus struct {
	// NodeName is the name of the node that was checked (mirrored from spec for convenience)
	NodeName string `json:"nodeName,omitempty"`

	// OverallStatus indicates the overall status of all checks
	OverallStatus string `json:"overallStatus,omitempty"`

	// Message provides a summary of the check results
	Message string `json:"message,omitempty"`

	// LastCheckTime is the timestamp of the last check
	LastCheckTime metav1.Time `json:"lastCheckTime,omitempty"`

	// CheckResults contains all check results
	CheckResults CheckResults `json:"checkResults,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Node",type="string",JSONPath=".spec.nodeName"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.overallStatus"
// +kubebuilder:printcolumn:name="Last Check",type="date",JSONPath=".status.lastCheckTime"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.message"

// NodeCheck is the Schema for the nodechecks API
type NodeCheck struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeCheckSpec   `json:"spec,omitempty"`
	Status NodeCheckStatus `json:"status,omitempty"`
}

// DeepCopyObject returns a generically typed copy of an object
func (in *NodeCheck) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopy returns a deep copy of the NodeCheck
func (in *NodeCheck) DeepCopy() *NodeCheck {
	if in == nil {
		return nil
	}
	out := new(NodeCheck)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies all properties of this object into another object of the same type
func (in *NodeCheck) DeepCopyInto(out *NodeCheck) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// +kubebuilder:object:root=true

// NodeCheckList contains a list of NodeCheck
type NodeCheckList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeCheck `json:"items"`
}

// DeepCopyObject returns a generically typed copy of an object
func (in *NodeCheckList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopy returns a deep copy of the NodeCheckList
func (in *NodeCheckList) DeepCopy() *NodeCheckList {
	if in == nil {
		return nil
	}
	out := new(NodeCheckList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies all properties of this object into another object of the same type
func (in *NodeCheckList) DeepCopyInto(out *NodeCheckList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]NodeCheck, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopyInto copies all properties of this object into another object of the same type
func (in *NodeCheckSpec) DeepCopyInto(out *NodeCheckSpec) {
	*out = *in
	in.SystemChecks.DeepCopyInto(&out.SystemChecks)
	in.KubernetesChecks.DeepCopyInto(&out.KubernetesChecks)
}

// DeepCopy returns a deep copy of the NodeCheckSpec
func (in *NodeCheckSpec) DeepCopy() *NodeCheckSpec {
	if in == nil {
		return nil
	}
	out := new(NodeCheckSpec)
	in.DeepCopyInto(out)
	return out
}



// DeepCopyInto copies all properties of this object into another object of the same type
func (in *SystemChecks) DeepCopyInto(out *SystemChecks) {
	*out = *in
	in.Disks.DeepCopyInto(&out.Disks)
	in.Network.DeepCopyInto(&out.Network)
}

// DeepCopy returns a deep copy of the SystemChecks
func (in *SystemChecks) DeepCopy() *SystemChecks {
	if in == nil {
		return nil
	}
	out := new(SystemChecks)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies all properties of this object into another object of the same type
func (in *DiskChecks) DeepCopyInto(out *DiskChecks) {
	*out = *in
}

// DeepCopy returns a deep copy of the DiskChecks
func (in *DiskChecks) DeepCopy() *DiskChecks {
	if in == nil {
		return nil
	}
	out := new(DiskChecks)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies all properties of this object into another object of the same type
func (in *NetworkChecks) DeepCopyInto(out *NetworkChecks) {
	*out = *in
}

// DeepCopy returns a deep copy of the NetworkChecks
func (in *NetworkChecks) DeepCopy() *NetworkChecks {
	if in == nil {
		return nil
	}
	out := new(NetworkChecks)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies all properties of this object into another object of the same type
func (in *KubernetesChecks) DeepCopyInto(out *KubernetesChecks) {
	*out = *in
}

// DeepCopy returns a deep copy of the KubernetesChecks
func (in *KubernetesChecks) DeepCopy() *KubernetesChecks {
	if in == nil {
		return nil
	}
	out := new(KubernetesChecks)
	in.DeepCopyInto(out)
	return out
}
