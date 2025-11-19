# Node Check Operator

A Kubernetes/OpenShift operator for comprehensive bare metal node monitoring. It performs in-depth checks at both the operating system and cluster levels, providing complete visibility into node health status.

## What does this operator do?

The operator monitors cluster nodes by running a series of automatic checks that verify:

- **Operating system status**: uptime, processes, memory, CPU, disks, network, hardware
- **Kubernetes/OpenShift status**: node conditions, pods, services, resource quotas, cluster operators

Results are exposed through:
- **OpenShift Console Plugin**: integrated interface in the standard console
- **Standalone web dashboard**: accessible via browser
- **Prometheus metrics**: for integration with existing monitoring systems

## Main Features

### OpenShift Console Integration

The plugin integrates natively into the OpenShift console, allowing you to view node status directly from the standard interface. No additional configuration is required: once the operator is installed, the plugin is automatically available in the Monitoring section.

### Web Dashboard

A standalone web dashboard provides a detailed view of all monitored nodes, with real-time statistics, historical charts, and complete details of each check performed.

### Prometheus Metrics

The operator exposes standard Prometheus metrics that are automatically collected by OpenShift Cluster Monitoring. Metrics include:

- **Aggregate metrics**: node count by status, check count by category
- **Per-node metrics**: temperature, CPU/RAM usage, load averages, uptime
- **Predefined alerts**: automatic notifications for critical nodes or failed checks

### Automatic Resource Management

The operator automatically creates and manages all necessary resources:
- Deployment and Service for the console plugin
- ServiceMonitor and PrometheusRule for monitoring
- DaemonSet for executing checks on nodes
- Route for dashboard access (optional)

## Installation

### Prerequisites

- OpenShift/Kubernetes cluster
- Cluster access with permissions to create CRD, ClusterRole, Deployment
- Docker or Podman for building images
- `kubectl` or `oc` for installation

### Quick Build and Installation

The easiest way to install the operator is to use the provided scripts:

```bash
# Complete build (operator + console plugin)
./scripts/build.sh

# Automatic installation (clean reinstall)
./scripts/install.sh --force
```

> **Warning:** `--force` performs a full cleanup before reinstalling. It deletes the operator namespace, workloads, RBAC manifests and the `nodechecks.nodecheck.openshift.io` CRD (and therefore every `NodeCheck` resource). Use it only when you really want a completely clean environment.

The scripts automatically handle:
- Multi-architecture build (AMD64 and ARM64)
- Image push to registry (if specified)
- Installation of CRD, RBAC, Deployment
- Creation of necessary resources

### Install with Helm

The repository ships with a ready-to-use Helm chart in `helm/node-check-operator`. This is the fastest way to deploy the operator on any Kubernetes/OpenShift cluster:

```bash
# Add any required registries / secrets first, then install
helm upgrade --install node-check-operator ./helm/node-check-operator \
  --namespace node-check-operator-system \
  --create-namespace
```

Customise images, resources or namespace with a custom `values.yaml`:

```bash
cat > custom-values.yaml <<'EOF'
image:
  repository: quay.io/rh_ee_afilice/node-check-operator
  tag: v1.0.7
consolePluginImage:
  repository: quay.io/rh_ee_afilice/node-check-operator-console-plugin
  tag: v1.0.7
enableOpenShiftFeatures: true
EOF

helm upgrade --install node-check-operator ./helm/node-check-operator \
  --namespace node-check-operator-system \
  --create-namespace \
  -f custom-values.yaml
```

> The Helm chart also installs the CRD (placed under `helm/node-check-operator/crds`). Upgrades follow the usual Helm workflow.

### Build with Custom Registry

For production, it is recommended to use a container registry:

```bash
# Build and push to registry
./scripts/build.sh \
  --registry quay.io \
  --image-name my-org/node-check-operator \
  --version v1.0.0 \
  --push

# Installation from registry
./scripts/install.sh \
  --registry quay.io \
  --image-name my-org/node-check-operator \
  --version v1.0.0

# Reinstall from scratch (removes CRD/RBAC/namespace)
./scripts/install.sh --force \
  --registry quay.io \
  --image-name my-org/node-check-operator \
  --version v1.0.0
```

### Verify Installation

After installation, verify that everything is active:

