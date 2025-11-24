package checks

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/albertofilice/node-check-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HardwareChecker handles hardware monitoring
type HardwareChecker struct {
	nodeName string
}

// NewHardwareChecker creates a new hardware checker
func NewHardwareChecker(nodeName string) *HardwareChecker {
	return &HardwareChecker{
		nodeName: nodeName,
	}
}

// CheckTemperature performs temperature monitoring
func (hc *HardwareChecker) CheckTemperature(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	command := "sensors"
	result.Command = command
	output, err := runHostCommand(ctx, command)
	if err != nil {
		cmd := exec.CommandContext(ctx, command)
		output, err = cmd.Output()
		if err != nil {
			// sensors might not be available, try alternative approach
			result.Status = "Warning"
			result.Message = "Temperature monitoring not available (sensors command not found)"
			details["error"] = err.Error()
			result.Details = mapToRawExtension(details)
			return result
		}
		details["check_source"] = "container"
		result.Command = command
	} else {
		details["check_source"] = "host"
		result.Command = command
	}

	sensorsOutput := strings.TrimSpace(string(output))
	details["sensors_output"] = sensorsOutput

	// Parse temperature readings
	lines := strings.Split(sensorsOutput, "\n")
	temperatures := make(map[string]float64)
	
	for _, line := range lines {
		if strings.Contains(line, "°C") {
			// Extract temperature values
			parts := strings.Fields(line)
			for i, part := range parts {
				if strings.Contains(part, "°C") {
					tempStr := strings.Trim(part, "°C")
					if temp, err := strconv.ParseFloat(tempStr, 64); err == nil {
						// Get the sensor name (usually the part before the temperature)
						sensorName := ""
						if i > 0 {
							sensorName = parts[i-1]
						}
						temperatures[sensorName] = temp
					}
				}
			}
		}
	}

	details["temperatures"] = temperatures

	// Check for high temperatures
	maxTemp := 0.0
	highTempSensors := []string{}
	
	for sensor, temp := range temperatures {
		if temp > 80.0 {
			highTempSensors = append(highTempSensors, fmt.Sprintf("%s: %.1f°C", sensor, temp))
		}
		if temp > maxTemp {
			maxTemp = temp
		}
	}

	if len(highTempSensors) > 0 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("High temperatures detected: %s", strings.Join(highTempSensors, ", "))
	} else if maxTemp > 0 {
		result.Status = "Healthy"
		result.Message = fmt.Sprintf("Temperatures are normal (max: %.1f°C)", maxTemp)
	} else {
		result.Status = "Warning"
		result.Message = "No temperature readings available"
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckIPMI performs IPMI monitoring
func (hc *HardwareChecker) CheckIPMI(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	command := "ipmitool sdr elist"
	result.Command = command

	output, err := runHostCommand(ctx, command)
	if err != nil || len(output) == 0 {
		cmd := exec.CommandContext(ctx, "ipmitool", "sdr", "elist")
		output, err = cmd.CombinedOutput()
		if err != nil {
			commandOutput := strings.TrimSpace(string(output))
			if commandOutput != "" {
				details["command_output"] = commandOutput
			}
			message := "IPMI monitoring not available (ipmitool may need access to /dev/ipmi* devices or IPMI hardware not present)"
			status := "Warning"
			if strings.Contains(strings.ToLower(commandOutput), "could not open device") ||
				strings.Contains(strings.ToLower(commandOutput), "no such file or directory") {
				message = "IPMI hardware not detected on this node"
				status = "Unknown"
				details["note"] = "Node appears to lack /dev/ipmi* devices; IPMI not supported."
			} else {
				details["note"] = "Ensure /dev/ipmi* devices are present on the host and mounted into the executor pod."
			}
			result.Status = status
			result.Message = message
			details["error"] = err.Error()
			result.Details = mapToRawExtension(details)
			return result
		}
		details["check_source"] = "container"
		result.Command = command
	} else {
		details["check_source"] = "host"
		result.Command = command
	}

	ipmiOutput := strings.TrimSpace(string(output))
	details["ipmi_output"] = ipmiOutput

	// Parse IPMI sensor data
	lines := strings.Split(ipmiOutput, "\n")
	sensors := make(map[string]string)
	criticalSensors := []string{}
	warningSensors := []string{}

	for _, line := range lines {
		if strings.Contains(line, "|") {
			parts := strings.Split(line, "|")
			if len(parts) >= 3 {
				sensorName := strings.TrimSpace(parts[0])
				status := strings.TrimSpace(parts[1])
				value := strings.TrimSpace(parts[2])

				sensors[sensorName] = value

				if strings.Contains(status, "Critical") {
					criticalSensors = append(criticalSensors, fmt.Sprintf("%s: %s", sensorName, value))
				} else if strings.Contains(status, "Warning") {
					warningSensors = append(warningSensors, fmt.Sprintf("%s: %s", sensorName, value))
				}
			}
		}
	}

	details["sensors"] = sensors
	details["critical_sensors"] = criticalSensors
	details["warning_sensors"] = warningSensors

	if len(criticalSensors) > 0 {
		result.Status = "Critical"
		result.Message = fmt.Sprintf("Critical IPMI sensors: %s", strings.Join(criticalSensors, ", "))
	} else if len(warningSensors) > 0 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("Warning IPMI sensors: %s", strings.Join(warningSensors, ", "))
	} else {
		result.Status = "Healthy"
		result.Message = "All IPMI sensors are normal"
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckBMC performs BMC monitoring
func (hc *HardwareChecker) CheckBMC(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	command := "ipmitool chassis status"
	result.Command = command

	output, err := runHostCommand(ctx, command)
	if err != nil || len(output) == 0 {
		cmd := exec.CommandContext(ctx, "ipmitool", "chassis", "status")
		output, err = cmd.CombinedOutput()
		if err != nil {
			commandOutput := strings.TrimSpace(string(output))
			if commandOutput != "" {
				details["command_output"] = commandOutput
			}
			message := "BMC monitoring not available (ipmitool may need access to /dev/ipmi* devices or BMC hardware not present)"
			status := "Warning"
			if strings.Contains(strings.ToLower(commandOutput), "could not open device") ||
				strings.Contains(strings.ToLower(commandOutput), "no such file or directory") {
				message = "BMC hardware not detected on this node"
				status = "Unknown"
				details["note"] = "Node appears to lack /dev/ipmi* devices; BMC not supported."
			} else {
				details["note"] = "Ensure /dev/ipmi* devices are present on the host and mounted into the executor pod."
			}
			result.Status = status
			result.Message = message
			details["error"] = err.Error()
			result.Details = mapToRawExtension(details)
			return result
		}
		details["check_source"] = "container"
		result.Command = command
	} else {
		details["check_source"] = "host"
		result.Command = command
	}

	bmcOutput := strings.TrimSpace(string(output))
	details["bmc_output"] = bmcOutput

	// Parse BMC status
	lines := strings.Split(bmcOutput, "\n")
	chassisStatus := make(map[string]string)
	
	for _, line := range lines {
		if strings.Contains(line, ":") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				chassisStatus[key] = value
			}
		}
	}

	details["chassis_status"] = chassisStatus

	// Check for critical status
	criticalStatus := []string{}
	warningStatus := []string{}

	for key, value := range chassisStatus {
		if strings.Contains(strings.ToLower(value), "off") && key != "System Power" {
			criticalStatus = append(criticalStatus, fmt.Sprintf("%s: %s", key, value))
		} else if strings.Contains(strings.ToLower(value), "warning") {
			warningStatus = append(warningStatus, fmt.Sprintf("%s: %s", key, value))
		}
	}

	if len(criticalStatus) > 0 {
		result.Status = "Critical"
		result.Message = fmt.Sprintf("Critical BMC status: %s", strings.Join(criticalStatus, ", "))
	} else if len(warningStatus) > 0 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("Warning BMC status: %s", strings.Join(warningStatus, ", "))
	} else {
		result.Status = "Healthy"
		result.Message = "BMC status is normal"
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckFanStatus checks fan status via IPMI
func (hc *HardwareChecker) CheckFanStatus(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	command := "ipmitool sdr type fan 2>/dev/null"
	result.Command = command

	output, err := runHostCommand(ctx, command)
	if err != nil {
		result.Status = "Unknown"
		result.Message = "Fan status check not available (ipmitool may need access to /dev/ipmi* devices or IPMI hardware not present)"
		details["note"] = "Fan monitoring requires IPMI access. If hardware supports IPMI, ensure ipmitool has access to /dev/ipmi* devices."
		result.Details = mapToRawExtension(details)
		return result
	}
	result.Command = command

	fanOutput := strings.TrimSpace(string(output))
	details["fan_output"] = fanOutput

	lines := strings.Split(fanOutput, "\n")
	fanStatus := []string{}
	criticalFans := []string{}
	warningFans := []string{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fanStatus = append(fanStatus, line)
		
		// Check for critical/warning status
		if strings.Contains(strings.ToLower(line), "critical") || strings.Contains(strings.ToLower(line), "failed") {
			criticalFans = append(criticalFans, line)
		} else if strings.Contains(strings.ToLower(line), "warning") || strings.Contains(strings.ToLower(line), "ns") {
			warningFans = append(warningFans, line)
		}
	}

	details["fan_count"] = len(fanStatus)
	details["fan_status"] = fanStatus

	if len(criticalFans) > 0 {
		result.Status = "Critical"
		result.Message = fmt.Sprintf("Critical fan issues detected: %d fans", len(criticalFans))
	} else if len(warningFans) > 0 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("Warning fan issues detected: %d fans", len(warningFans))
	} else {
		result.Status = "Healthy"
		result.Message = fmt.Sprintf("All fans are operating normally (%d fans)", len(fanStatus))
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckPowerSupply checks power supply status via IPMI
func (hc *HardwareChecker) CheckPowerSupply(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	command := "ipmitool sdr type 'Power Supply' 2>/dev/null"
	result.Command = command

	output, err := runHostCommand(ctx, command)
	if err != nil {
		result.Status = "Unknown"
		result.Message = "Power supply check not available (ipmitool may need access to /dev/ipmi* devices or IPMI hardware not present)"
		details["note"] = "Power supply monitoring requires IPMI access. If hardware supports IPMI, ensure ipmitool has access to /dev/ipmi* devices."
		result.Details = mapToRawExtension(details)
		return result
	}
	result.Command = command

	psOutput := strings.TrimSpace(string(output))
	details["power_supply_output"] = psOutput

	lines := strings.Split(psOutput, "\n")
	psStatus := []string{}
	criticalPS := []string{}
	warningPS := []string{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		psStatus = append(psStatus, line)
		
		if strings.Contains(strings.ToLower(line), "critical") || strings.Contains(strings.ToLower(line), "failed") {
			criticalPS = append(criticalPS, line)
		} else if strings.Contains(strings.ToLower(line), "warning") || strings.Contains(strings.ToLower(line), "ns") {
			warningPS = append(warningPS, line)
		}
	}

	details["power_supply_count"] = len(psStatus)
	details["power_supply_status"] = psStatus

	if len(criticalPS) > 0 {
		result.Status = "Critical"
		result.Message = fmt.Sprintf("Critical power supply issues detected: %d power supplies", len(criticalPS))
	} else if len(warningPS) > 0 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("Warning power supply issues detected: %d power supplies", len(warningPS))
	} else {
		result.Status = "Healthy"
		result.Message = fmt.Sprintf("All power supplies are operating normally (%d power supplies)", len(psStatus))
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckMemoryErrors checks for memory errors (ECC errors)
func (hc *HardwareChecker) CheckMemoryErrors(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	// Use more specific regex to avoid false positives
	// Match EDAC errors, MCE (Machine Check Exception), or actual memory errors
	// Exclude initialization messages and interface names
	// Exclude: "Giving out device", "Ver:", version numbers, initialization messages, "HANDLING IBECC" during boot
	command := "dmesg | grep -iE '\\b(EDAC|MCE|memory error|ecc error)' | grep -vE '(macvtap|tun|bridge|@if|veth|interface|Giving out device|Ver:|^\\[\\s*[0-9]+\\.[0-9]+\\]\\s*EDAC.*Ver|^\\[\\s*[0-9]+\\.[0-9]+\\]\\s*EDAC.*v[0-9]|^\\[\\s*[0-9]+\\.[0-9]+\\]\\s*EDAC.*Giving out)' | tail -50"
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

	memErrorOutput := strings.TrimSpace(string(output))
	details["memory_error_output"] = memErrorOutput

	// Also check journalctl with more specific pattern
	// Only match actual errors (UE - Uncorrected Errors, Hardware Error), not initialization
	journalOutput, journalErr := runHostCommand(ctx, "journalctl -k -p err --since '1 hour ago' --no-pager 2>/dev/null | grep -iE '\\b(EDAC.*\\b(UE|Uncorrected|Hardware Error)|MCE:\\s*\\[Hardware Error\\]|memory.*error.*uncorrected)' | grep -vE '(macvtap|tun|bridge|@if|veth|interface|Giving out device)' | tail -50")
	if journalErr == nil && len(journalOutput) > 0 {
		journalLines := strings.TrimSpace(string(journalOutput))
		if memErrorOutput != "" {
			memErrorOutput = memErrorOutput + "\n" + journalLines
		} else {
			memErrorOutput = journalLines
		}
		details["journal_memory_error_output"] = journalLines
	}

	// Check /sys/devices/system/edac for EDAC errors if available
	edacOutput, edacErr := runHostCommand(ctx, "find /sys/devices/system/edac -name '*_ce_count' -o -name '*_ue_count' 2>/dev/null | head -10 | xargs cat 2>/dev/null")
	if edacErr == nil && len(edacOutput) > 0 {
		details["edac_error_counts"] = string(edacOutput)
	}

	// Parse and filter errors more carefully
	realErrors := []string{}
	correctedErrors := []string{}
	uncorrectedErrors := []string{}
	
	if memErrorOutput != "" {
		lines := strings.Split(memErrorOutput, "\n")
		seen := make(map[string]bool)
		
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || seen[line] {
				continue
			}
			
			// Skip initialization messages
			lower := strings.ToLower(line)
			if strings.Contains(lower, "giving out device") ||
				strings.Contains(lower, "ver:") ||
				strings.Contains(lower, "version") ||
				strings.Contains(lower, "controller") && strings.Contains(lower, "interrupt") {
				// Skip initialization messages
				continue
			}
			
			// Check timestamp - skip messages from first 10 seconds (initialization)
			if strings.HasPrefix(line, "[") {
				// Extract timestamp from dmesg format: [seconds.microseconds]
				parts := strings.SplitN(line, "]", 2)
				if len(parts) == 2 {
					timestampStr := strings.TrimPrefix(parts[0], "[")
					if timestamp, parseErr := strconv.ParseFloat(timestampStr, 64); parseErr == nil {
						if timestamp < 10.0 {
							// Skip initialization messages (first 10 seconds)
							// But allow "HANDLING IBECC MEMORY ERROR" if it's not initialization
							if !strings.Contains(lower, "handling ibecc memory error") {
								continue
							}
						}
					}
				}
			}
			
			// Skip "HANDLING IBECC MEMORY ERROR" during initialization (these are informational)
			// Only count if it's a real error (UE - Uncorrected Error)
			if strings.Contains(lower, "handling ibecc memory error") {
				// This is just informational during initialization, skip it
				continue
			}
			
			seen[line] = true
			
			// Categorize errors
			if strings.Contains(lower, "uncorrected") || strings.Contains(lower, "\\bue\\b") || strings.Contains(lower, "hardware error") {
				uncorrectedErrors = append(uncorrectedErrors, line)
				realErrors = append(realErrors, line)
			} else if strings.Contains(lower, "corrected") || strings.Contains(lower, "\\bce\\b") {
				correctedErrors = append(correctedErrors, line)
				// CE (Corrected Errors) are less critical, but still worth noting
			} else if strings.Contains(lower, "mce") || strings.Contains(lower, "machine check") {
				// MCE errors are always serious
				realErrors = append(realErrors, line)
			} else {
				// Other memory errors - include them
				realErrors = append(realErrors, line)
			}
		}
	}

	details["error_count"] = len(realErrors)
	details["corrected_errors"] = len(correctedErrors)
	details["uncorrected_errors"] = len(uncorrectedErrors)
	if len(correctedErrors) > 0 {
		sampleSize := 5
		if len(correctedErrors) < sampleSize {
			sampleSize = len(correctedErrors)
		}
		details["corrected_error_samples"] = correctedErrors[:sampleSize]
	}
	if len(uncorrectedErrors) > 0 {
		sampleSize := 5
		if len(uncorrectedErrors) < sampleSize {
			sampleSize = len(uncorrectedErrors)
		}
		details["uncorrected_error_samples"] = uncorrectedErrors[:sampleSize]
	}

	// Determine status: Uncorrected Errors are Critical, Corrected Errors are Warning
	if len(uncorrectedErrors) > 0 {
		result.Status = "Critical"
		result.Message = fmt.Sprintf("Found %d uncorrected memory errors (UE) - %d total memory error events", len(uncorrectedErrors), len(realErrors))
	} else if len(correctedErrors) > 0 && len(realErrors) == 0 {
		// Only corrected errors, no real errors
		result.Status = "Warning"
		result.Message = fmt.Sprintf("Found %d corrected memory errors (CE) - these are handled automatically", len(correctedErrors))
	} else if len(realErrors) > 0 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("Found %d memory error events (ECC/MCE/EDAC)", len(realErrors))
	} else {
		result.Status = "Healthy"
		result.Message = "No memory errors detected"
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckPCIeErrors checks for PCIe errors
func (hc *HardwareChecker) CheckPCIeErrors(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	// Check dmesg for PCIe errors
	command := "dmesg | grep -i 'pci.*error\\|pcie.*error\\|aer.*error' | tail -50"
	result.Command = command

	output, err := runHostCommand(ctx, command)
	if err != nil {
		cmd := exec.CommandContext(ctx, "sh", "-c", "dmesg | grep -i 'pci.*error\\|pcie.*error\\|aer.*error' | tail -50")
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

	pcieErrorOutput := strings.TrimSpace(string(output))
	details["pcie_error_output"] = pcieErrorOutput

	// Check /sys/devices for AER (Advanced Error Reporting) errors
	aerOutput, aerErr := runHostCommand(ctx, "find /sys/devices -name 'aer_dev_fatal' -o -name 'aer_dev_nonfatal' 2>/dev/null | head -10 | xargs cat 2>/dev/null")
	if aerErr == nil && len(aerOutput) > 0 {
		details["aer_error_counts"] = string(aerOutput)
	}

	errorCount := 0
	if pcieErrorOutput != "" {
		errorCount = len(strings.Split(pcieErrorOutput, "\n"))
	}

	details["error_count"] = errorCount

	if errorCount > 0 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("Found %d PCIe error events", errorCount)
	} else {
		result.Status = "Healthy"
		result.Message = "No PCIe errors detected"
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckCPUMicrocode checks CPU microcode version
func (hc *HardwareChecker) CheckCPUMicrocode(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	// Check microcode version from /proc/cpuinfo
	command := "grep -m1 microcode /proc/cpuinfo"
	result.Command = command

	output, err := runHostCommand(ctx, command)
	if err != nil {
		cmd := exec.CommandContext(ctx, "sh", "-c", "grep -m1 microcode /proc/cpuinfo")
		output, err = cmd.Output()
		if err != nil {
			result.Status = "Warning"
			result.Message = "CPU microcode information not available"
			details["note"] = "Microcode information may not be available in /proc/cpuinfo"
			result.Details = mapToRawExtension(details)
			return result
		}
		details["check_source"] = "container"
		result.Command = command
	} else {
		details["check_source"] = "host"
		result.Command = command
	}

	microcodeLine := strings.TrimSpace(string(output))
	details["microcode_info"] = microcodeLine

	// Extract microcode version
	if strings.Contains(microcodeLine, "microcode") {
		fields := strings.Fields(microcodeLine)
		for i, field := range fields {
			if field == "microcode" && i+1 < len(fields) {
				details["microcode_version"] = fields[i+1]
				break
			}
		}
	}

	// Check dmesg for microcode update messages
	dmesgOutput, dmesgErr := runHostCommand(ctx, "dmesg | grep -i microcode | tail -10")
	if dmesgErr == nil && len(dmesgOutput) > 0 {
		details["microcode_dmesg"] = string(dmesgOutput)
	}

	result.Status = "Healthy"
	result.Message = "CPU microcode check completed"
	result.Details = mapToRawExtension(details)
	return result
}
