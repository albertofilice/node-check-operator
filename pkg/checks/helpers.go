package checks

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
)

// mapToRawExtension converts a map to runtime.RawExtension
func mapToRawExtension(data map[string]interface{}) runtime.RawExtension {
	if len(data) == 0 {
		return runtime.RawExtension{}
	}
	
	// Serialize nested structures as JSON strings to preserve them during Kubernetes deserialization
	// This is necessary because x-kubernetes-preserve-unknown-fields doesn't work for nested objects in arrays/maps
	dataCopy := make(map[string]interface{})
	for k, v := range data {
		// Check if this is a nested structure that might be lost during deserialization
		switch val := v.(type) {
		case []map[string]interface{}:
			// Serialize arrays of maps as JSON strings
			if jsonBytes, err := json.Marshal(val); err == nil {
				dataCopy[k] = string(jsonBytes)
			} else {
				dataCopy[k] = val
			}
		case map[string]interface{}:
			// Serialize nested maps as JSON strings
			if jsonBytes, err := json.Marshal(val); err == nil {
				dataCopy[k] = string(jsonBytes)
			} else {
				dataCopy[k] = val
			}
		case map[string]map[string]string:
			// Serialize nested maps of maps as JSON strings
			if jsonBytes, err := json.Marshal(val); err == nil {
				dataCopy[k] = string(jsonBytes)
			} else {
				dataCopy[k] = val
			}
		case map[string]map[string]float64:
			// Serialize nested maps of maps of floats as JSON strings (e.g., device_stats)
			if jsonBytes, err := json.Marshal(val); err == nil {
				dataCopy[k] = string(jsonBytes)
			} else {
				dataCopy[k] = val
			}
		case map[string]map[string]int64:
			// Serialize nested maps of maps of ints as JSON strings (e.g., interface_stats, socket_stats)
			if jsonBytes, err := json.Marshal(val); err == nil {
				dataCopy[k] = string(jsonBytes)
			} else {
				dataCopy[k] = val
			}
		case map[string]float64:
			// Serialize maps of floats as JSON strings
			if jsonBytes, err := json.Marshal(val); err == nil {
				dataCopy[k] = string(jsonBytes)
			} else {
				dataCopy[k] = val
			}
		case map[string]bool:
			// Serialize maps of bools as JSON strings (e.g., connectivity_results, dns_results)
			if jsonBytes, err := json.Marshal(val); err == nil {
				dataCopy[k] = string(jsonBytes)
			} else {
				dataCopy[k] = val
			}
		case map[string]string:
			// Serialize maps of strings as JSON strings (e.g., raid_arrays)
			if jsonBytes, err := json.Marshal(val); err == nil {
				dataCopy[k] = string(jsonBytes)
			} else {
				dataCopy[k] = val
			}
		case map[string]int:
			// Serialize maps of ints as JSON strings (e.g., socket_stats)
			if jsonBytes, err := json.Marshal(val); err == nil {
				dataCopy[k] = string(jsonBytes)
			} else {
				dataCopy[k] = val
			}
		case map[string]int64:
			// Serialize maps of int64s as JSON strings
			if jsonBytes, err := json.Marshal(val); err == nil {
				dataCopy[k] = string(jsonBytes)
			} else {
				dataCopy[k] = val
			}
		default:
			// For any other type, try to serialize as JSON if it's a complex type
			// This handles cases where the type might be interface{} but contains nested structures
			if jsonBytes, err := json.Marshal(val); err == nil {
				// Check if it's already a string (to avoid double-encoding)
				if str, ok := val.(string); ok {
					dataCopy[k] = str
				} else {
					// Try to detect if it's a nested structure by checking the JSON
					jsonStr := string(jsonBytes)
					if (strings.HasPrefix(jsonStr, "{") && strings.HasSuffix(jsonStr, "}")) ||
					   (strings.HasPrefix(jsonStr, "[") && strings.HasSuffix(jsonStr, "]")) {
						// It's a nested structure, serialize as string
						dataCopy[k] = jsonStr
					} else {
						dataCopy[k] = val
					}
				}
			} else {
				dataCopy[k] = val
			}
		}
	}
	
	jsonBytes, err := json.Marshal(dataCopy)
	if err != nil {
		errorData := map[string]interface{}{
			"error": fmt.Sprintf("failed to marshal data: %v", err),
		}
		jsonBytes, _ = json.Marshal(errorData)
	}
	return runtime.RawExtension{Raw: jsonBytes}
}

