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

// NetworkChecker handles network monitoring
type NetworkChecker struct {
	nodeName string
}

// NewNetworkChecker creates a new network checker
func NewNetworkChecker(nodeName string) *NetworkChecker {
	return &NetworkChecker{
		nodeName: nodeName,
	}
}

// CheckInterfaces performs network interface monitoring
func (nc *NetworkChecker) CheckInterfaces(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	command := "ip a"
	result.Command = command

	output, err := runHostCommand(ctx, command)
	if err != nil {
		cmd := exec.CommandContext(ctx, "ip", "a")
		output, err = cmd.Output()
		if err != nil {
			result.Status = "Critical"
			result.Message = fmt.Sprintf("Failed to execute ip a: %v", err)
			result.Details = mapToRawExtension(details)
			return result
		}
		details["check_source"] = "container"
		result.Command = command
	} else {
		details["check_source"] = "host"
		result.Command = command
	}

	ipOutput := strings.TrimSpace(string(output))
	details["ip_output"] = ipOutput

	// Helper function to check if an interface should be ignored
	isIgnoredInterface := func(ifaceName string) bool {
		// OpenShift-specific interfaces that should be disabled
		// Includes physical interfaces used by OVS bridges (they don't have IPs directly)
		ignoredList := []string{
			"wlp88s0f0",
			"ovs-system",
			"br-int",
			"enp87s0", // Physical interface used by OVS bridge, no IP assigned
		}
		for _, ignored := range ignoredList {
			if ifaceName == ignored {
				return true
			}
		}
		
		// Ignore veth interfaces (container/pod network interfaces)
		// Pattern: hash@ifN (e.g., "90202e15cd0b96f@if2")
		if strings.Contains(ifaceName, "@if") {
			return true
		}
		
		// Ignore interfaces that start with "veth" (virtual ethernet)
		if strings.HasPrefix(ifaceName, "veth") {
			return true
		}
		
		// Ignore interfaces that are long hex hashes (typically veth pairs)
		// Pattern: 12+ character hex strings (e.g., "90202e15cd0b96f")
		// These are usually veth interfaces created by container runtimes
		if len(ifaceName) >= 12 {
			// Check if it's a hex string (only contains 0-9, a-f)
			isHex := true
			for _, char := range ifaceName {
				if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
					isHex = false
					break
				}
			}
			if isHex {
				return true
			}
		}
		
		// Ignore loopback interface (lo)
		if ifaceName == "lo" {
			return true
		}
		
		return false
	}

	// Parse interface information
	lines := strings.Split(ipOutput, "\n")
	interfaces := make(map[string]map[string]string)
	currentInterface := ""
	downInterfaces := []string{}
	noIPInterfaces := []string{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Interface line (e.g., "2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500")
		if strings.Contains(line, ":") && strings.Contains(line, "<") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				currentInterface = strings.TrimSpace(parts[1])
				
				// Skip ignored interfaces (veth, container interfaces, etc.)
				if isIgnoredInterface(currentInterface) {
					currentInterface = ""
					continue
				}
				
				interfaces[currentInterface] = make(map[string]string)
				
				// Check interface status
				if strings.Contains(line, "UP") {
					interfaces[currentInterface]["status"] = "UP"
				} else {
					interfaces[currentInterface]["status"] = "DOWN"
					downInterfaces = append(downInterfaces, currentInterface)
				}
				
				// Extract MTU
				if strings.Contains(line, "mtu") {
					mtuParts := strings.Split(line, "mtu")
					if len(mtuParts) > 1 {
						mtu := strings.Fields(mtuParts[1])[0]
						interfaces[currentInterface]["mtu"] = mtu
					}
				}
			}
		}
		
		// IP address line
		if strings.Contains(line, "inet ") && currentInterface != "" {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				ip := parts[1]
				interfaces[currentInterface]["ip"] = ip
			}
		}
	}

	// Check for interfaces without IP addresses
	for iface, info := range interfaces {
		if _, hasIP := info["ip"]; !hasIP && info["status"] == "UP" {
			noIPInterfaces = append(noIPInterfaces, iface)
		}
	}

	details["interfaces"] = interfaces
	details["down_interfaces"] = downInterfaces
	details["no_ip_interfaces"] = noIPInterfaces

	if len(downInterfaces) > 0 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("Down interfaces: %s", strings.Join(downInterfaces, ", "))
	} else if len(noIPInterfaces) > 0 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("Interfaces without IP: %s", strings.Join(noIPInterfaces, ", "))
	} else {
		result.Status = "Healthy"
		result.Message = "All network interfaces are operational"
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckRouting performs routing table monitoring
func (nc *NetworkChecker) CheckRouting(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	command := "ip route"
	result.Command = command

	output, err := runHostCommand(ctx, command)
	if err != nil {
		cmd := exec.CommandContext(ctx, "ip", "route")
		output, err = cmd.Output()
		if err != nil {
			result.Status = "Critical"
			result.Message = fmt.Sprintf("Failed to execute ip route: %v", err)
			result.Details = mapToRawExtension(details)
			return result
		}
		details["check_source"] = "container"
		result.Command = command
	} else {
		details["check_source"] = "host"
		result.Command = command
	}

	routeOutput := strings.TrimSpace(string(output))
	details["route_output"] = routeOutput

	// Parse routing table
	lines := strings.Split(routeOutput, "\n")
	routes := []string{}
	defaultRoute := ""
	gatewayReachable := true

	for _, line := range lines {
		routes = append(routes, line)
		
		// Check for default route
		if strings.Contains(line, "default") {
			defaultRoute = line
			
			// Extract gateway from default route
			fields := strings.Fields(line)
			for i, field := range fields {
				if field == "via" && i+1 < len(fields) {
					gateway := fields[i+1]
					
					// Test gateway connectivity
					if _, err := runHostCommand(ctx, fmt.Sprintf("ping -c 1 -W 2 %s", gateway)); err != nil {
						// Fallback to container ping if host ping fails
						pingCmd := exec.CommandContext(ctx, "ping", "-c", "1", "-W", "2", gateway)
						if err := pingCmd.Run(); err != nil {
							gatewayReachable = false
						}
					}
					break
				}
			}
		}
	}

	details["routes"] = routes
	details["default_route"] = defaultRoute
	details["gateway_reachable"] = gatewayReachable

	if defaultRoute == "" {
		result.Status = "Critical"
		result.Message = "No default route found"
	} else if !gatewayReachable {
		result.Status = "Warning"
		result.Message = "Default gateway is not reachable"
	} else {
		result.Status = "Healthy"
		result.Message = "Routing table is normal"
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckConnectivity performs connectivity tests
func (nc *NetworkChecker) CheckConnectivity(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	result.Command = "ping -c 1 -W 5 <target>; curl -s -L -o /dev/null --head --max-time 10 --connect-timeout 5 <target>"

	// Test connectivity to common targets
	targets := []string{
		"8.8.8.8",      // Google DNS
		"1.1.1.1",      // Cloudflare DNS
		"google.com",   // Google
		"kubernetes.io", // Kubernetes
		"redhat.com",   // Red Hat
		"quay.io",      // Quay.io registry
	}

	connectivityResults := make(map[string]bool)
	failedTargets := []string{}

	for _, target := range targets {
		reachable := false

		if _, err := runHostCommand(ctx, fmt.Sprintf("ping -c 1 -W 5 %s", target)); err == nil {
			reachable = true
		} else if _, err := runHostCommand(ctx, fmt.Sprintf("curl -s -L -o /dev/null --head --max-time 10 --connect-timeout 5 https://%s", target)); err == nil {
			reachable = true
		} else if _, err := runHostCommand(ctx, fmt.Sprintf("curl -s -L -o /dev/null --head --max-time 10 --connect-timeout 5 http://%s", target)); err == nil {
			reachable = true
		} else {
			// Fallback to container commands if host access fails entirely
			pingCmd := exec.CommandContext(ctx, "ping", "-c", "1", "-W", "5", target)
			if pingCmd.Run() == nil {
				reachable = true
			} else {
				curlCmd := exec.CommandContext(ctx, "curl", "-s", "-L", "-o", "/dev/null", "--head",
					"--max-time", "10", "--connect-timeout", "5",
					fmt.Sprintf("https://%s", target))
				if curlCmd.Run() == nil {
					reachable = true
				} else {
					curlHttpCmd := exec.CommandContext(ctx, "curl", "-s", "-L", "-o", "/dev/null", "--head",
						"--max-time", "10", "--connect-timeout", "5",
						fmt.Sprintf("http://%s", target))
					if curlHttpCmd.Run() == nil {
						reachable = true
					}
				}
			}
		}

		connectivityResults[target] = reachable

		if !reachable {
			failedTargets = append(failedTargets, target)
		}
	}

	details["connectivity_results"] = connectivityResults
	details["failed_targets"] = failedTargets

	// Test DNS resolution
	dnsResults := make(map[string]bool)
	dnsTargets := []string{"google.com", "kubernetes.io", "github.com"}

	for _, target := range dnsTargets {
		if _, err := runHostCommand(ctx, fmt.Sprintf("getent hosts %s", target)); err == nil {
			dnsResults[target] = true
		} else if _, err := exec.CommandContext(ctx, "getent", "hosts", target).Output(); err == nil {
			dnsResults[target] = true
		} else {
			dnsResults[target] = false
		}
	}

	details["dns_results"] = dnsResults

	if len(failedTargets) > 2 {
		result.Status = "Critical"
		result.Message = fmt.Sprintf("Multiple connectivity failures: %s", strings.Join(failedTargets, ", "))
	} else if len(failedTargets) > 0 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("Some connectivity issues: %s", strings.Join(failedTargets, ", "))
	} else {
		result.Status = "Healthy"
		result.Message = "Network connectivity is normal"
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckStatistics performs network statistics monitoring
func (nc *NetworkChecker) CheckStatistics(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	command := "ss -s"
	result.Command = command

	output, err := runHostCommand(ctx, command)
	if err != nil {
		cmd := exec.CommandContext(ctx, "ss", "-s")
		output, err = cmd.Output()
		if err != nil {
			result.Status = "Warning"
			result.Message = fmt.Sprintf("Failed to execute ss -s: %v", err)
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

	ssOutput := strings.TrimSpace(string(output))
	details["ss_output"] = ssOutput

	// Parse socket statistics
	lines := strings.Split(ssOutput, "\n")
	socketStats := make(map[string]int)
	
	for _, line := range lines {
		if strings.Contains(line, "Total:") {
			// Parse total connections
			parts := strings.Fields(line)
			for i, part := range parts {
				if part == "Total:" && i+1 < len(parts) {
					if total, err := strconv.Atoi(parts[i+1]); err == nil {
						socketStats["total_connections"] = total
					}
				}
			}
		}
	}

	// Get interface statistics using ethtool if available
	interfaces := []string{"eth0", "ens33", "enp0s3"} // Common interface names
	interfaceStats := make(map[string]map[string]int64)
	
	for _, iface := range interfaces {
		statsOutput, err := runHostCommand(ctx, fmt.Sprintf("ethtool -S %s", iface))
		if err != nil {
			cmd := exec.CommandContext(ctx, "ethtool", "-S", iface)
			statsOutput, err = cmd.Output()
			if err != nil {
				continue // Interface might not exist
			}
		}

		stats := make(map[string]int64)
		lines := strings.Split(strings.TrimSpace(string(statsOutput)), "\n")
		
		for _, line := range lines {
			if strings.Contains(line, ":") {
				parts := strings.Split(line, ":")
				if len(parts) >= 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])
					if val, err := strconv.ParseInt(value, 10, 64); err == nil {
						stats[key] = val
					}
				}
			}
		}
		
		if len(stats) > 0 {
			interfaceStats[iface] = stats
		}
	}

	details["socket_stats"] = socketStats
	details["interface_stats"] = interfaceStats

	// Check for high error rates
	highErrorInterfaces := []string{}
	for iface, stats := range interfaceStats {
		if rxErrors, ok := stats["rx_errors"]; ok && rxErrors > 100 {
			highErrorInterfaces = append(highErrorInterfaces, fmt.Sprintf("%s: %d RX errors", iface, rxErrors))
		}
		if txErrors, ok := stats["tx_errors"]; ok && txErrors > 100 {
			highErrorInterfaces = append(highErrorInterfaces, fmt.Sprintf("%s: %d TX errors", iface, txErrors))
		}
	}

	details["high_error_interfaces"] = highErrorInterfaces

	if len(highErrorInterfaces) > 0 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("High network errors: %s", strings.Join(highErrorInterfaces, ", "))
	} else {
		result.Status = "Healthy"
		result.Message = "Network statistics are normal"
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckErrors checks network interface errors in detail
func (nc *NetworkChecker) CheckErrors(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	command := "ip -s link show"
	result.Command = command

	output, err := runHostCommand(ctx, command)
	if err != nil {
		cmd := exec.CommandContext(ctx, "sh", "-c", "ip -s link show")
		output, err = cmd.Output()
		if err != nil {
			result.Status = "Warning"
			result.Message = fmt.Sprintf("Failed to check network errors: %v", err)
			result.Details = mapToRawExtension(details)
			return result
		}
		details["check_source"] = "container"
		result.Command = command
	} else {
		details["check_source"] = "host"
		result.Command = command
	}

	ipOutput := strings.TrimSpace(string(output))
	details["ip_output"] = ipOutput

	// Parse errors from ip output
	lines := strings.Split(ipOutput, "\n")
	interfaceErrors := make(map[string]map[string]int64)
	currentInterface := ""
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, ":") {
			// New interface
			fields := strings.Fields(line)
			if len(fields) > 1 {
				currentInterface = strings.TrimSuffix(fields[1], ":")
				interfaceErrors[currentInterface] = make(map[string]int64)
			}
		} else if currentInterface != "" {
			if strings.Contains(line, "RX:") {
				// Parse RX errors
				fields := strings.Fields(line)
				for i, field := range fields {
					if field == "errors" && i+1 < len(fields) {
						if errCount, err := strconv.ParseInt(fields[i+1], 10, 64); err == nil {
							interfaceErrors[currentInterface]["rx_errors"] = errCount
						}
					}
					if field == "dropped" && i+1 < len(fields) {
						if dropCount, err := strconv.ParseInt(fields[i+1], 10, 64); err == nil {
							interfaceErrors[currentInterface]["rx_dropped"] = dropCount
						}
					}
				}
			} else if strings.Contains(line, "TX:") {
				// Parse TX errors
				fields := strings.Fields(line)
				for i, field := range fields {
					if field == "errors" && i+1 < len(fields) {
						if errCount, err := strconv.ParseInt(fields[i+1], 10, 64); err == nil {
							interfaceErrors[currentInterface]["tx_errors"] = errCount
						}
					}
					if field == "dropped" && i+1 < len(fields) {
						if dropCount, err := strconv.ParseInt(fields[i+1], 10, 64); err == nil {
							interfaceErrors[currentInterface]["tx_dropped"] = dropCount
						}
					}
				}
			}
		}
	}

	details["interface_errors"] = interfaceErrors

	// Check for high error rates
	highErrorInterfaces := []string{}
	for iface, errors := range interfaceErrors {
		totalErrors := int64(0)
		for _, count := range errors {
			totalErrors += count
		}
		if totalErrors > 1000 {
			highErrorInterfaces = append(highErrorInterfaces, fmt.Sprintf("%s: %d errors", iface, totalErrors))
		}
	}

	details["high_error_interfaces"] = highErrorInterfaces

	if len(highErrorInterfaces) > 0 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("High network errors detected on %d interfaces", len(highErrorInterfaces))
	} else {
		result.Status = "Healthy"
		result.Message = "Network error rates are normal"
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckLatency checks network latency to gateway/DNS
func (nc *NetworkChecker) CheckLatency(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	result.Command = "ip route | grep default; ping -c 3 -W 1 <gateway>; ping -c 3 -W 1 <dns>"

	// Get default gateway
	gatewayOutput, gatewayErr := runHostCommand(ctx, "ip route | grep default | head -1 | awk '{print $3}'")
	gateway := ""
	if gatewayErr == nil {
		gateway = strings.TrimSpace(string(gatewayOutput))
	}

	// Test latency to gateway
	if gateway != "" {
		pingOutput, pingErr := runHostCommand(ctx, fmt.Sprintf("ping -c 3 -W 1 %s 2>&1", gateway))
		if pingErr == nil {
			pingStr := string(pingOutput)
			details["gateway_ping"] = pingStr
			
			// Parse average latency
			if strings.Contains(pingStr, "avg") {
				lines := strings.Split(pingStr, "\n")
				for _, line := range lines {
					if strings.Contains(line, "avg") {
						details["gateway_latency"] = line
						break
					}
				}
			}
		}
		details["gateway"] = gateway
	}

	// Test latency to DNS (8.8.8.8 or first DNS server)
	dnsServers := []string{"8.8.8.8", "1.1.1.1"}
	resolvOutput, resolvErr := runHostCommand(ctx, "grep nameserver /etc/resolv.conf | head -1 | awk '{print $2}'")
	if resolvErr == nil {
		dnsServer := strings.TrimSpace(string(resolvOutput))
		if dnsServer != "" {
			dnsServers = append([]string{dnsServer}, dnsServers...)
		}
	}

	for _, dns := range dnsServers[:1] { // Test first DNS only
		pingOutput, pingErr := runHostCommand(ctx, fmt.Sprintf("ping -c 3 -W 1 %s 2>&1", dns))
		if pingErr == nil {
			pingStr := string(pingOutput)
			details[fmt.Sprintf("dns_%s_ping", dns)] = pingStr
			
			if strings.Contains(pingStr, "avg") {
				lines := strings.Split(pingStr, "\n")
				for _, line := range lines {
					if strings.Contains(line, "avg") {
						details[fmt.Sprintf("dns_%s_latency", dns)] = line
						break
					}
				}
			}
		}
	}

	result.Status = "Healthy"
	result.Message = "Network latency check completed"
	result.Details = mapToRawExtension(details)
	return result
}

// CheckDNSResolution checks DNS resolution
func (nc *NetworkChecker) CheckDNSResolution(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	result.Command = "getent hosts <domain> || nslookup <domain>"

	// Test DNS resolution
	// Note: kubernetes.default.svc.cluster.local is only resolvable from pod network, not host network
	// So we only test external DNS resolution from host
	testDomains := []string{"google.com", "8.8.8.8"}
	dnsResults := make(map[string]bool)
	
	for _, domain := range testDomains {
		output, err := runHostCommand(ctx, fmt.Sprintf("getent hosts %s 2>&1 || nslookup %s 2>&1 | head -5", domain, domain))
		if err != nil {
			cmd := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("getent hosts %s 2>&1 || nslookup %s 2>&1 | head -5", domain, domain))
			output, err = cmd.Output()
		}
		
		resolved := err == nil && len(output) > 0 && !strings.Contains(string(output), "not found") && !strings.Contains(string(output), "NXDOMAIN")
		dnsResults[domain] = resolved
	}

	details["dns_results"] = dnsResults
	details["note"] = "kubernetes.default.svc.cluster.local is only resolvable from pod network, not host network, so it's not tested here"

	failedDomains := []string{}
	for domain, resolved := range dnsResults {
		if !resolved {
			failedDomains = append(failedDomains, domain)
		}
	}

	if len(failedDomains) > 0 {
		result.Status = "Warning"
		result.Message = fmt.Sprintf("DNS resolution failed for: %s", strings.Join(failedDomains, ", "))
	} else {
		result.Status = "Healthy"
		result.Message = "DNS resolution is working"
	}

	result.Details = mapToRawExtension(details)
	return result
}

// CheckBondingStatus checks network bonding status
func (nc *NetworkChecker) CheckBondingStatus(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	// Check for bonding interfaces
	command := "ls /sys/class/net/ | grep bond"
	result.Command = command

	output, err := runHostCommand(ctx, command)
	if err != nil {
		result.Status = "Healthy"
		result.Message = "No bonding interfaces found"
		details["note"] = "No bonding interfaces detected (this is normal if bonding is not configured)"
		result.Details = mapToRawExtension(details)
		return result
	}

	bondInterfaces := strings.Fields(strings.TrimSpace(string(output)))
	if len(bondInterfaces) == 0 {
		result.Status = "Healthy"
		result.Message = "No bonding interfaces found"
		details["note"] = "No bonding interfaces detected"
		result.Details = mapToRawExtension(details)
		return result
	}

	details["bond_interfaces"] = bondInterfaces
	bondStatus := make(map[string]interface{})

	for _, bond := range bondInterfaces {
		// Check bonding status
		bondInfo, bondErr := runHostCommand(ctx, fmt.Sprintf("cat /proc/net/bonding/%s 2>/dev/null", bond))
		if bondErr == nil && len(bondInfo) > 0 {
			bondStatus[bond] = string(bondInfo)
		}
	}

	details["bond_status"] = bondStatus

	result.Status = "Healthy"
	result.Message = fmt.Sprintf("Found %d bonding interface(s)", len(bondInterfaces))
	result.Details = mapToRawExtension(details)
	return result
}

// CheckFirewallRules checks firewall rules (iptables/ipset)
func (nc *NetworkChecker) CheckFirewallRules(ctx context.Context) *v1alpha1.CheckResult {
	details := make(map[string]interface{})
	result := &v1alpha1.CheckResult{
		Timestamp: metav1.Now(),
		Status:    "Unknown",
	}

	result.Command = "iptables -L -n; iptables rule count via wc -l"

	// Check iptables rules count
	iptablesOutput, iptablesErr := runHostCommand(ctx, "iptables -L -n 2>/dev/null | wc -l")
	if iptablesErr == nil {
		if ruleCount, err := strconv.Atoi(strings.TrimSpace(string(iptablesOutput))); err == nil {
			details["iptables_rule_count"] = ruleCount
		}
	}

	// Check if firewall is active
	firewallActive := false
	if iptablesErr == nil {
		firewallActive = true
	}

	details["firewall_active"] = firewallActive

	result.Status = "Healthy"
	result.Message = "Firewall rules check completed"
	result.Details = mapToRawExtension(details)
	return result
}