```bash
# Check that the operator is running
kubectl get pods -n node-check-operator-system

# Verify that the console plugin was created
kubectl get consoleplugin node-check-console-plugin

# Check created services
kubectl get svc -n node-check-operator-system
```

### Deploy without OpenShift-specific components

On vanilla Kubernetes clusters you can disable the OpenShift Console plugin, dashboard and related monitoring resources. Use the new `--openshift false` flag (or the legacy `--operator-only`) during installation:

```bash
./scripts/install.sh --openshift false
```

The flag sets the environment variable `ENABLE_OPENSHIFT_FEATURES=false` inside the controller Deployment. You can also patch the Deployment manually by editing `config/manager/manager.yaml` before applying it. When the variable is `false` the operator only deploys the core controllers (CRD/DaemonSet/metrics) and skips the console plugin, dashboard server and ServiceMonitor/PrometheusRule resources.

## Usage

### Create a NodeCheck

To monitor a node, create a `NodeCheck` resource:

```yaml
apiVersion: nodecheck.openshift.io/v1alpha1
kind: NodeCheck
metadata:
  name: nodecheck-worker-1
  namespace: default
spec:
  # NodeName specifies which node to check
  # - If empty, the executor will auto-detect the node where it's running
  # - Use "*" or "all" to check all nodes (creates child NodeChecks for each node)
  # - Use a specific node name to check only that node
  nodeName: "worker-node-1"
  
  # CheckInterval defines how often to run checks (in minutes)
  checkInterval: 5
  
  # NodeSelector filters which nodes the executor DaemonSet should run on
  # When nodeName is "*", this also filters which nodes get child NodeChecks
  # nodeSelector:
  #   node-role.kubernetes.io/worker: ""
  
  # Tolerations allow the executor DaemonSet to run on tainted nodes
  # tolerations:
  # - key: "node-role.kubernetes.io/master"
  #   operator: "Exists"
  #   effect: "NoSchedule"
  
  systemChecks:
    uptime: true
    processes: true
    resources: true
    services: true
    memory: true
    
    hardware:
      temperature: true
      ipmi: true
      bmc: true
    
    disks:
      space: true
      smart: true
      performance: true
      raid: true
      pvs: true
      lvm: true
    
    network:
      interfaces: true
      routing: true
      connectivity: true
      statistics: true
    
    systemLogs: true
  
  kubernetesChecks:
    nodeStatus: true
    pods: true
    clusterOperators: true
    nodeResources: true
```

Apply the resource:

```bash
kubectl apply -f nodecheck.yaml
```

### Check Status

```bash
# List all NodeChecks
kubectl get nodecheck

# Details of a specific NodeCheck
kubectl describe nodecheck nodecheck-worker-1

# Complete YAML output
kubectl get nodecheck nodecheck-worker-1 -o yaml
```

### Access the Interface

**OpenShift Console (recommended):**
1. Open the OpenShift console
2. Navigate to **Monitoring > Node Check**
3. Or go directly to `/nodecheck` in the console

## Available Checks

### Operating System Checks

#### Uptime and Load
- System uptime
- Load average (1min, 5min, 15min)

#### Processes
- Active processes
- CPU and memory usage per process
- Zombie processes or abnormal states

#### System Resources
- CPU statistics (user, system, idle, iowait)
- Memory and swap
- Disk I/O
- Runnable/blocked processes

#### Services
- Failed systemd services
- Critical service status

#### Hardware
- **Temperature**: readings from sensors (lm_sensors)
- **IPMI**: available IPMI sensors
- **BMC**: baseboard management controller status

#### Disks
- **Space**: filesystem usage
- **SMART**: disk health status
- **Performance**: I/O statistics
- **RAID**: RAID array status
- **LVM**: physical volumes and logical volumes status

#### Memory
- RAM usage
- Swap usage
- Available memory

#### Network
- **Interfaces**: status and configuration
- **Routing**: routing tables
- **Connectivity**: ping and traceroute tests
- **Statistics**: network counters

#### System Logs
- Recent errors from journalctl
- System reboots
- Kernel errors

### Kubernetes/OpenShift Checks

#### Node Status
- Node conditions (Ready, MemoryPressure, DiskPressure, PIDPressure)
- Allocatable resources and capacity
- Taints and labels

