package checks

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/albertofilice/node-check-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// DiskChecker handles disk monitoring
type DiskChecker struct {
	nodeName string
}

// NewDiskChecker creates a new disk checker
func NewDiskChecker(nodeName string) *DiskChecker {
	return &DiskChecker{
		nodeName: nodeName,
	}
}

// CheckDiskSpace performs disk space monitoring
func (dc *DiskChecker) CheckDiskSpace(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	output, err := runHostCommand(ctx, "df -hPT")
	if err != nil {
		cmd := exec.CommandContext(ctx, "df", "-h")
		output, err = cmd.Output()
		if err != nil {
			result.Status = "Critical"
			result.Message = fmt.Sprintf("Failed to execute df: %v", err)
			result.Details = mapToRawExtension(details)
			return result
		}
		details["check_source"] = "container"
	} else {
		details["check_source"] = "host"
	}

	dfOutput := strings.TrimSpace(string(output))
	details["df_output"] = dfOutput

	// Parse disk usage
	lines := strings.Split(dfOutput, "\n")
	diskUsage := make(map[string]map[string]string)
	criticalDisks := []string{}
	warningDisks := []string{}

	skipFSTypes := map[string]bool{
		"tmpfs":      true,
		"devtmpfs":   true,
		"squashfs":   true,
		"overlay":    true,
		"proc":       true,
		"sysfs":      true,
		"cgroup":     true,
		"cgroup2":    true,
		"debugfs":    true,
		"tracefs":    true,
		"securityfs": true,
		"pstore":     true,
		"hugetlbfs":  true,
		"configfs":   true,
		"fusectl":    true,
		"ramfs":      true,
		"composefs":  true,
	}

	for i, line := range lines {
		if i == 0 {
			continue // Skip header
		}
		
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse df output: filesystem type size used available use% mounted_on
		// Use a more robust parsing that handles variable whitespace
		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}

		filesystem := fields[0]
		fsType := fields[1]
		size := fields[2]
		used := fields[3]
		available := fields[4]
		usePercent := fields[5]
		mountedOn := strings.Join(fields[6:], " ")

		// Skip known pseudo or read-only filesystems
		if skipFSTypes[fsType] {
			continue
		}
		if strings.Contains(mountedOn, "/tmp") || strings.HasPrefix(mountedOn, "/run/") {
			continue
		}

		// Skip tiny read-only filesystems that always show 100%
		if fsType == "composefs" || (available == "0" && usePercent == "100%") {
			continue
		}

		diskInfo := map[string]string{
			"size":        size,
			"used":        used,
			"available":   available,
			"use_percent": usePercent,
			"mounted_on":  mountedOn,
			"fs_type":     fsType,
		}
		diskUsage[filesystem] = diskInfo

		// Parse usage percentage
		usePercentStr := strings.Trim(usePercent, "%")
		if usePercentInt, err := strconv.Atoi(usePercentStr); err == nil {
			if usePercentInt >= 95 {
				criticalDisks = append(criticalDisks, fmt.Sprintf("%s: %d%%", mountedOn, usePercentInt))
			} else if usePercentInt >= 85 {
				warningDisks = append(warningDisks, fmt.Sprintf("%s: %d%%", mountedOn, usePercentInt))
			}
		}
	}

	// Log debug info to verify parsing
	log := ctrl.Log.WithName("DiskChecker")
	log.V(1).Info("Parsed disk usage", "diskUsageCount", len(diskUsage), "criticalDisks", len(criticalDisks), "warningDisks", len(warningDisks))
	
	// Convert map[string]map[string]string to []interface{} for better serialization
	// This avoids potential issues with nested maps in JSON serialization
	diskUsageList := make([]map[string]interface{}, 0, len(diskUsage))
	for filesystem, diskInfo := range diskUsage {
		entry := map[string]interface{}{
			"filesystem": filesystem,
			"fs_type":     diskInfo["fs_type"],
			"size":        diskInfo["size"],
			"used":        diskInfo["used"],
			"available":   diskInfo["available"],
			"use_percent": diskInfo["use_percent"],
			"mounted_on":  diskInfo["mounted_on"],
		}
		diskUsageList = append(diskUsageList, entry)
	}
	
	// Store both formats for compatibility
	details["disk_usage"] = diskUsageList  // List format (more reliable for JSON)
	details["disk_usage_map"] = diskUsage  // Original map format
	details["critical_disks"] = criticalDisks
	details["warning_disks"] = warningDisks

	if len(criticalDisks) > 0 {
		result.Status = "Critical"
		result.Message = fmt.Sprintf("Critical disk usage: %s", strings.Join(criticalDisks, ", "))
	} else if len(warningDisks) > 0 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("Warning disk usage: %s", strings.Join(warningDisks, ", "))
	} else {
		result.Status = "Healthy"
		result.Message = "Disk usage is normal"
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckSMART performs SMART disk health monitoring
func (dc *DiskChecker) CheckSMART(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	output, err := runHostCommand(ctx, "lsblk -d -n -o NAME")
	if err != nil {
		cmd := exec.CommandContext(ctx, "lsblk", "-d", "-n", "-o", "NAME")
		output, err = cmd.Output()
		if err != nil {
			result.Status = "Critical"
			result.Message = fmt.Sprintf("Failed to list disk devices: %v", err)
			result.Details = mapToRawExtension(details)
			return result
		}
	}

	devices := strings.Split(strings.TrimSpace(string(output)), "\n")
	smartResults := make(map[string]interface{})
	criticalDisks := []string{}
	warningDisks := []string{}

	for _, device := range devices {
		if strings.HasPrefix(device, "sd") || strings.HasPrefix(device, "nvme") {
			devicePath := "/dev/" + device
			
			// Execute smartctl for each device
			smartOutput, err := runHostCommand(ctx, fmt.Sprintf("smartctl -a %s", devicePath))
			if err != nil {
				smartCmd := exec.CommandContext(ctx, "smartctl", "-a", devicePath)
				sOutput, sErr := smartCmd.Output()
				if sErr != nil {
					// Device might not support SMART or not accessible
					smartResults[device] = map[string]string{
						"error":  sErr.Error(),
						"status": "not_accessible",
					}
					continue
				}
				smartOutput = sOutput
			}

			smartStr := strings.TrimSpace(string(smartOutput))
			smartResults[device] = smartStr

			// Parse SMART attributes for critical values
			lines := strings.Split(smartStr, "\n")
			
			for _, line := range lines {
				// Check for overall health status
				if strings.Contains(line, "SMART overall-health self-assessment test result:") {
					if !strings.Contains(line, "PASSED") {
						criticalDisks = append(criticalDisks, fmt.Sprintf("%s: SMART test failed", device))
					}
				}
				
				// Check for specific critical attributes
				if strings.Contains(line, "Reallocated_Sector_Ct") {
					fields := strings.Fields(line)
					if len(fields) >= 10 {
						if raw, err := strconv.Atoi(fields[9]); err == nil && raw > 0 {
							warningDisks = append(warningDisks, fmt.Sprintf("%s: %d reallocated sectors", device, raw))
						}
					}
				}
				
				if strings.Contains(line, "Current_Pending_Sector") {
					fields := strings.Fields(line)
					if len(fields) >= 10 {
						if raw, err := strconv.Atoi(fields[9]); err == nil && raw > 0 {
							criticalDisks = append(criticalDisks, fmt.Sprintf("%s: %d pending sectors", device, raw))
						}
					}
				}
			}
		}
	}

	details["smart_results"] = smartResults
	details["critical_disks"] = criticalDisks
	details["warning_disks"] = warningDisks

	if len(criticalDisks) > 0 {
		result.Status = "Critical"
		result.Message = fmt.Sprintf("Critical SMART issues: %s", strings.Join(criticalDisks, ", "))
	} else if len(warningDisks) > 0 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("Warning SMART issues: %s", strings.Join(warningDisks, ", "))
	} else {
		result.Status = "Healthy"
		result.Message = "All disks are healthy"
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckDiskPerformance performs disk performance monitoring using iostat
// It samples disk I/O statistics over 3 seconds and analyzes utilization, latency, and service time
func (dc *DiskChecker) CheckDiskPerformance(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	// iostat -x 1 3: extended stats, 1 second interval, 3 samples
	// This gives us average statistics over 3 seconds
	output, err := runHostCommand(ctx, "iostat -x 1 3")
	if err != nil {
		cmd := exec.CommandContext(ctx, "iostat", "-x", "1", "3")
		output, err = cmd.Output()
		if err != nil {
			result.Status = "Warning"
			result.Message = fmt.Sprintf("Failed to execute iostat: %v (iostat may not be installed)", err)
			details["error"] = err.Error()
			details["note"] = "iostat is part of sysstat package. Install with: yum install sysstat or apt-get install sysstat"
			result.Details = mapToRawExtension(details)
			return result
		}
		details["check_source"] = "container"
	} else {
		details["check_source"] = "host"
	}

	iostatOutput := strings.TrimSpace(string(output))
	details["iostat_output"] = iostatOutput

	// Parse iostat output for performance metrics
	lines := strings.Split(iostatOutput, "\n")
	deviceStats := make(map[string]map[string]float64)
	
	// Track issues for status determination
	highUtilization := []string{}
	highLatency := []string{}
	highServiceTime := []string{}

	// Find the LAST set of statistics (iostat outputs multiple samples)
	// We want the most recent/average statistics
	lastHeaderIndex := -1
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.Contains(lines[i], "Device") && strings.Contains(lines[i], "r/s") {
			lastHeaderIndex = i
			break
		}
	}

	if lastHeaderIndex == -1 {
		result.Status = "Warning"
		result.Message = "Unable to parse iostat output (header not found)"
		details["error"] = "iostat output format not recognized"
		result.Details = mapToRawExtension(details)
		return result
	}

	// Parse device statistics starting from the last header
	startIndex := lastHeaderIndex + 1
	for i := startIndex; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		
		fields := strings.Fields(line)
		// iostat -x output format (modern): 
		// Device r/s rkB/s rrqm/s %rrqm r_await rareq-sz w/s wkB/s wrqm/s %wrqm w_await wareq-sz d/s dkB/s drqm/s %drqm d_await dareq-sz f/s f_await aqu-sz %util
		// Minimum 23 fields required for full extended stats
		if len(fields) < 14 {
			continue
		}

		device := fields[0]
		if device == "Device" || strings.HasPrefix(device, "avg-cpu") || strings.HasPrefix(device, "Linux") {
			continue // Skip header lines and CPU stats
		}

		// Skip loop devices and other virtual devices for performance checks
		if strings.HasPrefix(device, "loop") || strings.HasPrefix(device, "dm-") {
			continue
		}

		stats := make(map[string]float64)
		
		// Parse metrics based on modern iostat -x format
		// Field positions: 0=Device, 1=r/s, 2=rkB/s, 3=rrqm/s, 4=%rrqm, 5=r_await, 6=rareq-sz,
		//                 7=w/s, 8=wkB/s, 9=wrqm/s, 10=%wrqm, 11=w_await, 12=wareq-sz,
		//                 13=d/s, 14=dkB/s, 15=drqm/s, 16=%drqm, 17=d_await, 18=dareq-sz,
		//                 19=f/s, 20=f_await, 21=aqu-sz, 22=%util
		
		if rps, err := strconv.ParseFloat(fields[1], 64); err == nil {
			stats["reads_per_sec"] = rps
		}
		if rkbs, err := strconv.ParseFloat(fields[2], 64); err == nil {
			stats["read_kb_per_sec"] = rkbs
		}
		if rrqm, err := strconv.ParseFloat(fields[3], 64); err == nil {
			stats["read_requests_merged"] = rrqm
		}
		// fields[4] is %rrqm - skip for now
		if r_await, err := strconv.ParseFloat(fields[5], 64); err == nil {
			stats["read_await_ms"] = r_await
		}
		if ravgq, err := strconv.ParseFloat(fields[6], 64); err == nil {
			stats["read_avg_queue_size"] = ravgq
		}
		if len(fields) > 7 {
			if wps, err := strconv.ParseFloat(fields[7], 64); err == nil {
				stats["writes_per_sec"] = wps
			}
		}
		if len(fields) > 8 {
			if wkbs, err := strconv.ParseFloat(fields[8], 64); err == nil {
				stats["write_kb_per_sec"] = wkbs
			}
		}
		if len(fields) > 9 {
			if wrqm, err := strconv.ParseFloat(fields[9], 64); err == nil {
				stats["write_requests_merged"] = wrqm
			}
		}
		// fields[10] is %wrqm - skip for now
		if len(fields) > 11 {
			if w_await, err := strconv.ParseFloat(fields[11], 64); err == nil {
				stats["write_await_ms"] = w_await
			}
		}
		if len(fields) > 12 {
			if wavgq, err := strconv.ParseFloat(fields[12], 64); err == nil {
				stats["write_avg_queue_size"] = wavgq
			}
		}
		// fields[13-18] are discard stats - skip for now
		// fields[19-20] are flush stats - skip for now
		if len(fields) > 21 {
			if aqu_sz, err := strconv.ParseFloat(fields[21], 64); err == nil {
				stats["avg_queue_size"] = aqu_sz
			}
		}
		if len(fields) > 22 {
			if util, err := strconv.ParseFloat(fields[22], 64); err == nil {
				stats["utilization_percent"] = util
			}
		}
		
		// Note: svctm (service time) was deprecated in newer iostat versions
		// We can approximate it from r_await and w_await, but it's not as accurate
		// For now, we'll skip service_time_ms if not available

		deviceStats[device] = stats

		// Get utilization for context-aware latency checks
		util, hasUtil := stats["utilization_percent"]

		// Check for performance issues
		// Utilization > 90% indicates disk saturation
		if hasUtil && util > 90 {
			highUtilization = append(highUtilization, fmt.Sprintf("%s: %.1f%%", device, util))
		}

		// Latency checks: consider utilization and device type
		// For NVMe/SSD: lower thresholds, but allow higher latency if utilization is high
		// For HDD: higher thresholds are acceptable
		// Note: Occasional latency spikes are normal, especially for writes
		// We only flag sustained high latency, especially when combined with high utilization
		isNVMe := strings.HasPrefix(device, "nvme") || strings.Contains(device, "nvme")
		
		// Determine latency thresholds based on device type and utilization
		readLatencyThreshold := 100.0  // Default for HDD (more lenient)
		writeLatencyThreshold := 200.0 // Default for HDD (writes can queue more)
		
		if isNVMe {
			// NVMe should have lower latency, but occasional spikes are normal
			// Be more lenient to avoid false positives from temporary I/O bursts
			if hasUtil && util > 80 {
				// Under heavy load, allow higher latency as it's expected
				readLatencyThreshold = 150.0  // Allow higher latency under heavy load
				writeLatencyThreshold = 300.0 // Writes can queue significantly under load
			} else if hasUtil && util > 50 {
				// Moderate load - allow some latency spikes
				readLatencyThreshold = 100.0  // More lenient for occasional spikes
				writeLatencyThreshold = 200.0 // Writes can have occasional spikes
			} else {
				// Low utilization - be lenient as spikes are likely temporary
				// Only flag very high latency (>100ms read, >200ms write) even under low load
				readLatencyThreshold = 100.0  // More lenient to avoid false positives
				writeLatencyThreshold = 250.0 // Writes can spike even under low load
			}
		} else if hasUtil && util > 80 {
			// For HDD under heavy load, allow higher latency
			readLatencyThreshold = 150.0
			writeLatencyThreshold = 300.0
		} else if hasUtil && util > 50 {
			// HDD under moderate load
			readLatencyThreshold = 100.0
			writeLatencyThreshold = 200.0
		}

		// Check read latency
		if r_await, ok := stats["read_await_ms"]; ok && r_await > readLatencyThreshold {
			context := ""
			if hasUtil {
				context = fmt.Sprintf(" (util: %.1f%%)", util)
			}
			highLatency = append(highLatency, fmt.Sprintf("%s read: %.1fms%s", device, r_await, context))
		}
		
		// Check write latency
		if w_await, ok := stats["write_await_ms"]; ok && w_await > writeLatencyThreshold {
			context := ""
			if hasUtil {
				context = fmt.Sprintf(" (util: %.1f%%)", util)
			}
			highLatency = append(highLatency, fmt.Sprintf("%s write: %.1fms%s", device, w_await, context))
		}

		// Service time checks: svctm is deprecated in modern iostat versions
		// We can approximate it from r_await and w_await, but it's less accurate
		// For now, we'll use a weighted average of r_await and w_await as an approximation
		r_await, hasRAwait := stats["read_await_ms"]
		w_await, hasWAwait := stats["write_await_ms"]
		rps, hasRPS := stats["reads_per_sec"]
		wps, hasWPS := stats["writes_per_sec"]
		
		if hasRAwait && hasWAwait && hasRPS && hasWPS {
			// Approximate service time as weighted average of r_await and w_await
			// This is not perfect but gives a reasonable estimate
			totalIO := rps + wps
			if totalIO > 0 {
				approxServiceTime := (r_await*rps + w_await*wps) / totalIO
				stats["service_time_ms"] = approxServiceTime
				
				serviceTimeThreshold := 50.0 // Default for HDD (more lenient)
				if isNVMe {
					// NVMe should have lower service time, but be lenient for occasional spikes
					if hasUtil && util > 80 {
						serviceTimeThreshold = 100.0 // Allow higher service time under heavy load
					} else if hasUtil && util > 50 {
						serviceTimeThreshold = 50.0  // Moderate load - allow some spikes
					} else {
						serviceTimeThreshold = 30.0  // Low load - be lenient to avoid false positives
					}
				} else if hasUtil && util > 80 {
					serviceTimeThreshold = 100.0 // HDD under heavy load
				} else if hasUtil && util > 50 {
					serviceTimeThreshold = 50.0  // HDD under moderate load
				}
				
				if approxServiceTime > serviceTimeThreshold {
					context := ""
					if hasUtil {
						context = fmt.Sprintf(" (util: %.1f%%, approx)", util)
					} else {
						context = " (approx)"
					}
					highServiceTime = append(highServiceTime, fmt.Sprintf("%s: %.1fms%s", device, approxServiceTime, context))
				}
			}
		}
	}

	details["device_stats"] = deviceStats
	details["check_method"] = "iostat -x (3 second average)"
	details["note"] = "Performance metrics are averaged over 3 seconds. Thresholds are adjusted based on device type (NVMe vs HDD) and utilization to avoid false positives from occasional I/O spikes. NVMe devices: Low load (<50% util): <100ms read, <250ms write, <30ms service time. Moderate load (50-80% util): <100ms read, <200ms write, <50ms service time. Heavy load (>80% util): <150ms read, <300ms write, <100ms service time. HDD devices have more lenient thresholds. Note: Service time is approximated from r_await and w_await as svctm is deprecated in modern iostat versions."
	details["high_utilization"] = highUtilization
	details["high_latency"] = highLatency
	details["high_service_time"] = highServiceTime

	// Determine status based on multiple factors
	issues := []string{}
	if len(highUtilization) > 0 {
		issues = append(issues, fmt.Sprintf("High utilization: %s", strings.Join(highUtilization, ", ")))
	}
	if len(highLatency) > 0 {
		issues = append(issues, fmt.Sprintf("High latency: %s", strings.Join(highLatency, ", ")))
	}
	if len(highServiceTime) > 0 {
		issues = append(issues, fmt.Sprintf("High service time: %s", strings.Join(highServiceTime, ", ")))
	}

	if len(issues) > 0 {
		result.Status = "Warning"
		result.Message = strings.Join(issues, "; ")
	} else {
		result.Status = "Healthy"
		result.Message = "Disk performance is normal (utilization, latency, and service time within acceptable ranges)"
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckRAID performs RAID status monitoring
func (dc *DiskChecker) CheckRAID(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	output, err := runHostCommand(ctx, "cat /proc/mdstat")
	if err != nil {
		cmd := exec.CommandContext(ctx, "cat", "/proc/mdstat")
		output, err = cmd.Output()
		if err != nil {
			result.Status = "Warning"
			result.Message = "No software RAID detected"
			details["error"] = err.Error()
			result.Details = mapToRawExtension(details)
			return result
		}
	}

	mdstatOutput := strings.TrimSpace(string(output))
	details["mdstat_output"] = mdstatOutput

	// Parse RAID status
	lines := strings.Split(mdstatOutput, "\n")
	raidArrays := make(map[string]string)
	criticalArrays := []string{}
	warningArrays := []string{}

	for _, line := range lines {
		if strings.Contains(line, "md") {
			// Parse RAID array status
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				arrayName := fields[0]
				status := strings.Join(fields[1:], " ")
				raidArrays[arrayName] = status

				// Check for degraded or failed arrays
				if strings.Contains(status, "degraded") {
					warningArrays = append(warningArrays, fmt.Sprintf("%s: degraded", arrayName))
				}
				if strings.Contains(status, "failed") {
					criticalArrays = append(criticalArrays, fmt.Sprintf("%s: failed", arrayName))
				}
			}
		}
	}

	details["raid_arrays"] = raidArrays
	details["critical_arrays"] = criticalArrays
	details["warning_arrays"] = warningArrays

	// Also check for hardware RAID using megacli if available
	megacliCmd := exec.CommandContext(ctx, "megacli", "-LDInfo", "-Lall", "-aALL")
	megacliOutput, err := megacliCmd.Output()
	if err == nil {
		megacliStr := strings.TrimSpace(string(megacliOutput))
		details["megacli_output"] = megacliStr
		
		// Parse megacli output for hardware RAID status
		lines := strings.Split(megacliStr, "\n")
		for _, line := range lines {
			if strings.Contains(line, "State") {
				if strings.Contains(line, "Degraded") {
					warningArrays = append(warningArrays, "Hardware RAID: degraded")
				} else if strings.Contains(line, "Failed") {
					criticalArrays = append(criticalArrays, "Hardware RAID: failed")
				}
			}
		}
	}

	if len(criticalArrays) > 0 {
		result.Status = "Critical"
		result.Message = fmt.Sprintf("Critical RAID issues: %s", strings.Join(criticalArrays, ", "))
	} else if len(warningArrays) > 0 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("Warning RAID issues: %s", strings.Join(warningArrays, ", "))
	} else {
		result.Status = "Healthy"
		result.Message = "RAID status is normal"
	}

	result.Details = mapToRawExtension(details)
	return result
}

