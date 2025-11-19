package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	nodeStatusGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "nodecheck_node_status_total",
		Help: "Number of NodeCheck resources per overall node status",
	}, []string{"status"})

	totalNodeChecksGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "nodecheck_nodechecks_total",
		Help: "Total number of NodeCheck resources monitored by the operator",
	})

	checkStatusGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "nodecheck_check_status_total",
		Help: "Aggregated number of check results per category, check name and status",
	}, []string{"category", "check", "status"})

	lastStatsTimestampGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "nodecheck_stats_last_update_timestamp_seconds",
		Help: "Unix timestamp of the last dashboard stats refresh performed by the operator",
	})

	// Node-level metrics with actual values
	temperatureGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "nodecheck_temperature_celsius",
		Help: "Maximum temperature in Celsius for a node",
	}, []string{"node"})

	cpuUsageGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "nodecheck_cpu_usage_percent",
		Help: "CPU usage percentage for a node",
	}, []string{"node"})

	memoryUsageGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "nodecheck_memory_usage_percent",
		Help: "Memory usage percentage for a node",
	}, []string{"node"})

	uptimeGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "nodecheck_uptime_seconds",
		Help: "System uptime in seconds for a node",
	}, []string{"node"})

	loadAverage1mGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "nodecheck_load_average_1m",
		Help: "1-minute load average for a node",
	}, []string{"node"})

	loadAverage5mGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "nodecheck_load_average_5m",
		Help: "5-minute load average for a node",
	}, []string{"node"})

	loadAverage15mGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "nodecheck_load_average_15m",
		Help: "15-minute load average for a node",
	}, []string{"node"})
)

func init() {
	// Register metrics with controller-runtime's registry
	metrics.Registry.MustRegister(
		nodeStatusGauge,
		totalNodeChecksGauge,
		checkStatusGauge,
		lastStatsTimestampGauge,
		temperatureGauge,
		cpuUsageGauge,
		memoryUsageGauge,
		uptimeGauge,
		loadAverage1mGauge,
		loadAverage5mGauge,
		loadAverage15mGauge,
	)
}

// DashboardSnapshot represents an aggregated view of the dashboard stats that can be exported as Prometheus metrics.
type DashboardSnapshot struct {
	TotalNodeChecks int
	LastUpdate      time.Time
	NodeStatus      map[string]int
	Checks          []CheckStatusSnapshot
	Nodes           []NodeMetricsSnapshot
}

// CheckStatusSnapshot contains the counters for a single check across all nodes.
type CheckStatusSnapshot struct {
	Name    string
	Category string
	Statuses map[string]int
}

// NodeMetricsSnapshot contains actual metric values for a specific node.
type NodeMetricsSnapshot struct {
	NodeName      string
	Temperature   *float64 // Max temperature in Celsius
	CPUUsage      *float64 // CPU usage percentage
	MemoryUsage   *float64 // Memory usage percentage
	Uptime        *float64 // Uptime in seconds
	LoadAverage1m *float64
	LoadAverage5m *float64
	LoadAverage15m *float64
}

// UpdateDashboardMetrics publishes the provided snapshot to the Prometheus metrics exposed by the controller-runtime server.
func UpdateDashboardMetrics(snapshot DashboardSnapshot) {
	totalNodeChecksGauge.Set(float64(snapshot.TotalNodeChecks))

	if snapshot.LastUpdate.IsZero() {
		lastStatsTimestampGauge.Set(float64(time.Now().Unix()))
	} else {
		lastStatsTimestampGauge.Set(float64(snapshot.LastUpdate.Unix()))
	}

	nodeStatusGauge.Reset()
	for _, status := range []string{"Healthy", "Warning", "Critical", "Unknown"} {
		value := float64(snapshot.NodeStatus[status])
		nodeStatusGauge.WithLabelValues(status).Set(value)
	}

	checkStatusGauge.Reset()
	for _, check := range snapshot.Checks {
		for _, status := range []string{"Healthy", "Warning", "Critical", "Unknown"} {
			value := float64(check.Statuses[status])
			checkStatusGauge.WithLabelValues(check.Category, check.Name, status).Set(value)
		}
	}

	// Update node-level metrics
	temperatureGauge.Reset()
	cpuUsageGauge.Reset()
	memoryUsageGauge.Reset()
	uptimeGauge.Reset()
	loadAverage1mGauge.Reset()
	loadAverage5mGauge.Reset()
	loadAverage15mGauge.Reset()

	for _, node := range snapshot.Nodes {
		if node.Temperature != nil {
			temperatureGauge.WithLabelValues(node.NodeName).Set(*node.Temperature)
		}
		if node.CPUUsage != nil {
			cpuUsageGauge.WithLabelValues(node.NodeName).Set(*node.CPUUsage)
		}
		if node.MemoryUsage != nil {
			memoryUsageGauge.WithLabelValues(node.NodeName).Set(*node.MemoryUsage)
		}
		if node.Uptime != nil {
			uptimeGauge.WithLabelValues(node.NodeName).Set(*node.Uptime)
		}
		if node.LoadAverage1m != nil {
			loadAverage1mGauge.WithLabelValues(node.NodeName).Set(*node.LoadAverage1m)
		}
		if node.LoadAverage5m != nil {
			loadAverage5mGauge.WithLabelValues(node.NodeName).Set(*node.LoadAverage5m)
		}
		if node.LoadAverage15m != nil {
			loadAverage15mGauge.WithLabelValues(node.NodeName).Set(*node.LoadAverage15m)
		}
	}
}