// readProcFile reads a file from /proc on the host, with fallback to container
func readProcFile(ctx context.Context, path string) ([]byte, error) {
	// Try to read from host first
	hostPath := "/host/root" + path
	if data, err := os.ReadFile(hostPath); err == nil {
		return data, nil
	}
	// Fallback to container
	return os.ReadFile(path)
}

// readLoadAvg reads load averages directly from /proc/loadavg
func readLoadAvg(ctx context.Context) (load1, load5, load15 float64, err error) {
	data, err := readProcFile(ctx, "/proc/loadavg")
	if err != nil {
		return 0, 0, 0, err
	}
	
	parts := strings.Fields(string(data))
	if len(parts) < 3 {
		return 0, 0, 0, fmt.Errorf("invalid /proc/loadavg format")
	}
	
	load1, err = strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, 0, 0, err
	}
	load5, err = strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return 0, 0, 0, err
	}
	load15, err = strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return 0, 0, 0, err
	}
	
	return load1, load5, load15, nil
}

// readMemInfo reads memory information from /proc/meminfo
func readMemInfo(ctx context.Context) (total, available, free, used, buffers, cached int64, err error) {
	data, err := readProcFile(ctx, "/proc/meminfo")
	if err != nil {
		return 0, 0, 0, 0, 0, 0, err
	}
	
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		
		key := strings.TrimSuffix(fields[0], ":")
		value, parseErr := strconv.ParseInt(fields[1], 10, 64)
		if parseErr != nil {
			continue
		}
		// Convert from KB to bytes
		value *= 1024
		
		switch key {
		case "MemTotal":
			total = value
		case "MemAvailable":
			available = value
		case "MemFree":
			free = value
		case "Buffers":
			buffers = value
		case "Cached":
			cached = value
		}
	}
	
	if total == 0 {
		return 0, 0, 0, 0, 0, 0, fmt.Errorf("could not read MemTotal")
	}
	
	// Calculate used memory
	if available > 0 {
		used = total - available
	} else {
		// Fallback calculation if MemAvailable is not available
		used = total - free - buffers - cached
	}
	
	return total, available, free, used, buffers, cached, nil
}

// getProcsBlocked reads procs_blocked from /proc/stat
func getProcsBlocked(ctx context.Context) (int, error) {
	data, err := readProcFile(ctx, "/proc/stat")
	if err != nil {
		return 0, err
	}
	
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "procs_blocked") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				blocked, err := strconv.Atoi(fields[1])
				if err != nil {
					return 0, err
				}
				return blocked, nil
			}
		}
	}
	return 0, fmt.Errorf("procs_blocked not found in /proc/stat")
}

// EventWindow tracks events in a sliding time window for debouncing
type EventWindow struct {
	mu     sync.Mutex
	events []time.Time
	window time.Duration
}

// NewEventWindow creates a new event window with the specified duration
func NewEventWindow(window time.Duration) *EventWindow {
	return &EventWindow{
		events: make([]time.Time, 0),
		window: window,
	}
}

// Add adds a new event to the window
func (w *EventWindow) Add() {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now()
	w.events = append(w.events, now)
	// Trim old events outside the window
	cutoff := now.Add(-w.window)
	i := 0
	for ; i < len(w.events) && w.events[i].Before(cutoff); i++ {
	}
	w.events = w.events[i:]
}

// Count returns the number of events in the current window
func (w *EventWindow) Count() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-w.window)
	count := 0
	for i := len(w.events) - 1; i >= 0; i-- {
		if w.events[i].After(cutoff) {
			count++
		} else {
			break
		}
	}
	return count
}

// LastEvent returns the time of the last event, or zero time if none
func (w *EventWindow) LastEvent() time.Time {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.events) == 0 {
		return time.Time{}
	}
	return w.events[len(w.events)-1]
}

// withTimeout adds a timeout to a context if not already present
func withTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