// parseSizeToGB parses a size string (e.g., "10.5G", "1024M", "1T") and returns the size in GB
func parseSizeToGB(sizeStr string) float64 {
	sizeStr = strings.TrimSpace(sizeStr)
	if sizeStr == "" {
		return 0
	}

	// Remove unit and convert to float
	var size float64
	var unit string
	
	// Try to parse the number and unit
	for i := len(sizeStr) - 1; i >= 0; i-- {
		if (sizeStr[i] >= '0' && sizeStr[i] <= '9') || sizeStr[i] == '.' {
			size, _ = strconv.ParseFloat(sizeStr[:i+1], 64)
			if i+1 < len(sizeStr) {
				unit = strings.ToUpper(sizeStr[i+1:])
			}
			break
		}
	}

	if size == 0 {
		return 0
	}

	// Convert to GB based on unit
	switch unit {
	case "T", "TB", "TiB":
		return size * 1024
	case "G", "GB", "GiB":
		return size
	case "M", "MB", "MiB":
		return size / 1024
	case "K", "KB", "KiB":
		return size / (1024 * 1024)
	default:
		// Assume GB if no unit
		return size
	}
}

// CheckPVs performs Physical Volume (LVM) monitoring using the pvs command
func (dc *DiskChecker) CheckPVs(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	// Check Physical Volumes using pvs command
	pvsCmd := exec.CommandContext(ctx, "pvs", "--noheadings", "--units", "g", "--separator", "|", "-o", "pv_name,vg_name,pv_size,pv_free,pv_attr")
	pvsOutput, err := pvsCmd.Output()
	if err != nil {
		result.Status = "Warning"
		result.Message = "LVM Physical Volumes not available or no physical volumes found"
		details["error"] = err.Error()
		result.Details = mapToRawExtension(details)
		return result
	}

	pvsStr := strings.TrimSpace(string(pvsOutput))
	details["pvs_output"] = pvsStr

	pvDetails := make([]map[string]interface{}, 0)
	criticalPVs := []string{}
	warningPVs := []string{}
	lines := strings.Split(pvsStr, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Split(line, "|")
		if len(fields) >= 5 {
			pvName := strings.TrimSpace(fields[0])
			vgName := strings.TrimSpace(fields[1])
			pvSize := strings.TrimSpace(fields[2])
			pvFree := strings.TrimSpace(fields[3])
			pvAttr := strings.TrimSpace(fields[4])

			pvInfo := map[string]interface{}{
				"name": pvName,
				"vg":   vgName,
				"size": pvSize,
				"free": pvFree,
				"attr": pvAttr,
			}

			// Parse size and free to check disk usage
			pvSizeGB := parseSizeToGB(pvSize)
			pvFreeGB := parseSizeToGB(pvFree)
			if pvSizeGB > 0 {
				pvUsed := pvSizeGB - pvFreeGB
				pvUsedPercent := (pvUsed / pvSizeGB) * 100
				pvFreePercent := (pvFreeGB / pvSizeGB) * 100

				pvInfo["size_gb"] = pvSizeGB
				pvInfo["free_gb"] = pvFreeGB
				pvInfo["used_gb"] = pvUsed
				pvInfo["used_percent"] = fmt.Sprintf("%.1f", pvUsedPercent)
				pvInfo["free_percent"] = fmt.Sprintf("%.1f", pvFreePercent)

				// Check for low space (only if PV is in a VG)
				// Warning threshold set to 5% to reduce false positives
				// In OpenShift clusters, 5-10% free space is often acceptable
				if vgName != "" {
					if pvFreePercent < 5 {
						criticalPVs = append(criticalPVs, fmt.Sprintf("PV %s: only %.1f%% free space", pvName, pvFreePercent))
					}
				}
			}

			// Check PV attributes (5th character indicates state)
			if len(pvAttr) >= 5 {
				state := pvAttr[4]
				if state == 'm' {
					criticalPVs = append(criticalPVs, fmt.Sprintf("PV %s: missing", pvName))
				}
			}

			pvDetails = append(pvDetails, pvInfo)
		}
	}

	details["physical_volumes"] = pvDetails
	details["critical_pvs"] = criticalPVs
	details["warning_pvs"] = warningPVs

	if len(criticalPVs) > 0 {
		result.Status = "Critical"
		result.Message = fmt.Sprintf("Critical Physical Volume issues: %s", strings.Join(criticalPVs, ", "))
	} else if len(warningPVs) > 0 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("Warning Physical Volume issues: %s", strings.Join(warningPVs, ", "))
	} else {
		result.Status = "Healthy"
		if len(pvDetails) > 0 {
			result.Message = fmt.Sprintf("Physical Volumes are healthy (%d total)", len(pvDetails))
		} else {
			result.Message = "No Physical Volumes found"
		}
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckLVM performs Logical Volume Manager monitoring
func (dc *DiskChecker) CheckLVM(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	// Check if LVM tools are available
	lvsCmd := exec.CommandContext(ctx, "lvs", "--noheadings", "--units", "g", "--separator", "|", "-o", "lv_name,vg_name,lv_size,lv_attr,lv_health_status")
	lvsOutput, err := lvsCmd.Output()
	if err != nil {
		result.Status = "Warning"
		result.Message = "LVM not available or no logical volumes found"
		details["error"] = err.Error()
		result.Details = mapToRawExtension(details)
		return result
	}

	lvsStr := strings.TrimSpace(string(lvsOutput))
	details["lvs_output"] = lvsStr

	// Parse logical volumes
	lines := strings.Split(lvsStr, "\n")
	lvDetails := make([]map[string]interface{}, 0)
	criticalLVs := []string{}
	warningLVs := []string{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Split(line, "|")
		if len(fields) >= 5 {
			lvName := strings.TrimSpace(fields[0])
			vgName := strings.TrimSpace(fields[1])
			lvSize := strings.TrimSpace(fields[2])
			lvAttr := strings.TrimSpace(fields[3])
			lvHealth := strings.TrimSpace(fields[4])

			lvInfo := map[string]interface{}{
				"name":   lvName,
				"vg":     vgName,
				"size":   lvSize,
				"attr":   lvAttr,
				"health": lvHealth,
			}

			// Check LV health status
			if lvHealth == "p" || lvHealth == "m" {
				criticalLVs = append(criticalLVs, fmt.Sprintf("%s/%s: %s", vgName, lvName, lvHealth))
			} else if lvHealth != "" && lvHealth != "-" {
				warningLVs = append(warningLVs, fmt.Sprintf("%s/%s: %s", vgName, lvName, lvHealth))
			}

			// Check LV attributes (5th character indicates state)
			if len(lvAttr) >= 5 {
				state := lvAttr[4]
				if state == 's' {
					warningLVs = append(warningLVs, fmt.Sprintf("%s/%s: suspended", vgName, lvName))
				} else if state == 'I' {
					warningLVs = append(warningLVs, fmt.Sprintf("%s/%s: invalid snapshot", vgName, lvName))
				}
			}

			lvDetails = append(lvDetails, lvInfo)
		}
	}

	details["logical_volumes"] = lvDetails

	// Check Volume Groups
	vgsCmd := exec.CommandContext(ctx, "vgs", "--noheadings", "--units", "g", "--separator", "|", "-o", "vg_name,vg_size,vg_free,vg_attr")
	vgsOutput, err := vgsCmd.Output()
	if err == nil {
		vgsStr := strings.TrimSpace(string(vgsOutput))
		details["vgs_output"] = vgsStr

		vgDetails := make([]map[string]interface{}, 0)
		lines := strings.Split(vgsStr, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			fields := strings.Split(line, "|")
			if len(fields) >= 4 {
				vgName := strings.TrimSpace(fields[0])
				vgSizeStr := strings.TrimSpace(fields[1])
				vgFreeStr := strings.TrimSpace(fields[2])
				vgAttr := strings.TrimSpace(fields[3])

				vgInfo := map[string]interface{}{
					"name": vgName,
					"size": vgSizeStr,
					"free": vgFreeStr,
					"attr": vgAttr,
				}

				// Parse size and free to check disk usage
				vgSize := parseSizeToGB(vgSizeStr)
				vgFree := parseSizeToGB(vgFreeStr)
				if vgSize > 0 {
					vgUsed := vgSize - vgFree
					vgUsedPercent := (vgUsed / vgSize) * 100
					vgFreePercent := (vgFree / vgSize) * 100

					vgInfo["size_gb"] = vgSize
					vgInfo["free_gb"] = vgFree
					vgInfo["used_gb"] = vgUsed
					vgInfo["used_percent"] = fmt.Sprintf("%.1f", vgUsedPercent)
					vgInfo["free_percent"] = fmt.Sprintf("%.1f", vgFreePercent)

					// Check for low space
					// Warning threshold set to 5% to reduce false positives
					// In OpenShift clusters, 5-10% free space is often acceptable
					if vgFreePercent < 5 {
						criticalLVs = append(criticalLVs, fmt.Sprintf("VG %s: only %.1f%% free space", vgName, vgFreePercent))
					}
				}

				vgDetails = append(vgDetails, vgInfo)
			}
		}
		details["volume_groups"] = vgDetails
		
		// Check for thin pools and get detailed information
		thinPoolDetails := make([]map[string]interface{}, 0)
		for _, vgInfo := range vgDetails {
			vgName, ok := vgInfo["name"].(string)
			if !ok {
				continue
			}
			
			// Find thin pools in this VG
			thinPoolCmd := exec.CommandContext(ctx, "lvs", "--noheadings", "--units", "g", "--separator", "|", 
				"-o", "lv_name,lv_size,data_percent", vgName)
			thinPoolOutput, err := thinPoolCmd.Output()
			if err != nil {
				continue
			}
			
			thinPoolLines := strings.Split(strings.TrimSpace(string(thinPoolOutput)), "\n")
			for _, line := range thinPoolLines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				
				fields := strings.Split(line, "|")
				if len(fields) >= 3 {
					lvName := strings.TrimSpace(fields[0])
					totalSizeStr := strings.TrimSpace(fields[1])
					dataPercentStr := strings.TrimSpace(fields[2])
					
					// Check if this is a thin pool (usually has "pool" in name or specific attributes)
					// Also check lv_attr for 't' (thin pool) or 'T' (thin pool metadata)
					// For now, we'll check all LVs and identify thin pools by checking data_percent
					if dataPercentStr != "" && dataPercentStr != "-" {
						// This is likely a thin pool
						totalSizeGB := parseSizeToGB(totalSizeStr)
						dataPercent := 0.0
						if percent, err := strconv.ParseFloat(strings.Trim(dataPercentStr, "%"), 64); err == nil {
							dataPercent = percent
						}
						
						usedSizeGB := totalSizeGB * dataPercent / 100
						freeSizeGB := totalSizeGB - usedSizeGB
						
						// Get VG free space
						vgFreeGB := 0.0
						if vgFreeStr, ok := vgInfo["free"].(string); ok {
							vgFreeGB = parseSizeToGB(vgFreeStr)
						}
						
						thinPoolInfo := map[string]interface{}{
							"vg_name":      vgName,
							"thin_pool":    lvName,
							"total_size_gb": totalSizeGB,
							"data_percent": dataPercent,
							"used_size_gb": usedSizeGB,
							"free_size_gb": freeSizeGB,
							"vg_free_gb":   vgFreeGB,
						}
						
						// Check for low space in thin pool
						if dataPercent > 90 {
							criticalLVs = append(criticalLVs, fmt.Sprintf("Thin Pool %s/%s: %.1f%% used", vgName, lvName, dataPercent))
						} else if dataPercent > 80 {
							warningLVs = append(warningLVs, fmt.Sprintf("Thin Pool %s/%s: %.1f%% used", vgName, lvName, dataPercent))
						}
						
						// Check for low VG free space
						if vgFreeGB > 0 && totalSizeGB > 0 {
							vgFreePercent := (vgFreeGB / totalSizeGB) * 100
							if vgFreePercent < 10 {
								warningLVs = append(warningLVs, fmt.Sprintf("VG %s: only %.1f%% free space for thin pool %s", vgName, vgFreePercent, lvName))
							}
						}
						
						thinPoolDetails = append(thinPoolDetails, thinPoolInfo)
					}
				}
			}
		}
		
		if len(thinPoolDetails) > 0 {
			details["thin_pools"] = thinPoolDetails
		}
	}

	details["critical_lvs"] = criticalLVs
	details["warning_lvs"] = warningLVs

	if len(criticalLVs) > 0 {
		result.Status = "Critical"
		result.Message = fmt.Sprintf("Critical LVM issues: %s", strings.Join(criticalLVs, ", "))
	} else if len(warningLVs) > 0 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("Warning LVM issues: %s", strings.Join(warningLVs, ", "))
	} else {
		result.Status = "Healthy"
		if len(lvDetails) > 0 {
			result.Message = fmt.Sprintf("LVM status is normal (%d logical volumes)", len(lvDetails))
		} else {
			result.Message = "No LVM volumes found"
		}
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckIOWait checks disk I/O wait time
func (dc *DiskChecker) CheckIOWait(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	output, err := runHostCommand(ctx, "iostat -x 1 3")
	if err != nil {
		cmd := exec.CommandContext(ctx, "iostat", "-x", "1", "3")
		output, err = cmd.Output()
		if err != nil {
			result.Status = "Warning"
			result.Message = fmt.Sprintf("Failed to execute iostat: %v (iostat may not be installed)", err)
			details["error"] = err.Error()
			details["note"] = "iostat is part of sysstat package. Install with: yum install sysstat or apt-get install sysstat"
			result.Details = mapToRawExtension(details)
			return result
		}
		details["check_source"] = "container"
	} else {
		details["check_source"] = "host"
	}

	iostatOutput := strings.TrimSpace(string(output))
	lines := strings.Split(iostatOutput, "\n")
	
	// Find the last set of statistics
	lastHeaderIndex := -1
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.Contains(lines[i], "Device") && strings.Contains(lines[i], "r/s") {
			lastHeaderIndex = i
			break
		}
	}

	if lastHeaderIndex == -1 {
		result.Status = "Warning"
		result.Message = "Unable to parse iostat output"
		result.Details = mapToRawExtension(details)
		return result
	}

	// Parse device statistics
	highIOWait := []string{}
	maxIOWait := 0.0
	
	for i := lastHeaderIndex + 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		
		fields := strings.Fields(line)
		if len(fields) < 14 {
			continue
		}

		device := fields[0]
		if device == "Device" || strings.HasPrefix(device, "avg-cpu") || strings.HasPrefix(device, "Linux") {
			continue
		}

		// Skip loop and dm devices
		if strings.HasPrefix(device, "loop") || strings.HasPrefix(device, "dm-") {
			continue
		}

		// Parse utilization (field 22 in modern iostat)
		if len(fields) > 22 {
			if util, err := strconv.ParseFloat(fields[22], 64); err == nil {
				if util > 90 {
					highIOWait = append(highIOWait, fmt.Sprintf("%s: %.1f%%", device, util))
					if util > maxIOWait {
						maxIOWait = util
					}
				}
			}
		}
	}

	details["high_io_wait_devices"] = highIOWait
	details["max_io_wait"] = maxIOWait

	if len(highIOWait) > 0 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("High I/O wait detected on %d devices (max: %.1f%%)", len(highIOWait), maxIOWait)
	} else {
		result.Status = "Healthy"
		result.Message = "I/O wait is normal"
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckQueueDepth checks disk I/O queue depth
func (dc *DiskChecker) CheckQueueDepth(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	output, err := runHostCommand(ctx, "iostat -x 1 3")
	if err != nil {
		cmd := exec.CommandContext(ctx, "iostat", "-x", "1", "3")
		output, err = cmd.Output()
		if err != nil {
			result.Status = "Warning"
			result.Message = fmt.Sprintf("Failed to execute iostat: %v", err)
			result.Details = mapToRawExtension(details)
			return result
		}
	}

	iostatOutput := strings.TrimSpace(string(output))
	lines := strings.Split(iostatOutput, "\n")
	
	lastHeaderIndex := -1
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.Contains(lines[i], "Device") && strings.Contains(lines[i], "aqu-sz") {
			lastHeaderIndex = i
			break
		}
	}

	if lastHeaderIndex == -1 {
		result.Status = "Warning"
		result.Message = "Unable to parse iostat output"
		result.Details = mapToRawExtension(details)
		return result
	}

	highQueueDepth := []string{}
	
	for i := lastHeaderIndex + 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		
		fields := strings.Fields(line)
		if len(fields) < 22 {
			continue
		}

		device := fields[0]
		if strings.HasPrefix(device, "loop") || strings.HasPrefix(device, "dm-") {
			continue
		}

		// Parse average queue size (field 21 in modern iostat)
		if len(fields) > 21 {
			if aquSz, err := strconv.ParseFloat(fields[21], 64); err == nil {
				if aquSz > 10.0 {
					highQueueDepth = append(highQueueDepth, fmt.Sprintf("%s: %.2f", device, aquSz))
				}
			}
		}
	}

	details["high_queue_depth_devices"] = highQueueDepth

	if len(highQueueDepth) > 0 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("High queue depth detected on %d devices", len(highQueueDepth))
	} else {
		result.Status = "Healthy"
		result.Message = "Queue depth is normal"
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckFilesystemErrors checks for filesystem errors
func (dc *DiskChecker) CheckFilesystemErrors(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	// Check dmesg for filesystem errors
	output, err := runHostCommand(ctx, "dmesg | grep -i 'filesystem error\\|ext4.*error\\|xfs.*error\\|ext3.*error' | tail -50")
	if err != nil {
		cmd := exec.CommandContext(ctx, "sh", "-c", "dmesg | grep -i 'filesystem error\\|ext4.*error\\|xfs.*error\\|ext3.*error' | tail -50")
		output, err = cmd.Output()
		if err != nil {
			details["check_source"] = "container"
		} else {
			details["check_source"] = "host"
		}
	} else {
		details["check_source"] = "host"
	}

	fsErrorOutput := strings.TrimSpace(string(output))
	details["filesystem_error_output"] = fsErrorOutput

	// Check journalctl if available
	journalOutput, journalErr := runHostCommand(ctx, "journalctl --no-pager --since '7 days ago' | grep -i 'filesystem error\\|ext4.*error\\|xfs.*error' | tail -50")
	if journalErr == nil && len(journalOutput) > 0 {
		details["journal_fs_error_output"] = string(journalOutput)
	}

	errorCount := 0
	if fsErrorOutput != "" {
		errorCount = len(strings.Split(fsErrorOutput, "\n"))
	}

	details["error_count"] = errorCount

	if errorCount > 0 {
		result.Status = "Critical"
		result.Message = fmt.Sprintf("Found %d filesystem error events", errorCount)
	} else {
		result.Status = "Healthy"
		result.Message = "No filesystem errors detected"
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckInodeUsage checks inode usage
func (dc *DiskChecker) CheckInodeUsage(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	highInodeUsage := []string{}
	criticalInodeUsage := []string{}
	processedEntries := make(map[string]bool)

	addEntry := func(filesystem, fsType, mountpoint, ipercent, source string) {
		if ipercent == "" || ipercent == "-" {
			return
		}

		key := fmt.Sprintf("%s|%s", filesystem, mountpoint)
		if processedEntries[key] {
			return
		}

		// Only consider real block devices (skip tmpfs/overlay/loop devices)
		if !strings.HasPrefix(filesystem, "/dev/") || strings.HasPrefix(filesystem, "/dev/loop") {
			return
		}

		// Skip obvious container-specific mounts
		if strings.Contains(mountpoint, "/var/lib/containers/storage/overlay") ||
			strings.Contains(mountpoint, "/var/lib/kubelet/pods") ||
			strings.HasPrefix(mountpoint, "/run/") ||
			strings.HasPrefix(mountpoint, "/sys/") {
			return
		}

		percentStr := strings.TrimSuffix(ipercent, "%")
		percent, err := strconv.Atoi(percentStr)
		if err != nil {
			return
		}

		processedEntries[key] = true
		deviceInfo := filesystem
		if fsType != "" {
			deviceInfo = fmt.Sprintf("%s (%s)", filesystem, fsType)
		}
		entry := fmt.Sprintf("%s on %s: %d%%", deviceInfo, mountpoint, percent)
		if source != "" {
			entry = fmt.Sprintf("%s [%s]", entry, source)
		}

		if percent >= 95 {
			criticalInodeUsage = append(criticalInodeUsage, entry)
		} else if percent >= 85 {
			highInodeUsage = append(highInodeUsage, entry)
		}
	}

	// Sample important mount points explicitly to avoid overlay/tmpfs noise
	importantPaths := []string{"/", "/etc", "/var", "/var/log", "/var/lib/containers", "/var/lib/kubelet", "/boot"}
	checkedImportant := []string{}
	importantSet := make(map[string]bool)
	for _, path := range importantPaths {
		cmd := fmt.Sprintf("df -i %s 2>/dev/null", path)
		output, err := runHostCommand(ctx, cmd)
		if err != nil || len(output) == 0 {
			continue
		}

		outputStr := strings.TrimSpace(string(output))
		lines := strings.Split(outputStr, "\n")
		for i, line := range lines {
			if i == 0 {
				continue // header
			}

			fields := strings.Fields(line)
			if len(fields) < 6 {
				continue
			}

			filesystem := fields[0]
			ipercent := fields[4]
			mountpoint := fields[len(fields)-1]
			addEntry(filesystem, "", mountpoint, ipercent, fmt.Sprintf("path %s", path))
			if !importantSet[path] {
				importantSet[path] = true
				checkedImportant = append(checkedImportant, path)
			}
		}
	}
	details["important_paths_checked"] = checkedImportant

	output, err := runHostCommand(ctx, "df -iPT")
	if err != nil {
		cmd := exec.CommandContext(ctx, "df", "-iPT")
		output, err = cmd.Output()
		if err != nil {
			result.Status = "Warning"
			result.Message = fmt.Sprintf("Failed to check inode usage: %v", err)
			result.Details = mapToRawExtension(details)
			return result
		}
		details["check_source"] = "container"
	} else {
		details["check_source"] = "host"
	}

	dfOutput := strings.TrimSpace(string(output))
	lines := strings.Split(dfOutput, "\n")

	for i, line := range lines {
		if i == 0 {
			continue // Skip header
		}
		
		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}

		filesystem := fields[0]
		fstype := fields[1]
		ipercent := fields[5]
		mountpoint := strings.Join(fields[6:], " ")

		// Skip entries that don't report inode usage (show "-" or 0 total inodes)
		if ipercent == "-" {
			continue
		}

		addEntry(filesystem, fstype, mountpoint, ipercent, "scan")
	}

	details["high_inode_usage"] = highInodeUsage
	details["critical_inode_usage"] = criticalInodeUsage

	if len(criticalInodeUsage) > 0 {
		result.Status = "Critical"
		result.Message = fmt.Sprintf("Critical inode usage on %d filesystems", len(criticalInodeUsage))
	} else if len(highInodeUsage) > 0 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("High inode usage on %d filesystems", len(highInodeUsage))
	} else {
		result.Status = "Healthy"
		result.Message = "Inode usage is normal"
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckMountPoints checks mount points status by looking for actual filesystem errors in dmesg
// This is more reliable than checking read-only mounts, as many read-only mounts are normal
func (dc *DiskChecker) CheckMountPoints(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	// Check dmesg for filesystem remount errors
	remountOutput, err := runHostCommand(ctx, "dmesg | grep -i 'remount'")
	if err != nil {
		cmd := exec.CommandContext(ctx, "sh", "-c", "dmesg | grep -i 'remount'")
		remountOutput, err = cmd.Output()
		if err != nil {
			// If dmesg is not accessible, try journalctl
			cmd = exec.CommandContext(ctx, "sh", "-c", "journalctl -k --no-pager | grep -i 'remount'")
			remountOutput, err = cmd.Output()
			if err != nil {
				details["check_source"] = "container (dmesg/journalctl not accessible)"
			} else {
				details["check_source"] = "container (journalctl)"
			}
		} else {
			details["check_source"] = "host (dmesg)"
		}
	} else {
		details["check_source"] = "host (dmesg)"
	}

	// Check dmesg for read-only filesystem errors
	readonlyOutput, err := runHostCommand(ctx, "dmesg | grep -i 'readonly'")
	if err != nil {
		cmd := exec.CommandContext(ctx, "sh", "-c", "dmesg | grep -i 'readonly'")
		readonlyOutput, err = cmd.Output()
		if err != nil {
			// If dmesg is not accessible, try journalctl
			cmd = exec.CommandContext(ctx, "sh", "-c", "journalctl -k --no-pager | grep -i 'readonly'")
			readonlyOutput, err = cmd.Output()
		}
	}

	remountErrors := []string{}
	readonlyErrors := []string{}

	remountLines := strings.Split(strings.TrimSpace(string(remountOutput)), "\n")
	for _, line := range remountLines {
		line = strings.TrimSpace(line)
		if line != "" {
			lineLower := strings.ToLower(line)
			// Look for actual remount errors (not just informational messages)
			// Common patterns: "remount.*read-only", "remount failed", "error.*remount"
			if (strings.Contains(lineLower, "remount") && strings.Contains(lineLower, "read-only")) ||
				strings.Contains(lineLower, "remount failed") ||
				(strings.Contains(lineLower, "error") && strings.Contains(lineLower, "remount")) ||
				(strings.Contains(lineLower, "warning") && strings.Contains(lineLower, "remount")) {
				remountErrors = append(remountErrors, line)
			}
		}
	}

	readonlyLines := strings.Split(strings.TrimSpace(string(readonlyOutput)), "\n")
	for _, line := range readonlyLines {
		line = strings.TrimSpace(line)
		if line != "" {
			lineLower := strings.ToLower(line)
			// Look for actual read-only filesystem errors
			// Skip normal informational messages about read-only mounts
			// Common error patterns: "read-only filesystem", "remount.*read-only", "error.*read-only"
			if (strings.Contains(lineLower, "error") || strings.Contains(lineLower, "warning") || strings.Contains(lineLower, "failed")) &&
				strings.Contains(lineLower, "read-only") &&
				!strings.Contains(lineLower, "mounted read-only") &&
				!strings.Contains(lineLower, "mounting read-only") {
				readonlyErrors = append(readonlyErrors, line)
			} else if strings.Contains(lineLower, "remount") && strings.Contains(lineLower, "read-only") {
				// Remount to read-only is usually an error
				readonlyErrors = append(readonlyErrors, line)
			}
		}
	}

	// Also get mount points for informational purposes
	mountOutput, err := runHostCommand(ctx, "mount")
	if err != nil {
		cmd := exec.CommandContext(ctx, "sh", "-c", "mount")
		mountOutput, err = cmd.Output()
	}
	
	mountPoints := []string{}
	if err == nil {
		mountLines := strings.Split(strings.TrimSpace(string(mountOutput)), "\n")
		for _, line := range mountLines {
			line = strings.TrimSpace(line)
			if line != "" {
				mountPoints = append(mountPoints, line)
			}
		}
	}

	details["mount_point_count"] = len(mountPoints)
	details["remount_errors"] = remountErrors
	details["readonly_errors"] = readonlyErrors
	details["total_filesystem_errors"] = len(remountErrors) + len(readonlyErrors)

	if len(remountErrors) > 0 || len(readonlyErrors) > 0 {
		result.Status = "Warning"
		errorMessages := []string{}
		if len(remountErrors) > 0 {
			errorMessages = append(errorMessages, fmt.Sprintf("%d remount error(s)", len(remountErrors)))
		}
		if len(readonlyErrors) > 0 {
			errorMessages = append(errorMessages, fmt.Sprintf("%d read-only filesystem error(s)", len(readonlyErrors)))
		}
		result.Message = fmt.Sprintf("Found filesystem errors: %s", strings.Join(errorMessages, ", "))
	} else {
		result.Status = "Healthy"
		result.Message = fmt.Sprintf("No filesystem errors detected (%d mount points)", len(mountPoints))
	}

	result.Details = mapToRawExtension(details)
	return result
}
