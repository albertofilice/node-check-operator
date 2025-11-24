package checks

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/albertofilice/node-check-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SystemChecker handles system-level checks
type SystemChecker struct {
	nodeName        string
	oomWindow       *EventWindow
	panicWindow     *EventWindow
	blockedWindow   *EventWindow
}

// Global event windows for tracking events across checks
var (
	globalOOMWindow     = NewEventWindow(10 * time.Minute)
	globalPanicWindow   = NewEventWindow(30 * time.Minute)
	globalBlockedWindow = NewEventWindow(5 * time.Minute)
)

// NewSystemChecker creates a new system checker
func NewSystemChecker(nodeName string) *SystemChecker {
	return &SystemChecker{
		nodeName: nodeName,
	}
}

// CheckUptime performs uptime and load checks
// Uses /proc/loadavg directly instead of parsing uptime output for better reliability
func (sc *SystemChecker) CheckUptime(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	// Add timeout to context
	ctx, cancel := withTimeout(ctx, 5*time.Second)
	defer cancel()

	// Read load averages directly from /proc/loadavg (locale-independent)
	load1, load5, load15, err := readLoadAvg(ctx)
	if err != nil {
		// Fallback to uptime command if /proc/loadavg is not accessible
		command := "uptime"
		result.Command = command
		output, cmdErr := runHostCommand(ctx, command)
		if cmdErr != nil {
			cmd := exec.CommandContext(ctx, command)
			output, cmdErr = cmd.Output()
			if cmdErr != nil {
				result.Status = "Warning"
				result.Message = fmt.Sprintf("Failed to read load averages: %v (fallback also failed: %v)", err, cmdErr)
				details["check_source"] = "failed"
				result.Details = mapToRawExtension(details)
				return result
			}
			details["check_source"] = "container_fallback"
		} else {
			details["check_source"] = "host_fallback"
		}

		// Parse uptime output as fallback
		uptimeStr := strings.TrimSpace(string(output))
		details["uptime"] = uptimeStr
		parts := strings.Fields(uptimeStr)
		if len(parts) >= 10 {
			load1Str := strings.TrimSuffix(parts[len(parts)-3], ",")
			load5Str := strings.TrimSuffix(parts[len(parts)-2], ",")
			load15Str := parts[len(parts)-1]
			load1, _ = strconv.ParseFloat(load1Str, 64)
			load5, _ = strconv.ParseFloat(load5Str, 64)
			load15, _ = strconv.ParseFloat(load15Str, 64)
		} else {
			result.Status = "Warning"
			result.Message = "Could not parse load averages from uptime output"
			result.Details = mapToRawExtension(details)
			return result
		}
	} else {
		details["check_source"] = "proc_loadavg"
		result.Command = "read /proc/loadavg"
	}

	// Use runtime.NumCPU() instead of executing nproc
	numCores := runtime.NumCPU()
	if numCores == 0 {
		numCores = 1 // Safety fallback
	}
	details["cpu_cores"] = numCores

	details["load_1min"] = load1
	details["load_5min"] = load5
	details["load_15min"] = load15

	// Calculate dynamic thresholds based on number of cores
	warningThreshold := float64(numCores) * 0.75
	criticalThreshold := float64(numCores) * 1.5

	details["warning_threshold"] = warningThreshold
	details["critical_threshold"] = criticalThreshold

	// Determine status based on load average
	// Use 1-minute and 5-minute for sustained load patterns
	if load1 > criticalThreshold || load5 > criticalThreshold {
		result.Status = "Critical"
		result.Message = fmt.Sprintf("Very high load average: %.2f (1m), %.2f (5m), %.2f (15m) - threshold: %.2f (%.0f cores)",
			load1, load5, load15, criticalThreshold, float64(numCores))
	} else if load1 > warningThreshold || load5 > warningThreshold {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("High load average: %.2f (1m), %.2f (5m), %.2f (15m) - threshold: %.2f (%.0f cores)",
			load1, load5, load15, warningThreshold, float64(numCores))
	} else {
		result.Status = "Healthy"
		result.Message = fmt.Sprintf("Load average is normal: %.2f (1m), %.2f (5m), %.2f (15m) - %.0f cores",
			load1, load5, load15, float64(numCores))
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckProcesses performs process monitoring
func (sc *SystemChecker) CheckProcesses(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	// Add timeout to context
	ctx, cancel := withTimeout(ctx, 5*time.Second)
	defer cancel()

	command := "top -bn1 | head -20"
	result.Command = command
	topOutput := ""
	if output, err := runHostCommand(ctx, command); err == nil && len(output) > 0 {
		topOutput = strings.TrimSpace(string(output))
		details["top_output"] = topOutput
		details["check_source"] = "host"
		result.Command = command
	} else {
		// Fallback to container processes if host access fails
		cmd := exec.CommandContext(ctx, "sh", "-c", command)
		output, err := cmd.Output()
		if err != nil {
			// Don't mark as Critical for transient failures
			if ctx.Err() == context.DeadlineExceeded {
				result.Status = "Warning"
				result.Message = fmt.Sprintf("Process check timed out: %v", err)
			} else {
				result.Status = "Warning"
				result.Message = fmt.Sprintf("Failed to execute top: %v (showing container processes)", err)
			}
			details["check_source"] = "failed"
			result.Details = mapToRawExtension(details)
			return result
		}
		topOutput = strings.TrimSpace(string(output))
		details["top_output"] = topOutput
		details["check_source"] = "container"
		result.Command = command
		result.Message = "Warning: Showing container processes (host access unavailable)"
	}

	// Parse CPU and memory usage from top output
	lines := strings.Split(topOutput, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Cpu(s)") {
			// Extract CPU usage
			parts := strings.Fields(line)
			for i, part := range parts {
				if strings.Contains(part, "%") {
					if i > 0 {
						cpuStr := strings.Trim(part, "%")
						if cpu, err := strconv.ParseFloat(cpuStr, 64); err == nil {
							details["cpu_usage"] = cpu
							if cpu > 90 {
								result.Status = "Warning"
								result.Message = fmt.Sprintf("High CPU usage: %.1f%%", cpu)
							} else {
								result.Status = "Healthy"
								result.Message = "CPU usage is normal"
							}
						}
					}
					break
				}
			}
		}
		if strings.Contains(line, "Mem:") {
			// Extract memory usage
			parts := strings.Fields(line)
			for i, part := range parts {
				if strings.Contains(part, "used") && i > 0 {
					memStr := strings.Trim(parts[i-1], "k")
					if mem, err := strconv.ParseInt(memStr, 10, 64); err == nil {
						details["memory_used_kb"] = mem
					}
				}
			}
		}
	}

	if result.Status == "Unknown" {
		result.Status = "Healthy"
		result.Message = "Process monitoring completed"
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckResources performs resource monitoring using vmstat
func (sc *SystemChecker) CheckResources(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	// Add timeout to context (vmstat takes ~3 seconds for 3 samples)
	ctx, cancel := withTimeout(ctx, 8*time.Second)
	defer cancel()

	command := "vmstat 1 3"
	result.Command = command
	output, err := runHostCommand(ctx, command)
	if err != nil {
		cmd := exec.CommandContext(ctx, "vmstat", "1", "3")
		output, err = cmd.Output()
		if err != nil {
			// Don't mark as Critical for transient failures
			if ctx.Err() == context.DeadlineExceeded {
				result.Status = "Warning"
				result.Message = fmt.Sprintf("Resource check timed out: %v", err)
			} else {
				result.Status = "Warning"
				result.Message = fmt.Sprintf("Failed to execute vmstat: %v", err)
			}
			details["check_source"] = "failed"
			result.Details = mapToRawExtension(details)
			return result
		}
		details["check_source"] = "container"
		result.Command = command
	} else {
		details["check_source"] = "host"
		result.Command = command
	}

	vmstatOutput := strings.TrimSpace(string(output))
	details["vmstat_output"] = vmstatOutput

	// Parse vmstat output
	lines := strings.Split(vmstatOutput, "\n")
	if len(lines) >= 4 {
		// Skip header lines and get the last non-empty line (average)
		// vmstat outputs multiple samples, we want the last one
		lastLine := ""
		for i := len(lines) - 1; i >= 0; i-- {
			line := strings.TrimSpace(lines[i])
			if line != "" && !strings.HasPrefix(line, "procs") && !strings.HasPrefix(line, " r ") {
				lastLine = line
				break
			}
		}
		
		if lastLine == "" {
			result.Status = "Warning"
			result.Message = "Could not parse vmstat output (no data lines found)"
			result.Details = mapToRawExtension(details)
			return result
		}
		
		fields := strings.Fields(lastLine)
		
		// Modern vmstat has 18 fields (includes 'gu' for guest time), older versions have 17
		if len(fields) >= 17 {
			// Parse key metrics
			if r, err := strconv.ParseInt(fields[0], 10, 64); err == nil {
				details["runnable_processes"] = r
			}
			if b, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
				details["blocked_processes"] = b
			}
			if swpd, err := strconv.ParseInt(fields[2], 10, 64); err == nil {
				details["swap_used_kb"] = swpd
			}
			if free, err := strconv.ParseInt(fields[3], 10, 64); err == nil {
				details["free_memory_kb"] = free
			}
			if si, err := strconv.ParseInt(fields[6], 10, 64); err == nil {
				details["swap_in_per_sec"] = si
			}
			if so, err := strconv.ParseInt(fields[7], 10, 64); err == nil {
				details["swap_out_per_sec"] = so
			}
			if us, err := strconv.ParseInt(fields[12], 10, 64); err == nil {
				details["cpu_user_percent"] = us
			}
			if sy, err := strconv.ParseInt(fields[13], 10, 64); err == nil {
				details["cpu_system_percent"] = sy
			}
			if id, err := strconv.ParseInt(fields[14], 10, 64); err == nil {
				details["cpu_idle_percent"] = id
			}
		}
	}

	// Check for high swap usage
	if swapUsed, ok := details["swap_used_kb"].(int64); ok && swapUsed > 0 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("Swap is being used: %d KB", swapUsed)
	} else {
		result.Status = "Healthy"
		result.Message = "Resource usage is normal"
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckServices performs service status checks
func (sc *SystemChecker) CheckServices(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	// Check for failed services directly on the host
	command := "systemctl --failed --no-pager"
	result.Command = command
	output, err := runHostCommand(ctx, command)
	if err != nil || len(output) == 0 {
		journalCommand := "journalctl --no-pager -u '*.service' --since '1 hour ago' --priority=err --no-hostname | head -50"
		journalOutput, journalErr := runHostCommand(ctx, journalCommand)
		if journalErr == nil && len(journalOutput) > 0 {
			output = journalOutput
			err = nil
			result.Command = journalCommand
			details["check_method"] = "journalctl"
		}
	}
	
	// If still no output, check if systemd is available on the host
	if err != nil || len(output) == 0 {
		// Check if host uses systemd
		systemdCommand := "test -d /run/systemd && echo systemd || echo no-systemd"
		systemdCheck, sysErr := runHostCommand(ctx, systemdCommand)
		if sysErr == nil && strings.Contains(string(systemdCheck), "no-systemd") {
			result.Status = "Warning"
			result.Message = "Service check not available (host does not use systemd)"
			details["note"] = "Host uses non-systemd init system (e.g., busybox). Service monitoring requires systemd."
			result.Details = mapToRawExtension(details)
			return result
		}
		
		result.Status = "Warning"
		result.Message = "Service check not available (cannot access host systemd via nsenter)"
		details["error"] = "nsenter or systemd access failed"
		result.Details = mapToRawExtension(details)
		return result
	}

	serviceOutput := strings.TrimSpace(string(output))
	details["failed_services"] = serviceOutput

	// Count failed services
	lines := strings.Split(serviceOutput, "\n")
	failedCount := 0
	serviceNames := []string{}
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "UNIT") && !strings.HasPrefix(line, "LOAD") && !strings.HasPrefix(line, "ACTIVE") && !strings.HasPrefix(line, "SUB") && !strings.HasPrefix(line, "DESCRIPTION") && !strings.Contains(line, "lines") {
			// Check if this looks like a service line
			if strings.Contains(line, "failed") || strings.Contains(line, ".service") {
				failedCount++
				// Extract service name if possible
				fields := strings.Fields(line)
				if len(fields) > 0 {
					serviceName := fields[0]
					if strings.HasSuffix(serviceName, ".service") {
						serviceNames = append(serviceNames, serviceName)
					}
				}
			}
		}
	}

	details["failed_service_count"] = failedCount
	if len(serviceNames) > 0 {
		details["failed_service_names"] = serviceNames
	}

	if failedCount > 0 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("Found %d failed services", failedCount)
	} else {
		result.Status = "Healthy"
		result.Message = "All services are running normally"
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckMemory performs memory monitoring
// Uses /proc/meminfo directly instead of parsing free -h output for better reliability
func (sc *SystemChecker) CheckMemory(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	// Add timeout to context
	ctx, cancel := withTimeout(ctx, 5*time.Second)
	defer cancel()

	// Read memory information directly from /proc/meminfo
	total, available, free, used, buffers, cached, err := readMemInfo(ctx)
	if err != nil {
		// Fallback to free command if /proc/meminfo is not accessible
		command := "free -h"
		result.Command = command
		output, cmdErr := runHostCommand(ctx, command)
		if cmdErr != nil {
			cmd := exec.CommandContext(ctx, "free", "-h")
			output, cmdErr = cmd.Output()
			if cmdErr != nil {
				result.Status = "Warning"
				result.Message = fmt.Sprintf("Failed to read memory info: %v (fallback also failed: %v)", err, cmdErr)
				details["check_source"] = "failed"
				result.Details = mapToRawExtension(details)
				return result
			}
			details["check_source"] = "container_fallback"
		} else {
			details["check_source"] = "host_fallback"
		}

		// Parse free output as fallback
		freeOutput := strings.TrimSpace(string(output))
		details["free_output"] = freeOutput
		lines := strings.Split(freeOutput, "\n")
		if len(lines) >= 2 {
			fields := strings.Fields(lines[1])
			if len(fields) >= 7 {
				totalStr := fields[1]
				usedStr := fields[2]
				freeStr := fields[3]
				sharedStr := fields[4]
				buffCacheStr := fields[5]
				availableStr := fields[6]

				details["total_memory"] = totalStr
				details["used_memory"] = usedStr
				details["free_memory"] = freeStr
				details["shared_memory"] = sharedStr
				details["buff_cache"] = buffCacheStr
				details["available_memory"] = availableStr

				if totalBytes, parseErr := parseMemorySize(totalStr); parseErr == nil {
					if usedBytes, parseErr := parseMemorySize(usedStr); parseErr == nil {
						usagePercent := float64(usedBytes) / float64(totalBytes) * 100
						details["memory_usage_percent"] = usagePercent

						if usagePercent > 90 {
							result.Status = "Critical"
							result.Message = fmt.Sprintf("Very high memory usage: %.1f%%", usagePercent)
						} else if usagePercent > 80 {
							result.Status = "Warning"
							result.Message = fmt.Sprintf("High memory usage: %.1f%%", usagePercent)
						} else {
							result.Status = "Healthy"
							result.Message = "Memory usage is normal"
						}
					}
				}
			}
		}

		if result.Status == "Unknown" {
			result.Status = "Warning"
			result.Message = "Memory check completed (using fallback method)"
		}

		result.Details = mapToRawExtension(details)
		return result
	}

	// Use /proc/meminfo data
	details["check_source"] = "proc_meminfo"
	result.Command = "read /proc/meminfo"

	// Convert to human-readable format for display
	details["total_memory_bytes"] = total
	details["available_memory_bytes"] = available
	details["free_memory_bytes"] = free
	details["used_memory_bytes"] = used
	details["buffers_bytes"] = buffers
	details["cached_bytes"] = cached

	// Calculate usage percentage using available memory (more accurate)
	var usagePercent float64
	if available > 0 {
		usagePercent = float64(total-available) / float64(total) * 100
	} else {
		// Fallback if MemAvailable is not available
		usagePercent = float64(used) / float64(total) * 100
	}
	details["memory_usage_percent"] = usagePercent

	// Determine status
	if usagePercent > 90 {
		result.Status = "Critical"
		result.Message = fmt.Sprintf("Very high memory usage: %.1f%% (%.1f GB used / %.1f GB total)",
			usagePercent, float64(used)/(1024*1024*1024), float64(total)/(1024*1024*1024))
	} else if usagePercent > 80 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("High memory usage: %.1f%% (%.1f GB used / %.1f GB total)",
			usagePercent, float64(used)/(1024*1024*1024), float64(total)/(1024*1024*1024))
	} else {
		result.Status = "Healthy"
		result.Message = fmt.Sprintf("Memory usage is normal: %.1f%% (%.1f GB available / %.1f GB total)",
			usagePercent, float64(available)/(1024*1024*1024), float64(total)/(1024*1024*1024))
	}

	result.Details = mapToRawExtension(details)
	return result
}

// parseMemorySize parses memory size strings like "8.2Gi" or "1024Mi"
func parseMemorySize(sizeStr string) (int64, error) {
	sizeStr = strings.ToUpper(sizeStr)
	
	var multiplier int64 = 1
	if strings.HasSuffix(sizeStr, "G") {
		multiplier = 1024 * 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "G")
	} else if strings.HasSuffix(sizeStr, "M") {
		multiplier = 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "M")
	} else if strings.HasSuffix(sizeStr, "K") {
		multiplier = 1024
		sizeStr = strings.TrimSuffix(sizeStr, "K")
	}

	// Handle decimal values
	if strings.Contains(sizeStr, ".") {
		parts := strings.Split(sizeStr, ".")
		if len(parts) == 2 {
			whole, err := strconv.ParseInt(parts[0], 10, 64)
			if err != nil {
				return 0, err
			}
			decimal, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				return 0, err
			}
			// Simple decimal handling
			return (whole*multiplier + decimal*multiplier/10), nil
		}
	}

	value, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		return 0, err
	}

	return value * multiplier, nil
}