#### Pods
- Pods running on the node
- Failed, pending, crash loop pods
- Restart count

#### Cluster Operators
- OpenShift operator status
- Conditions and messages

#### Node Resources
- Allocatable CPU and memory
- Resource usage
- Limits and requests

## Prometheus Metrics

The operator exposes metrics on `/metrics` (port 8080) that are automatically collected by Prometheus via ServiceMonitor.

### Aggregate Metrics

- `nodecheck_nodechecks_total`: total number of managed NodeChecks
- `nodecheck_node_status_total{status}`: node count by status (Healthy/Warning/Critical/Unknown)
- `nodecheck_check_status_total{category,check,status}`: check count by category, name and status
- `nodecheck_stats_last_update_timestamp_seconds`: timestamp of last statistics update

### Per-Node Metrics

- `nodecheck_temperature_celsius{node}`: maximum temperature in degrees Celsius
- `nodecheck_cpu_usage_percent{node}`: CPU usage percentage
- `nodecheck_memory_usage_percent{node}`: memory usage percentage
- `nodecheck_uptime_seconds{node}`: uptime in seconds
- `nodecheck_load_average_1m{node}`: 1-minute load average
- `nodecheck_load_average_5m{node}`: 5-minute load average
- `nodecheck_load_average_15m{node}`: 15-minute load average

### Predefined Alerts

The operator automatically creates a `PrometheusRule` with the following alerts:

- **NodeCheckCriticalDetected**: triggered when at least one node is in critical status
- **NodeCheckDegradedChecks**: triggered when there are persistent critical checks

## Configuration

### Node Selection

The `nodeName` field supports three modes:

1. **Specific node**: specify the exact node name
   ```yaml
   spec:
     nodeName: "worker-node-1"
   ```

2. **All nodes**: use `"*"` or `"all"` to check all nodes in the cluster
   ```yaml
   spec:
     nodeName: "*"
   ```
   This creates child NodeChecks for each node automatically.

3. **Auto-detection**: leave `nodeName` empty or omit it
   ```yaml
   spec:
     nodeName: ""  # or omit the field
   ```
   Each executor pod will automatically detect the node where it's running and update the NodeCheck accordingly.

### Node Selector

Use `nodeSelector` to control which nodes the executor DaemonSet should run on:

```yaml
spec:
  nodeName: "*"
  nodeSelector:
    node-role.kubernetes.io/worker: ""
```

When `nodeName` is `"*"`, the `nodeSelector` also filters which nodes get child NodeChecks created. Only nodes matching the selector will be monitored.

### Tolerations

Use `tolerations` to allow the executor DaemonSet to run on tainted nodes:

```yaml
spec:
  nodeName: "*"
  tolerations:
  - key: "node-role.kubernetes.io/master"
    operator: "Exists"
    effect: "NoSchedule"
  - key: "dedicated"
    operator: "Equal"
    value: "compute"
    effect: "NoSchedule"
```

Tolerations are aggregated from all NodeChecks, so if multiple NodeChecks specify tolerations, the DaemonSet will tolerate all of them.

### Check Intervals

The interval between checks is configurable via `checkInterval` (in minutes):

```yaml
spec:
  checkInterval: 5  # Checks every 5 minutes (default)
```

Valid values: 1-1440 minutes.

### Enable/Disable Checks

All checks are optional and can be enabled or disabled in the NodeCheck spec:

```yaml
spec:
  systemChecks:
    uptime: true          # Enable uptime check
    processes: false      # Disable processes check
    # ... other checks
```

### Installation Namespace

By default, the operator is installed in the `node-check-operator-system` namespace. To change namespace, modify:

- `config/manager/manager.yaml`: Deployment namespace
- Environment variables in Deployment: `WATCH_NAMESPACE`

### Examples

See the `examples/` directory for complete examples:

- `basic-nodecheck.yaml`: basic configuration for a single node
- `full-nodecheck.yaml`: all checks enabled
- `nodecheck-with-selector.yaml`: using NodeSelector and Tolerations with `nodeName="*"`
- `nodecheck-auto-detect.yaml`: auto-detection example with NodeSelector

## Status and Results

### Possible States

Each NodeCheck and each individual check can have one of the following states:

