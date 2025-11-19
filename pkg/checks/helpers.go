package checks

import (
	"encoding/json"
	"fmt"
	"strings"

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