// CheckUninterruptibleTasks checks for tasks in uninterruptible sleep state (D state)
// This is important because Linux load averages include these tasks, which can indicate
// I/O wait issues. Based on Brendan Gregg's analysis:
// https://www.brendangregg.com/blog/2017-08-08/linux-load-averages.html
// Uses sliding window to avoid false positives from transient spikes
func (sc *SystemChecker) CheckUninterruptibleTasks(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	// Add timeout to context
	ctx, cancel := withTimeout(ctx, 5*time.Second)
	defer cancel()

	// Read procs_blocked directly from /proc/stat
	blocked, err := getProcsBlocked(ctx)
	if err != nil {
		// Fallback to command
		command := "cat /proc/stat"
		result.Command = command
		statOutput, cmdErr := runHostCommand(ctx, command)
		if cmdErr != nil {
			cmd := exec.CommandContext(ctx, "cat", "/proc/stat")
			statOutput, cmdErr = cmd.Output()
			if cmdErr != nil {
				result.Status = "Warning"
				result.Message = fmt.Sprintf("Failed to read /proc/stat: %v (fallback also failed: %v)", err, cmdErr)
				details["check_source"] = "failed"
				result.Details = mapToRawExtension(details)
				return result
			}
			details["check_source"] = "container_fallback"
		} else {
			details["check_source"] = "host_fallback"
		}

		// Parse output
		lines := strings.Split(string(statOutput), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "procs_blocked") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					if parsed, parseErr := strconv.Atoi(fields[1]); parseErr == nil {
						blocked = parsed
						break
					}
				}
			}
		}
	} else {
		details["check_source"] = "proc_stat"
		result.Command = "read /proc/stat"
	}

	details["procs_blocked"] = blocked

	// Use sliding window to track sustained high blocked processes
	if blocked > 5 {
		globalBlockedWindow.Add()
	}
	recentHighCount := globalBlockedWindow.Count()
	details["recent_high_blocked_count"] = recentHighCount
	details["window_duration_minutes"] = 5

	// Also read /proc/loadavg for context
	load1, load5, load15, loadErr := readLoadAvg(ctx)
	if loadErr == nil {
		details["load_1min"] = load1
		details["load_5min"] = load5
		details["load_15min"] = load15
	}

	// Determine status with debouncing: require sustained high count
	// Critical only if we've seen high blocked count multiple times in the window
	if blocked > 10 && recentHighCount >= 3 {
		result.Status = "Critical"
		result.Message = fmt.Sprintf("High number of uninterruptible tasks: %d (sustained, may indicate I/O wait issues)", blocked)
	} else if blocked > 10 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("High number of uninterruptible tasks: %d (transient, monitoring)", blocked)
	} else if blocked > 5 && recentHighCount >= 2 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("Elevated number of uninterruptible tasks: %d (sustained)", blocked)
	} else if blocked > 5 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("Elevated number of uninterruptible tasks: %d (transient)", blocked)
	} else {
		result.Status = "Healthy"
		result.Message = fmt.Sprintf("Uninterruptible tasks count is normal: %d", blocked)
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckSystemLogs performs system log monitoring using journalctl
func (sc *SystemChecker) CheckSystemLogs(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	command := "journalctl --no-pager -p err --since '1 hour ago' --no-hostname"
	result.Command = command
	output, err := runHostCommand(ctx, command)
	if err != nil || len(output) == 0 {
		systemdCheck, sysErr := runHostCommand(ctx, "test -d /run/systemd && echo systemd || echo no-systemd")
		if sysErr == nil && strings.Contains(string(systemdCheck), "no-systemd") {
			result.Status = "Warning"
			result.Message = "System logs check not available (host does not use systemd)"
			details["note"] = "Host uses non-systemd init system (e.g., busybox). System log monitoring requires systemd/journald."
			result.Details = mapToRawExtension(details)
			return result
		}

		result.Status = "Warning"
		result.Message = "System logs check not available (cannot access host journal via nsenter)"
		details["error"] = "nsenter or journalctl access failed"
		result.Details = mapToRawExtension(details)
		return result
	}

	logOutput := strings.TrimSpace(string(output))
	lines := strings.Split(logOutput, "\n")
	filteredLines := []string{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}
		filteredLines = append(filteredLines, line)
	}

	errorCount := len(filteredLines)
	criticalErrors := []string{}
	const sampleLimit = 10
	displayLines := filteredLines
	if len(filteredLines) > sampleLimit {
		displayLines = filteredLines[:sampleLimit]
		details["recent_errors_truncated"] = true
	}
	if len(displayLines) == 0 {
		details["recent_errors"] = "-- No entries --"
	} else {
		details["recent_errors"] = strings.Join(displayLines, "\n")
	}

	for _, line := range filteredLines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "panic") ||
			strings.Contains(lower, "fatal") ||
			strings.Contains(lower, "kernel") ||
			strings.Contains(lower, "oom") {
			criticalErrors = append(criticalErrors, line)
		}
	}

	// Explicitly document the time window being checked
	details["check_time_window"] = "1 hour"
	details["note"] = "Only errors from the last hour are checked. Errors that occurred earlier or have been resolved may not appear."
	details["error_count"] = errorCount
	details["critical_errors"] = criticalErrors

	// Check for system reboots in the last 24 hours
	if rebootOutput, err := runHostCommand(ctx, "journalctl --no-pager --list-boots --no-hostname | tail -5"); err == nil && len(rebootOutput) > 0 {
		rebootLines := strings.Split(strings.TrimSpace(string(rebootOutput)), "\n")
		details["recent_boots"] = len(rebootLines)
		if len(rebootLines) > 1 {
			details["last_boot"] = rebootLines[len(rebootLines)-1]
		}
	}

	// Determine status based on errors
	if len(criticalErrors) > 0 {
		result.Status = "Critical"
		result.Message = fmt.Sprintf("Found %d critical errors in system logs (last hour)", len(criticalErrors))
	} else if errorCount == 0 {
		result.Status = "Healthy"
		result.Message = "No errors found in system logs (last hour)"
	} else if errorCount > 10 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("Found %d errors in system logs in the last hour", errorCount)
	} else {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("Found %d errors in system logs in the last hour", errorCount)
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckFileDescriptors checks file descriptor usage
func (sc *SystemChecker) CheckFileDescriptors(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	// Add timeout to context
	ctx, cancel := withTimeout(ctx, 5*time.Second)
	defer cancel()

	// Try to read directly from /proc first
	fileNrPath := "/proc/sys/fs/file-nr"
	data, err := readProcFile(ctx, fileNrPath)
	if err != nil {
		// Fallback to command
		command := "cat /proc/sys/fs/file-nr"
		result.Command = command
		output, cmdErr := runHostCommand(ctx, command)
		if cmdErr != nil {
			cmd := exec.CommandContext(ctx, "sh", "-c", command)
			output, cmdErr = cmd.Output()
			if cmdErr != nil {
				result.Status = "Warning"
				result.Message = fmt.Sprintf("Failed to read file descriptor stats: %v (fallback also failed: %v)", err, cmdErr)
				details["check_source"] = "failed"
				result.Details = mapToRawExtension(details)
				return result
			}
			details["check_source"] = "container_fallback"
			data = output
		} else {
			details["check_source"] = "host_fallback"
			data = output
		}
	} else {
		details["check_source"] = "proc_file-nr"
		result.Command = "read /proc/sys/fs/file-nr"
	}

	fields := strings.Fields(strings.TrimSpace(string(data)))
	if len(fields) >= 3 {
		if allocated, err := strconv.ParseInt(fields[0], 10, 64); err == nil {
			details["allocated"] = allocated
		}
		if unused, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
			details["unused"] = unused
		}
		if max, err := strconv.ParseInt(fields[2], 10, 64); err == nil {
			details["max"] = max
			if allocated, ok := details["allocated"].(int64); ok {
				usagePercent := float64(allocated) / float64(max) * 100
				details["usage_percent"] = usagePercent
				if usagePercent > 90 {
					result.Status = "Critical"
					result.Message = fmt.Sprintf("File descriptor usage is critical: %.1f%% (%d/%d)", usagePercent, allocated, max)
				} else if usagePercent > 80 {
					result.Status = "Warning"
					result.Message = fmt.Sprintf("File descriptor usage is high: %.1f%% (%d/%d)", usagePercent, allocated, max)
				} else {
					result.Status = "Healthy"
					result.Message = fmt.Sprintf("File descriptor usage is normal: %.1f%% (%d/%d)", usagePercent, allocated, max)
				}
			}
		}
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckZombieProcesses checks for zombie processes
func (sc *SystemChecker) CheckZombieProcesses(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	// Add timeout to context
	ctx, cancel := withTimeout(ctx, 5*time.Second)
	defer cancel()

	command := "ps -eo stat | awk '/^Z/ {c++} END {print c+0}'"
	result.Command = command
	output, err := runHostCommand(ctx, command)
	if err != nil {
		cmd := exec.CommandContext(ctx, "sh", "-c", command)
		output, err = cmd.Output()
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				result.Status = "Warning"
				result.Message = fmt.Sprintf("Zombie process check timed out: %v", err)
			} else {
				result.Status = "Warning"
				result.Message = fmt.Sprintf("Failed to check zombie processes: %v", err)
			}
			details["check_source"] = "failed"
			result.Details = mapToRawExtension(details)
			return result
		}
		details["check_source"] = "container"
		result.Command = command
	} else {
		details["check_source"] = "host"
		result.Command = command
	}

	outputStr := strings.TrimSpace(string(output))
	var zombieCount int
	if outputStr == "" {
		// Empty output means no zombies found
		zombieCount = 0
	} else {
		zombieCount, err = strconv.Atoi(outputStr)
		if err != nil {
			result.Status = "Warning"
			result.Message = fmt.Sprintf("Failed to parse zombie process count: %v (output: %s)", err, outputStr)
			details["raw_output"] = outputStr
			result.Details = mapToRawExtension(details)
			return result
		}
	}

	details["zombie_count"] = zombieCount

	if zombieCount > 10 {
		result.Status = "Critical"
		result.Message = fmt.Sprintf("High number of zombie processes: %d", zombieCount)
	} else if zombieCount > 0 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("Zombie processes detected: %d", zombieCount)
	} else {
		result.Status = "Healthy"
		result.Message = "No zombie processes found"
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckNTPSync checks NTP/chrony synchronization status
func (sc *SystemChecker) CheckNTPSync(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	// Try chronyd first (common on RHEL/CentOS)
	command := "chronyc tracking 2>/dev/null"
	result.Command = command
	output, err := runHostCommand(ctx, command)
	if err == nil && len(output) > 0 {
		details["ntp_daemon"] = "chronyd"
		details["chronyc_output"] = string(output)
		result.Command = command
		
		// Check if synchronized
		if strings.Contains(string(output), "Leap status") {
			if strings.Contains(string(output), "Normal") {
				result.Status = "Healthy"
				result.Message = "NTP synchronization is normal (chronyd)"
			} else {
				result.Status = "Warning"
				result.Message = "NTP synchronization may have issues (chronyd)"
			}
		}
		result.Details = mapToRawExtension(details)
		return result
	}

	// Try ntpd
	ntpdCommand := "ntpq -p 2>/dev/null"
	result.Command = ntpdCommand
	output, err = runHostCommand(ctx, ntpdCommand)
	if err == nil && len(output) > 0 {
		details["ntp_daemon"] = "ntpd"
		details["ntpq_output"] = string(output)
		result.Command = ntpdCommand
		
		// Check for synchronized peers
		if strings.Contains(string(output), "*") {
			result.Status = "Healthy"
			result.Message = "NTP synchronization is normal (ntpd)"
		} else {
			result.Status = "Warning"
			result.Message = "No synchronized NTP peers found (ntpd)"
		}
		result.Details = mapToRawExtension(details)
		return result
	}

	// Try systemd-timesyncd
	timesyncdCommand := "timedatectl status 2>/dev/null"
	result.Command = timesyncdCommand
	output, err = runHostCommand(ctx, timesyncdCommand)
	if err == nil && len(output) > 0 {
		details["ntp_daemon"] = "systemd-timesyncd"
		details["timedatectl_output"] = string(output)
		result.Command = timesyncdCommand
		
		if strings.Contains(string(output), "synchronized: yes") {
			result.Status = "Healthy"
			result.Message = "NTP synchronization is normal (systemd-timesyncd)"
		} else {
			result.Status = "Warning"
			result.Message = "NTP synchronization may have issues (systemd-timesyncd)"
		}
		result.Details = mapToRawExtension(details)
		return result
	}

	result.Status = "Warning"
	result.Message = "NTP daemon not found or not accessible"
	result.Command = "chronyc tracking || ntpq -p || timedatectl status (none available)"
	details["note"] = "No NTP daemon (chronyd, ntpd, or systemd-timesyncd) found or accessible"
	result.Details = mapToRawExtension(details)
	return result
}

// CheckKernelPanics checks for kernel panics in system logs
// Uses sliding window to track panic events over time
func (sc *SystemChecker) CheckKernelPanics(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	// Add timeout to context
	ctx, cancel := withTimeout(ctx, 10*time.Second)
	defer cancel()

	// Check dmesg for kernel panics
	command := "dmesg | grep -i 'kernel panic\\|Oops\\|BUG' | tail -20"
	result.Command = command
	output, err := runHostCommand(ctx, command)
	if err != nil {
		cmd := exec.CommandContext(ctx, "sh", "-c", command)
		output, err = cmd.Output()
		if err != nil {
			details["check_source"] = "container"
			result.Command = command
		} else {
			details["check_source"] = "host"
			result.Command = command
		}
	} else {
		details["check_source"] = "host"
		result.Command = command
	}

	panicOutput := strings.TrimSpace(string(output))
	details["panic_output"] = panicOutput

	// Also check journalctl if available
	journalCommand := "journalctl --no-pager -k --since '30 days ago' | grep -i 'kernel panic\\|Oops\\|BUG' | tail -20"
	journalOutput, journalErr := runHostCommand(ctx, journalCommand)
	if journalErr == nil && len(journalOutput) > 0 {
		details["journal_panic_output"] = string(journalOutput)
		// Combine outputs
		if panicOutput != "" {
			panicOutput = panicOutput + "\n" + string(journalOutput)
		} else {
			panicOutput = string(journalOutput)
		}
	}

	panicCount := 0
	if panicOutput != "" {
		lines := strings.Split(panicOutput, "\n")
		seen := make(map[string]bool)
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !seen[line] {
				seen[line] = true
				panicCount++
				// Add to sliding window for each unique panic event
				globalPanicWindow.Add()
			}
		}
	}

	details["panic_count"] = panicCount
	recentPanicCount := globalPanicWindow.Count()
	details["recent_panic_count_in_window"] = recentPanicCount
	details["window_duration_minutes"] = 30
	if lastEvent := globalPanicWindow.LastEvent(); !lastEvent.IsZero() {
		details["last_panic_event"] = lastEvent.Format(time.RFC3339)
	}

	// Kernel panics are always serious, but use window to indicate if it's ongoing
	if panicCount > 0 || recentPanicCount > 0 {
		result.Status = "Critical"
		if recentPanicCount > 0 {
			result.Message = fmt.Sprintf("Found %d kernel panic/Oops/BUG events (recent: %d in last 30 min)", panicCount, recentPanicCount)
		} else {
			result.Message = fmt.Sprintf("Found %d kernel panic/Oops/BUG events", panicCount)
		}
	} else {
		result.Status = "Healthy"
		result.Message = "No kernel panics detected"
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckOOMKiller checks for OOM killer events
// Uses sliding window to avoid false positives from transient events
func (sc *SystemChecker) CheckOOMKiller(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	// Add timeout to context
	ctx, cancel := withTimeout(ctx, 10*time.Second)
	defer cancel()

	// Check dmesg for OOM killer events (limit to last hour to avoid stale entries)
	dmesgCmd := "dmesg --since=-1h | grep -i 'Out of memory\\|oom-killer\\|killed process' | tail -20"
	result.Command = dmesgCmd
	output, err := runHostCommand(ctx, dmesgCmd)
	if err != nil || len(output) == 0 {
		// Fallback: some distros may not support --since on dmesg
		fallbackCmd := "dmesg --ctime | tail -200 | grep -i 'Out of memory\\|oom-killer\\|killed process'"
		result.Command = fallbackCmd
		output, err = runHostCommand(ctx, fallbackCmd)
		if err != nil {
			cmd := exec.CommandContext(ctx, "sh", "-c", fallbackCmd)
			output, err = cmd.Output()
			if err != nil {
				details["check_source"] = "container"
				result.Command = fallbackCmd
			} else {
				details["check_source"] = "host"
				result.Command = fallbackCmd
			}
		} else {
			details["check_source"] = "host"
			result.Command = fallbackCmd
		}
	} else {
		details["check_source"] = "host"
		result.Command = dmesgCmd
	}

	oomOutput := strings.TrimSpace(string(output))
	details["oom_output"] = oomOutput

	// Also check journalctl if available - only check last hour to avoid false positives
	journalCommand := "journalctl --no-pager --since '1 hour ago' | grep -i 'Out of memory\\|oom-killer\\|killed process' | tail -20"
	journalOutput, journalErr := runHostCommand(ctx, journalCommand)
	if journalErr == nil && len(journalOutput) > 0 {
		journalStr := strings.TrimSpace(string(journalOutput))
		details["journal_oom_output"] = journalStr
		// Combine with dmesg output
		if oomOutput != "" {
			oomOutput = oomOutput + "\n" + journalStr
		} else {
			oomOutput = journalStr
		}
	}

	oomCount := 0
	if oomOutput != "" {
		// Count unique lines (remove duplicates)
		lines := strings.Split(oomOutput, "\n")
		seen := make(map[string]bool)
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !seen[line] {
				seen[line] = true
				oomCount++
				// Add to sliding window for each unique OOM event
				globalOOMWindow.Add()
			}
		}
	}

	details["oom_count"] = oomCount
	recentOOMCount := globalOOMWindow.Count()
	details["recent_oom_count_in_window"] = recentOOMCount
	details["window_duration_minutes"] = 10
	if lastEvent := globalOOMWindow.LastEvent(); !lastEvent.IsZero() {
		details["last_oom_event"] = lastEvent.Format(time.RFC3339)
	}

	// Use sliding window: Warning after 1 event, Critical only if multiple events in window
	if recentOOMCount >= 3 {
		result.Status = "Critical"
		result.Message = fmt.Sprintf("Found %d OOM killer events in the last 10 minutes (sustained issue)", recentOOMCount)
	} else if oomCount > 0 || recentOOMCount > 0 {
		result.Status = "Warning"
		if recentOOMCount > 0 {
			result.Message = fmt.Sprintf("Found %d OOM killer events (recent: %d in last 10 min)", oomCount, recentOOMCount)
		} else {
			result.Message = fmt.Sprintf("Found %d OOM killer events", oomCount)
		}
	} else {
		result.Status = "Healthy"
		result.Message = "No OOM killer events detected"
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckCPUFrequency checks CPU frequency scaling and throttling
func (sc *SystemChecker) CheckCPUFrequency(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	// Check CPU frequency scaling governor
	command := "cat /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor 2>/dev/null | sort -u"
	result.Command = command
	output, err := runHostCommand(ctx, command)
	if err != nil {
		details["note"] = "CPU frequency scaling information not available"
		result.Status = "Warning"
		result.Message = "CPU frequency scaling check not available"
		result.Details = mapToRawExtension(details)
		return result
	}
	result.Command = command

	governors := strings.Split(strings.TrimSpace(string(output)), "\n")
	details["governors"] = governors

	// Check for throttling events
	throttleCommand := "cat /sys/devices/system/cpu/cpu*/thermal_throttle/* 2>/dev/null | head -20"
	throttleOutput, throttleErr := runHostCommand(ctx, throttleCommand)
	if throttleErr == nil && len(throttleOutput) > 0 {
		details["throttle_info"] = string(throttleOutput)
	}

	// Check current frequencies
	freqCommand := "cat /sys/devices/system/cpu/cpu*/cpufreq/scaling_cur_freq 2>/dev/null | head -5"
	freqOutput, freqErr := runHostCommand(ctx, freqCommand)
	if freqErr == nil && len(freqOutput) > 0 {
		details["current_frequencies_sample"] = string(freqOutput)
	}

	result.Status = "Healthy"
	result.Message = fmt.Sprintf("CPU frequency scaling active (governors: %s)", strings.Join(governors, ", "))
	result.Details = mapToRawExtension(details)
	return result
}

// CheckInterruptsBalance checks interrupt distribution across CPU cores
func (sc *SystemChecker) CheckInterruptsBalance(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	command := "cat /proc/interrupts | head -30"
	result.Command = command
	output, err := runHostCommand(ctx, command)
	if err != nil {
		cmd := exec.CommandContext(ctx, "sh", "-c", command)
		output, err = cmd.Output()
		if err != nil {
			result.Status = "Warning"
			result.Message = fmt.Sprintf("Failed to read interrupt statistics: %v", err)
			result.Details = mapToRawExtension(details)
			return result
		}
		details["check_source"] = "container"
		result.Command = command
	} else {
		details["check_source"] = "host"
		result.Command = command
	}

	interruptsOutput := strings.TrimSpace(string(output))
	details["interrupts_sample"] = interruptsOutput

	// Count CPU cores from first line
	lines := strings.Split(interruptsOutput, "\n")
	if len(lines) > 0 {
		headerFields := strings.Fields(lines[0])
		cpuCount := len(headerFields) - 1 // Subtract the first column (IRQ name)
		details["cpu_count"] = cpuCount
	}

	result.Status = "Healthy"
	result.Message = "Interrupt balance check completed"
	result.Details = mapToRawExtension(details)
	return result
}

// CheckCPUStealTime checks CPU steal time (important in virtualized environments)
func (sc *SystemChecker) CheckCPUStealTime(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	command := "top -bn1 | grep -i 'Cpu(s)'"
	result.Command = command
	output, err := runHostCommand(ctx, command)
	if err != nil {
		cmd := exec.CommandContext(ctx, "sh", "-c", command)
		output, err = cmd.Output()
		if err != nil {
			result.Status = "Warning"
			result.Message = fmt.Sprintf("Failed to check CPU steal time: %v", err)
			result.Details = mapToRawExtension(details)
			return result
		}
		details["check_source"] = "container"
		result.Command = command
	} else {
		details["check_source"] = "host"
		result.Command = command
	}

	cpuLine := string(output)
	details["cpu_line"] = cpuLine

	// Parse steal time from top output (format: %steal)
	stealPercent := 0.0
	fields := strings.Fields(cpuLine)
	for i, field := range fields {
		if strings.Contains(field, "st") && i > 0 {
			stealStr := strings.Trim(fields[i-1], "%")
			if steal, err := strconv.ParseFloat(stealStr, 64); err == nil {
				stealPercent = steal
				break
			}
		}
	}

	details["steal_percent"] = stealPercent

	if stealPercent > 10.0 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("High CPU steal time: %.1f%% (may indicate resource contention in virtualized environment)", stealPercent)
	} else if stealPercent > 5.0 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("Elevated CPU steal time: %.1f%%", stealPercent)
	} else {
		result.Status = "Healthy"
		result.Message = fmt.Sprintf("CPU steal time is normal: %.1f%%", stealPercent)
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckMemoryFragmentation checks memory fragmentation
func (sc *SystemChecker) CheckMemoryFragmentation(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	command := "cat /proc/buddyinfo"
	result.Command = command
	output, err := runHostCommand(ctx, command)
	if err != nil {
		cmd := exec.CommandContext(ctx, "sh", "-c", command)
		output, err = cmd.Output()
		if err != nil {
			result.Status = "Warning"
			result.Message = fmt.Sprintf("Failed to check memory fragmentation: %v", err)
			result.Details = mapToRawExtension(details)
			return result
		}
		details["check_source"] = "container"
		result.Command = command
	} else {
		details["check_source"] = "host"
		result.Command = command
	}

	buddyInfo := strings.TrimSpace(string(output))
	details["buddyinfo"] = buddyInfo

	// Parse buddyinfo to detect fragmentation
	// High fragmentation would show many small free blocks
	lines := strings.Split(buddyInfo, "\n")
	totalFreePages := 0
	for _, line := range lines {
		if strings.Contains(line, "Normal") {
			fields := strings.Fields(line)
			// Sum free pages from order 0-10
			for i := 4; i < len(fields) && i < 15; i++ {
				if pages, err := strconv.Atoi(fields[i]); err == nil {
					totalFreePages += pages
				}
			}
		}
	}

	details["total_free_pages"] = totalFreePages

	result.Status = "Healthy"
	result.Message = "Memory fragmentation check completed"
	result.Details = mapToRawExtension(details)
	return result
}

// CheckSwapActivity checks swap activity (not just presence)
func (sc *SystemChecker) CheckSwapActivity(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	command := "vmstat 1 3"
	result.Command = command
	output, err := runHostCommand(ctx, command)
	if err != nil {
		cmd := exec.CommandContext(ctx, "vmstat", "1", "3")
		output, err = cmd.Output()
		if err != nil {
			result.Status = "Warning"
			result.Message = fmt.Sprintf("Failed to check swap activity: %v", err)
			result.Details = mapToRawExtension(details)
			return result
		}
		details["check_source"] = "container"
		result.Command = command
	} else {
		details["check_source"] = "host"
		result.Command = command
	}

	vmstatOutput := strings.TrimSpace(string(output))
	lines := strings.Split(vmstatOutput, "\n")
	
	// Get last data line
	lastLine := ""
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" && !strings.HasPrefix(line, "procs") && !strings.HasPrefix(line, " r ") {
			lastLine = line
			break
		}
	}

	if lastLine != "" {
		fields := strings.Fields(lastLine)
		if len(fields) >= 8 {
			if si, err := strconv.ParseInt(fields[6], 10, 64); err == nil {
				details["swap_in_per_sec"] = si
			}
			if so, err := strconv.ParseInt(fields[7], 10, 64); err == nil {
				details["swap_out_per_sec"] = so
			}
			
			si, _ := details["swap_in_per_sec"].(int64)
			so, _ := details["swap_out_per_sec"].(int64)
			
			if si > 100 || so > 100 {
				result.Status = "Warning"
				result.Message = fmt.Sprintf("High swap activity detected (si: %d, so: %d pages/sec)", si, so)
			} else if si > 0 || so > 0 {
				result.Status = "Warning"
				result.Message = fmt.Sprintf("Swap activity detected (si: %d, so: %d pages/sec)", si, so)
			} else {
				result.Status = "Healthy"
				result.Message = "No swap activity detected"
			}
		}
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckContextSwitches checks context switch rate
func (sc *SystemChecker) CheckContextSwitches(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	command := "vmstat 1 3"
	result.Command = command
	output, err := runHostCommand(ctx, command)
	if err != nil {
		cmd := exec.CommandContext(ctx, "vmstat", "1", "3")
		output, err = cmd.Output()
		if err != nil {
			result.Status = "Warning"
			result.Message = fmt.Sprintf("Failed to check context switches: %v", err)
			result.Details = mapToRawExtension(details)
			return result
		}
		details["check_source"] = "container"
		result.Command = command
	} else {
		details["check_source"] = "host"
		result.Command = command
	}

	vmstatOutput := strings.TrimSpace(string(output))
	lines := strings.Split(vmstatOutput, "\n")
	
	// Get last data line
	lastLine := ""
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" && !strings.HasPrefix(line, "procs") && !strings.HasPrefix(line, " r ") {
			lastLine = line
			break
		}
	}

	if lastLine != "" {
		fields := strings.Fields(lastLine)
		if len(fields) >= 11 {
			if cs, err := strconv.ParseInt(fields[10], 10, 64); err == nil {
				details["context_switches_per_sec"] = cs
				
				if cs > 100000 {
					result.Status = "Warning"
					result.Message = fmt.Sprintf("Very high context switch rate: %d/sec", cs)
				} else if cs > 50000 {
					result.Status = "Warning"
					result.Message = fmt.Sprintf("High context switch rate: %d/sec", cs)
				} else {
					result.Status = "Healthy"
					result.Message = fmt.Sprintf("Context switch rate is normal: %d/sec", cs)
				}
			}
		}
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckSELinuxStatus checks SELinux status
func (sc *SystemChecker) CheckSELinuxStatus(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	command := "getenforce 2>/dev/null"
	result.Command = command
	output, err := runHostCommand(ctx, command)
	if err != nil {
		result.Status = "Warning"
		result.Message = "SELinux status check not available (getenforce not found or not accessible)"
		details["note"] = "SELinux may not be installed or accessible"
		result.Details = mapToRawExtension(details)
		return result
	}
	result.Command = command

	status := strings.TrimSpace(string(output))
	details["status"] = status

	// Get detailed status
	sestatusCommand := "sestatus 2>/dev/null"
	configOutput, configErr := runHostCommand(ctx, sestatusCommand)
	if configErr == nil && len(configOutput) > 0 {
		details["sestatus_output"] = string(configOutput)
	}

	if status == "Enforcing" {
		result.Status = "Healthy"
		result.Message = "SELinux is enforcing"
	} else if status == "Permissive" {
		result.Status = "Warning"
		result.Message = "SELinux is in permissive mode"
	} else if status == "Disabled" {
		result.Status = "Warning"
		result.Message = "SELinux is disabled"
	} else {
		result.Status = "Unknown"
		result.Message = fmt.Sprintf("SELinux status: %s", status)
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckSSHAccess checks SSH configuration and recent access
func (sc *SystemChecker) CheckSSHAccess(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	// Check if SSH is running
	sshStatusCommand := "systemctl is-active sshd 2>/dev/null || systemctl is-active ssh 2>/dev/null"
	sshStatus, sshErr := runHostCommand(ctx, sshStatusCommand)
	if sshErr == nil {
		details["ssh_service_status"] = strings.TrimSpace(string(sshStatus))
	}

	// Check recent SSH connections
	command := "last -n 20 2>/dev/null | head -20"
	result.Command = command
	output, err := runHostCommand(ctx, command)
	if err != nil {
		// Try wtmp or utmp
		whoCommand := "who 2>/dev/null"
		result.Command = whoCommand
		output, err = runHostCommand(ctx, whoCommand)
	}

	if err == nil && len(output) > 0 {
		details["recent_ssh_connections"] = string(output)
		result.Command = result.Command
	}

	// Check SSH config file permissions (if accessible)
	configPermsCommand := "ls -l /etc/ssh/sshd_config 2>/dev/null"
	configPerms, configErr := runHostCommand(ctx, configPermsCommand)
	if configErr == nil {
		details["sshd_config_permissions"] = strings.TrimSpace(string(configPerms))
	}

	result.Status = "Healthy"
	result.Message = "SSH access check completed"
	result.Details = mapToRawExtension(details)
	return result
}

// CheckKernelModules checks loaded kernel modules
func (sc *SystemChecker) CheckKernelModules(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	command := "lsmod | head -50"
	result.Command = command
	output, err := runHostCommand(ctx, command)
	if err != nil {
		cmd := exec.CommandContext(ctx, "sh", "-c", command)
		output, err = cmd.Output()
		if err != nil {
			result.Status = "Warning"
			result.Message = fmt.Sprintf("Failed to list kernel modules: %v", err)
			result.Details = mapToRawExtension(details)
			return result
		}
		details["check_source"] = "container"
		result.Command = command
	} else {
		details["check_source"] = "host"
		result.Command = command
	}

	modulesOutput := strings.TrimSpace(string(output))
	lines := strings.Split(modulesOutput, "\n")
	moduleCount := len(lines) - 1 // Subtract header
	
	details["module_count"] = moduleCount
	sampleSize := 20
	if moduleCount < sampleSize {
		sampleSize = moduleCount
	}
	if moduleCount > 0 {
		details["modules_sample"] = lines[:sampleSize]
	}

	result.Status = "Healthy"
	result.Message = fmt.Sprintf("Found %d loaded kernel modules", moduleCount)
	result.Details = mapToRawExtension(details)
	return result
}