- **Healthy**: everything ok, no problems detected
- **Warning**: minor problems or conditions to monitor
- **Critical**: serious problems requiring attention
- **Unknown**: check not available or not executed

### Results Structure

Results are organized by category:

```yaml
status:
  overallStatus: "Healthy|Warning|Critical|Unknown"
  lastCheckTime: "2024-01-01T12:00:00Z"
  message: "General status description"
  checkResults:
    systemResults:
      uptime: {...}
      processes: {...}
      resources: {...}
      # ... other system checks
    kubernetesResults:
      nodeStatus: {...}
      pods: {...}
      # ... other Kubernetes checks
```

Each check includes:

```yaml
status: "Healthy|Warning|Critical|Unknown"
message: "Descriptive message"
timestamp: "2024-01-01T12:00:00Z"
details:
  # Check-specific data
  cpu_usage: 45.2
  memory_usage_percent: 67.8
  # ... other details
```

## Troubleshooting

### Operator Not Starting

```bash
# Check logs
kubectl logs -n node-check-operator-system \
  deployment/node-check-operator-controller-manager

# Check RBAC permissions
kubectl describe clusterrole node-check-operator-manager-role
```

### Console Plugin Not Appearing

```bash
# Verify that the ConsolePlugin was created
kubectl get consoleplugin node-check-console-plugin

# Verify plugin deployment
kubectl get pods -n node-check-operator-system \
  -l app=node-check-console-plugin

# Check plugin logs
kubectl logs -n node-check-operator-system \
  deployment/node-check-console-plugin

# Restart console if necessary
kubectl rollout restart deployment/console -n openshift-console
```

### Checks Not Executing

```bash
# Verify that the executor DaemonSet is active
kubectl get daemonset -n node-check-operator-system

# Verify executor pods on nodes
kubectl get pods -n node-check-operator-system \
  -l app=node-check-executor

# Check executor logs
kubectl logs -n node-check-operator-system \
  -l app=node-check-executor --tail=100
```

### Metrics Not Appearing

```bash
# Verify that the ServiceMonitor was created
kubectl get servicemonitor -n node-check-operator-system

# Verify that the metrics Service exists
kubectl get svc node-check-operator-metrics -n node-check-operator-system

# Test metrics endpoint directly
kubectl port-forward -n node-check-operator-system \
  svc/node-check-operator-metrics 8080:8080
curl http://localhost:8080/metrics | grep nodecheck
```

### Permission Denied on Nodes

If checks fail with permission errors, verify that:

1. Required packages are installed on nodes
2. Commands are executable (some require root privileges)
3. The executor DaemonSet has necessary permissions

## Development

### Project Structure

```
├── api/v1alpha1/          # API definitions (CRD)
├── controllers/           # Kubernetes controllers
│   ├── nodecheck_controller.go
│   ├── consoleplugin_controller.go
│   └── executor_daemonset_controller.go
├── pkg/
│   ├── checks/           # Check implementations
│   ├── dashboard/        # Dashboard server and API
│   └── metrics/          # Prometheus metrics
├── console-plugin/       # OpenShift Console Plugin (React)
├── config/               # Kubernetes manifests
│   ├── crd/             # Custom Resource Definitions
│   ├── rbac/            # RBAC permissions
│   └── manager/         # Operator deployment
└── scripts/             # Build and installation scripts
```

### Manual Build

```bash
# Build operator
make build
make docker-build

# Build console plugin
make plugin-build
make plugin-docker-build

# Build everything
make build-all
```

### Run Locally

```bash
# Run operator in development mode
make run

# Or with Go directly
go run ./main.go --mode=operator
```

### Adding New Checks

1. Add the field in the spec (`api/v1alpha1/nodecheck_types.go`)
2. Implement the check in `pkg/checks/`
3. Integrate in the controller (`controllers/nodecheck_controller.go`)
4. Update status logic
5. Add tests if necessary

### Testing

```bash
# Unit tests
make test

# Check formatting
make fmt

# Check linting
make vet
```

## Contributing

1. Fork the repository
2. Create a branch for the feature (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

Apache License 2.0

## Support

For issues, questions or suggestions:
- Open an issue on GitHub
- Check examples in `config/samples/` and `examples/`
- Consult README and QUICKSTART.md for detailed information
