#!/bin/bash

# Node Check Operator - Unified Install Script
# Installs the operator on any Kubernetes/OpenShift cluster

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Utility functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

# Update ENABLE_OPENSHIFT_FEATURES value inside manager.yaml using only POSIX tools
update_enable_feature_flag() {
    local file_path="$1"
    local new_value="$2"
    local temp_file
    temp_file=$(mktemp) || {
        log_error "Unable to create temporary file while updating ${file_path}"
        exit 1
    }

    if awk -v val="$new_value" '
        BEGIN {pending=0; replaced=0}
        {
            if (pending && $0 ~ /^[[:space:]]+value:/) {
                sub(/value: "[^"]*"/, "value: \"" val "\"")
                pending=0
                replaced=1
            }
            print
            if ($0 ~ /- name: ENABLE_OPENSHIFT_FEATURES/) {
                pending=1
            }
        }
        END {
            if (replaced != 1) {
                exit 1
            }
        }
    ' "$file_path" > "$temp_file"; then
        mv "$temp_file" "$file_path"
    else
        rm -f "$temp_file"
        log_error "ENABLE_OPENSHIFT_FEATURES block not found in ${file_path}"
        exit 1
    fi
}

# Default configuration
REGISTRY=${REGISTRY:-"quay.io"}
IMAGE_NAME=${IMAGE_NAME:-"node-check-operator"}
VERSION=${VERSION:-"latest"}
NAMESPACE=${NAMESPACE:-"node-check-operator-system"}
KUBECTL_CMD=""
FORCE_REINSTALL=${FORCE_REINSTALL:-"false"}
ENABLE_OPENSHIFT_FEATURES=${ENABLE_OPENSHIFT_FEATURES:-"true"}
USE_HELM=${USE_HELM:-"false"}
HELM_RELEASE_NAME=${HELM_RELEASE_NAME:-"node-check-operator"}
ONLY_REMOVE=${ONLY_REMOVE:-"false"}

# Show help
show_help() {
    echo "Node Check Operator - Install Script"
    echo
    echo "Usage: $0 [OPTIONS]"
    echo
    echo "Options:"
    echo "  --registry REGISTRY    Image registry (default: quay.io)"
    echo "  --image-name NAME      Image name (default: node-check-operator)"
    echo "  --version VERSION      Image version (default: latest)"
    echo "  --namespace NAMESPACE  Namespace for operator (default: node-check-operator-system)"
    echo "  --force                Force reinstallation by removing existing resources"
    echo "  --only-remove          Uninstall the operator without reinstalling"
    echo "  --openshift [true|false] Enable OpenShift-specific features (console plugin, dashboard). Default: true"
    echo "  --helm                 Install using Helm chart instead of raw manifests"
    echo "  --help                 Show this help"
    echo
    echo "Examples:"
    echo "  $0"
    echo "  $0 --registry docker.io --image-name my-org/node-check-operator --version v1.0.0"
    echo "  $0 --openshift false   # Disable OpenShift-specific features"
    echo "  $0 --helm              # Install using Helm chart"
    echo "  $0 --helm --force      # Clean and reinstall using Helm"
    echo "  $0 --only-remove       # Uninstall the operator"
    echo
}

# Parse arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --registry)
                REGISTRY="$2"
                shift 2
                ;;
            --image-name)
                IMAGE_NAME="$2"
                shift 2
                ;;
            --version)
                VERSION="$2"
                shift 2
                ;;
            --namespace)
                NAMESPACE="$2"
                shift 2
                ;;
            --force)
                FORCE_REINSTALL="true"
                shift
                ;;
            --only-remove|--uninstall)
                ONLY_REMOVE="true"
                shift
                ;;
            --openshift)
                if [[ -z "$2" ]]; then
                    log_error "--openshift requires a value (true/false)"
                    exit 1
                fi
                case "$(echo "$2" | tr '[:upper:]' '[:lower:]')" in
                    true|yes|1)
                        ENABLE_OPENSHIFT_FEATURES="true"
                        ;;
                    false|no|0)
                        ENABLE_OPENSHIFT_FEATURES="false"
                        ;;
                    *)
                        log_error "Invalid value for --openshift: $2 (use true/false)"
                        exit 1
                        ;;
                esac
                shift 2
                ;;
            --helm)
                USE_HELM="true"
                shift
                ;;
            --help)
                show_help
                exit 0
                ;;
            *)
                log_error "Unknown argument: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check kubectl/oc
    if command -v oc &> /dev/null; then
        KUBECTL_CMD="oc"
        log_info "Found 'oc' command"
    elif command -v kubectl &> /dev/null; then
        KUBECTL_CMD="kubectl"
        log_info "Found 'kubectl' command"
    else
        log_error "Neither 'oc' nor 'kubectl' is available. Install one of them."
        exit 1
    fi
    
    # Check Helm if --helm is used
    if [ "${USE_HELM}" = "true" ]; then
        if ! command -v helm &> /dev/null; then
            log_error "Helm is required when using --helm flag. Install Helm: https://helm.sh/docs/intro/install/"
            exit 1
        fi
        log_info "Found 'helm' command"
        
        # Check Helm chart directory exists
        if [ ! -d "helm/node-check-operator" ]; then
            log_error "Helm chart not found at helm/node-check-operator"
            exit 1
        fi
        log_info "Helm chart found"
    fi
    
    # Check cluster connection
    if ! $KUBECTL_CMD cluster-info &> /dev/null; then
        log_error "Cannot connect to cluster. Check configuration."
        exit 1
    fi
    
    # Check cluster architecture
    NODE_ARCH=$($KUBECTL_CMD get nodes -o jsonpath='{.items[0].status.nodeInfo.architecture}')
    log_info "Cluster architecture: ${NODE_ARCH}"
    
    # Check Docker or Podman for image pull
    if command -v docker &> /dev/null; then
        CONTAINER_CMD="docker"
        log_info "Found Docker for image pull"
    elif command -v podman &> /dev/null; then
        CONTAINER_CMD="podman"
        log_info "Found Podman for image pull"
    else
        log_warn "Neither Docker nor Podman is available. Ensure images are available in the cluster."
    fi
    
    log_info "Prerequisites verified"
}

# Install CRD
install_crd() {
    log_info "Installing Custom Resource Definition..."
    
    if $KUBECTL_CMD apply -f config/crd/bases/nodecheck.openshift.io_nodechecks.yaml; then
        log_info "CRD installed successfully"
    else
        log_error "Error installing CRD"
        exit 1
    fi
}

# Install RBAC
install_rbac() {
    log_info "Installing RBAC..."
    
    if $KUBECTL_CMD apply -f config/rbac/role.yaml; then
        log_info "ClusterRole installed"
    else
        log_error "Error installing ClusterRole"
        exit 1
    fi
    
    if $KUBECTL_CMD apply -f config/rbac/role_binding.yaml; then
        log_info "ClusterRoleBinding installed"
    else
        log_error "Error installing ClusterRoleBinding"
        exit 1
    fi
}

# Configure Security Context Constraint for OpenShift
# Note: The privileged SCC is now automatically configured via ClusterRoleBinding in manager.yaml
configure_scc() {
    log_info "Verifying Security Context Constraint configuration (OpenShift)..."
    
    # The ClusterRoleBinding for the privileged SCC is included in manager.yaml
    # and is automatically created during manager installation.
    # It is no longer necessary to manually run 'oc adm policy add-scc-to-user'
    
    log_info "SCC 'privileged' will be automatically configured via ClusterRoleBinding in manager.yaml"
    
    # Check if we are on OpenShift and if the ClusterRoleBinding exists
    if command -v oc &> /dev/null; then
        if $KUBECTL_CMD get clusterrolebinding node-check-operator-controller-manager-privileged &>/dev/null; then
            log_info "ClusterRoleBinding for privileged SCC found"
        else
            log_info "ClusterRoleBinding for privileged SCC will be created during manager installation"
        fi
    fi
}

# Update manifests with correct configuration
update_manifests() {
    log_step "Updating manifests with correct configuration..."
    
    # Create temporary directory for manifests
    TEMP_DIR=$(mktemp -d)
    cp -r config/manager $TEMP_DIR/
    
    # Update manager.yaml
    sed -i.bak "s|image: controller:latest|image: ${REGISTRY}/${IMAGE_NAME}:${VERSION}|g" $TEMP_DIR/manager/manager.yaml
    sed -i.bak "s|namespace: node-check-operator-system|namespace: ${NAMESPACE}|g" $TEMP_DIR/manager/manager.yaml
    rm -f $TEMP_DIR/manager/manager.yaml.bak
    
    # Update ENABLE_OPENSHIFT_FEATURES environment variable
    update_enable_feature_flag "$TEMP_DIR/manager/manager.yaml" "${ENABLE_OPENSHIFT_FEATURES}"
    
    # Update kustomization if it exists
    if [ -f "$TEMP_DIR/manager/kustomization.yaml" ]; then
    sed -i.bak "s|namespace: node-check-operator-system|namespace: ${NAMESPACE}|g" $TEMP_DIR/manager/kustomization.yaml
    rm -f $TEMP_DIR/manager/kustomization.yaml.bak
    fi
    
    log_info "Manifests updated"
}

# Remove all resources managed by the operator
remove_operator() {
    log_info "Removing operator and managed resources..."
    
    # Remove operator Deployment
    $KUBECTL_CMD delete -f $TEMP_DIR/manager/manager.yaml --ignore-not-found=true 2>/dev/null || true
    
    # Remove executor DaemonSet (if exists)
    $KUBECTL_CMD delete daemonset node-check-executor -n ${NAMESPACE} --ignore-not-found=true 2>/dev/null || true
    
    # Remove ConsolePlugin CR (if exists)
    $KUBECTL_CMD delete consoleplugin node-check-console-plugin --ignore-not-found=true 2>/dev/null || true
    
    # Remove ConsolePlugin Deployment and Service (if exist)
    $KUBECTL_CMD delete deployment node-check-console-plugin -n ${NAMESPACE} --ignore-not-found=true 2>/dev/null || true
    $KUBECTL_CMD delete service node-check-console-plugin -n ${NAMESPACE} --ignore-not-found=true 2>/dev/null || true
    
    # Remove Dashboard Service and Route (if exist)
    $KUBECTL_CMD delete service node-check-operator-dashboard -n ${NAMESPACE} --ignore-not-found=true 2>/dev/null || true
    $KUBECTL_CMD delete route node-check-operator-dashboard -n ${NAMESPACE} --ignore-not-found=true 2>/dev/null || true
    
    # Wait a bit to ensure resources are removed
    sleep 2
    
    log_info "Operator and managed resources removed"
}

# Install operator with Helm
install_with_helm() {
    log_info "Installing operator using Helm..."
    
    # Build full image paths
    OPERATOR_IMAGE="${REGISTRY}/${IMAGE_NAME}:${VERSION}"
    CONSOLE_PLUGIN_IMAGE="${REGISTRY}/${IMAGE_NAME}-console-plugin:${VERSION}"
    
    # Prepare Helm values
    HELM_VALUES=""
    HELM_VALUES="${HELM_VALUES} --set namespace.name=${NAMESPACE}"
    # namespace.create will be set to false after we create the namespace manually
    HELM_VALUES="${HELM_VALUES} --set image.repository=${REGISTRY}/${IMAGE_NAME}"
    HELM_VALUES="${HELM_VALUES} --set image.tag=${VERSION}"
    HELM_VALUES="${HELM_VALUES} --set consolePluginImage.repository=${REGISTRY}/${IMAGE_NAME}-console-plugin"
    HELM_VALUES="${HELM_VALUES} --set consolePluginImage.tag=${VERSION}"
    HELM_VALUES="${HELM_VALUES} --set enableOpenShiftFeatures=${ENABLE_OPENSHIFT_FEATURES}"
    
    # Check if resources exist that might conflict (created manually before)
    if $KUBECTL_CMD get clusterrole node-check-operator-manager-role &>/dev/null; then
        log_warn "Found existing ClusterRole 'node-check-operator-manager-role' not managed by Helm"
        log_info "Removing existing RBAC resources to avoid conflicts..."
        $KUBECTL_CMD delete clusterrolebinding node-check-operator-manager-rolebinding --ignore-not-found=true 2>/dev/null || true
        $KUBECTL_CMD delete clusterrole node-check-operator-manager-role --ignore-not-found=true 2>/dev/null || true
        sleep 2
    fi
    
    # Always check if namespace exists and wait for deletion if needed
    # This is important because Helm --create-namespace fails if namespace is still terminating
    if $KUBECTL_CMD get namespace ${NAMESPACE} &>/dev/null; then
        NS_PHASE=$($KUBECTL_CMD get namespace ${NAMESPACE} -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
        if [ "$NS_PHASE" = "Terminating" ] || [ "$NS_PHASE" = "Active" ]; then
            if [ "$NS_PHASE" = "Terminating" ]; then
                log_warn "Namespace ${NAMESPACE} is terminating, waiting for deletion..."
            else
                log_warn "Namespace ${NAMESPACE} still exists, deleting it..."
                $KUBECTL_CMD delete namespace ${NAMESPACE} --ignore-not-found=true 2>/dev/null || true
            fi
            wait_for_namespace_deletion
        fi
    fi
    
    # Double-check namespace is gone before proceeding
    if $KUBECTL_CMD get namespace ${NAMESPACE} &>/dev/null; then
        log_error "Namespace ${NAMESPACE} still exists after waiting. Please delete it manually and try again."
        exit 1
    fi
    
    # Create namespace manually with OpenShift-specific labels and annotations
    # This ensures the namespace exists before Helm tries to create resources in it
    log_info "Creating namespace ${NAMESPACE} with OpenShift labels..."
    cat <<EOF | $KUBECTL_CMD apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: ${NAMESPACE}
  labels:
    control-plane: controller-manager
    openshift.io/cluster-monitoring: "true"
    security.openshift.io/scc.podSecurityLabelSync: "true"
    pod-security.kubernetes.io/warn: privileged
    pod-security.kubernetes.io/audit: privileged
    pod-security.kubernetes.io/warn-version: latest
    pod-security.kubernetes.io/audit-version: latest
  annotations:
    security.openshift.io/MinimallySufficientPodSecurityStandard: privileged
EOF
    
    if [ $? -ne 0 ]; then
        log_error "Failed to create namespace ${NAMESPACE}"
        exit 1
    fi
    
    # Verify namespace was created
    sleep 1  # Give Kubernetes a moment to create the namespace
    if ! $KUBECTL_CMD get namespace ${NAMESPACE} &>/dev/null; then
        log_error "Namespace ${NAMESPACE} was not created successfully"
        exit 1
    fi
    log_info "Namespace ${NAMESPACE} created successfully"
    
    # Set namespace.create=false since we created it manually
    HELM_VALUES="${HELM_VALUES} --set namespace.create=false"
    
    # Install or upgrade
        if helm list -n ${NAMESPACE} 2>/dev/null | grep -q "^${HELM_RELEASE_NAME}"; then
            log_info "Upgrading existing Helm release..."
            if helm upgrade ${HELM_RELEASE_NAME} ./helm/node-check-operator \
                -n ${NAMESPACE} \
                ${HELM_VALUES} \
                --wait \
                --timeout 10m; then
                log_info "Helm release upgraded successfully"
            else
                log_error "Error upgrading Helm release"
                exit 1
            fi
        else
            log_info "Installing new Helm release..."
            # Helm will create the namespace via template (namespace.create=true) if it doesn't exist
            if helm install ${HELM_RELEASE_NAME} ./helm/node-check-operator \
                -n ${NAMESPACE} \
                ${HELM_VALUES} \
                --wait \
                --timeout 10m; then
                log_info "Helm release installed successfully"
            else
                log_error "Error installing Helm release"
                log_info "If you see 'resource already exists' errors, try running with --force to clean up first"
                exit 1
            fi
        fi
}

# Install operator with retry
install_operator() {
    log_info "Installing operator..."
    
    # Create namespace if it doesn't exist
    $KUBECTL_CMD create namespace ${NAMESPACE} --dry-run=client -o yaml | $KUBECTL_CMD apply -f -
    
    # If force, remove first
    if [ "${FORCE_REINSTALL}" = "true" ]; then
        remove_operator
        sleep 2
    fi
    
    # Try installation
    if ! $KUBECTL_CMD apply -f $TEMP_DIR/manager/manager.yaml; then
        log_warn "First attempt failed, trying to remove and reinstall..."
        remove_operator
        sleep 3
        
        if $KUBECTL_CMD apply -f $TEMP_DIR/manager/manager.yaml; then
            log_info "Operator installed successfully after retry"
        else
            log_error "Error installing operator even after retry"
            exit 1
        fi
    else
        log_info "Operator installed successfully"
    fi
}

wait_for_namespace_deletion() {
    if ! $KUBECTL_CMD get namespace ${NAMESPACE} &>/dev/null; then
        return
    fi
    log_info "Waiting for namespace ${NAMESPACE} to terminate..."
    for i in {1..40}; do
        if ! $KUBECTL_CMD get namespace ${NAMESPACE} &>/dev/null; then
            log_info "Namespace ${NAMESPACE} deleted"
            return
        fi
        sleep 3
    done
    log_warn "Namespace ${NAMESPACE} is still terminating; continuing installation"
}

force_cleanup() {
    log_step "Force cleanup requested - removing previous installation"

    # If using Helm, uninstall the release first
    if [ "${USE_HELM}" = "true" ]; then
        log_info "Uninstalling Helm release ${HELM_RELEASE_NAME}..."
        if helm list -n ${NAMESPACE} | grep -q "^${HELM_RELEASE_NAME}"; then
            helm uninstall ${HELM_RELEASE_NAME} -n ${NAMESPACE} --ignore-not-found=true 2>/dev/null || true
            log_info "Helm release uninstalled"
        else
            log_info "No Helm release found, continuing with manual cleanup"
        fi
    fi

    if $KUBECTL_CMD api-resources --api-group=nodecheck.openshift.io &>/dev/null; then
        log_info "Deleting NodeCheck resources in all namespaces"
        $KUBECTL_CMD delete nodecheck --all --all-namespaces --ignore-not-found=true 2>/dev/null || true
    else
        log_info "NodeCheck CRD not registered, skipping CR cleanup"
    fi

    log_info "Deleting operator workloads in namespace ${NAMESPACE}"
    $KUBECTL_CMD delete deployment node-check-operator-controller-manager -n ${NAMESPACE} --ignore-not-found=true 2>/dev/null || true
    $KUBECTL_CMD delete daemonset node-check-executor -n ${NAMESPACE} --ignore-not-found=true 2>/dev/null || true
    $KUBECTL_CMD delete deployment node-check-console-plugin -n ${NAMESPACE} --ignore-not-found=true 2>/dev/null || true
    $KUBECTL_CMD delete service node-check-console-plugin -n ${NAMESPACE} --ignore-not-found=true 2>/dev/null || true
    $KUBECTL_CMD delete service node-check-operator-metrics -n ${NAMESPACE} --ignore-not-found=true 2>/dev/null || true
    $KUBECTL_CMD delete service node-check-operator-dashboard -n ${NAMESPACE} --ignore-not-found=true 2>/dev/null || true

    if $KUBECTL_CMD api-resources --api-group=monitoring.coreos.com &>/dev/null; then
        log_info "Deleting monitoring custom resources"
        $KUBECTL_CMD delete servicemonitor node-check-operator-metrics -n ${NAMESPACE} --ignore-not-found=true 2>/dev/null || true
        $KUBECTL_CMD delete prometheusrule node-check-operator-rules -n ${NAMESPACE} --ignore-not-found=true 2>/dev/null || true
    fi

    if $KUBECTL_CMD api-resources --api-group=console.openshift.io &>/dev/null; then
        log_info "Deleting ConsolePlugin custom resource"
        $KUBECTL_CMD delete consoleplugin node-check-console-plugin --ignore-not-found=true 2>/dev/null || true
    fi

    if $KUBECTL_CMD api-resources --api-group=route.openshift.io &>/dev/null; then
        $KUBECTL_CMD delete route node-check-operator-dashboard -n ${NAMESPACE} --ignore-not-found=true 2>/dev/null || true
    fi

    log_info "Deleting namespace ${NAMESPACE} (if present)"
    $KUBECTL_CMD delete namespace ${NAMESPACE} --ignore-not-found=true 2>/dev/null || true
    wait_for_namespace_deletion

    # Delete RBAC and CRD definitions (they may have been created manually before using Helm)
    log_info "Deleting RBAC and CRD definitions"
    $KUBECTL_CMD delete -f config/rbac/role_binding.yaml --ignore-not-found=true 2>/dev/null || true
    $KUBECTL_CMD delete -f config/rbac/role.yaml --ignore-not-found=true 2>/dev/null || true
    $KUBECTL_CMD delete -f config/crd/bases/nodecheck.openshift.io_nodechecks.yaml --ignore-not-found=true 2>/dev/null || true
    
    # Wait a bit for resources to be fully deleted
    sleep 2

    log_info "Force cleanup completed"
}

# Verify installation
verify_installation() {
    log_info "Verifying installation..."
    
    # Verify that the operator deployment is ready (now it's a Deployment, not DaemonSet)
    if $KUBECTL_CMD wait --for=condition=available --timeout=300s deployment/node-check-operator-controller-manager -n ${NAMESPACE} 2>/dev/null; then
        log_info "Operator is ready"
    else
        log_error "Operator is not ready after 5 minutes"
        log_info "Check logs with: $KUBECTL_CMD logs -n ${NAMESPACE} deployment/node-check-operator-controller-manager"
        exit 1
    fi
    
    # Note: Console Plugin will be created automatically by the operator when a NodeCheck is created
    # We don't verify it here because it may not exist yet
    if [ "${ENABLE_OPENSHIFT_FEATURES}" = "true" ]; then
        if $KUBECTL_CMD get deployment node-check-console-plugin -n ${NAMESPACE} &>/dev/null; then
            log_info "Console Plugin deployment found (will be ready shortly)"
        else
            log_info "Console Plugin will be created automatically when you create a NodeCheck resource"
        fi
    fi
    
    # Verify that the CRD is installed
    if $KUBECTL_CMD get crd nodechecks.nodecheck.openshift.io &> /dev/null; then
        log_info "CRD is available"
    else
        log_error "CRD is not available"
        exit 1
    fi
}

# Create example NodeCheck
create_example() {
    log_info "Creating example NodeCheck..."
    
    # Get the name of the first worker node
    NODE_NAME=$($KUBECTL_CMD get nodes -o jsonpath='{.items[0].metadata.name}')
    
    if [ -z "$NODE_NAME" ]; then
        log_error "No nodes found in cluster"
        exit 1
    fi
    
    log_info "Using node: $NODE_NAME"
    
    # Create example NodeCheck
    cat <<EOF | $KUBECTL_CMD apply -f -
apiVersion: nodecheck.openshift.io/v1alpha1
kind: NodeCheck
metadata:
  name: full-nodecheck
  namespace: ${NAMESPACE}
spec:
  checkInterval: 1
  kubernetesChecks:
    clusterOperators: true
    cniPlugin: true
    containerRuntime: true
    kubeletHealth: true
    nodeConditions: true
    nodeResources: true
    nodeResourceUsage: true
    nodeStatus: true
    pods: true
  nodeName: '*'
  systemChecks:
    contextSwitches: true
    cpuFrequency: true
    cpuStealTime: true
    fileDescriptors: true
    hardware:
      bmc: true
      cpuMicrocode: true
      fanStatus: true
      ipmi: true
      memoryErrors: true
      pcieErrors: true
      powerSupply: true
      temperature: true
    interruptsBalance: true
    kernelModules: true
    kernelPanics: true
    memory: true
    memoryFragmentation: true
    network:
      bondingStatus: true
      connectivity: true
      dnsResolution: true
      errors: true
      interfaces: true
      latency: true
      routing: true
      statistics: true
      firewallRules: true
    ntpSync: true
    oomKiller: true
    processes: true
    resources: true
    selinuxStatus: true
    services: true
    sshAccess: true
    swapActivity: true
    systemLogs: true
    uninterruptibleTasks: true
    uptime: true
    zombieProcesses: true
    disks:
      filesystemErrors: true
      inodeUsage: true
      ioWait: true
      lvm: true
      mountPoints: true
      performance: true
      pvs: true
      queueDepth: true
      raid: true
      smart: true
      space: true
EOF
    
    log_info "Example NodeCheck created"
}

# Show useful information
show_info() {
    log_info "Installation completed!"
    echo
    echo "To check operator status:"
    echo "  $KUBECTL_CMD get pods -n ${NAMESPACE}"
    echo
    echo "To check NodeChecks:"
    echo "  $KUBECTL_CMD get nodecheck"
    echo
    echo "To see NodeCheck details:"
    echo "  $KUBECTL_CMD describe nodecheck example-nodecheck"
    echo
    echo "To see operator logs:"
    echo "  $KUBECTL_CMD logs -n ${NAMESPACE} deployment/node-check-operator-controller-manager"
    echo
    
    if [ "${ENABLE_OPENSHIFT_FEATURES}" = "true" ]; then
        echo "OpenShift Console Plugin:"
        echo "  The plugin is integrated into the OpenShift Console"
        echo "  Navigate to: Monitoring > Node Check"
        echo "  Or: /nodecheck in the console"
        echo
        echo "Web Dashboard:"
        echo "  Port-forward: $KUBECTL_CMD port-forward -n ${NAMESPACE} svc/node-check-operator-dashboard 8082:31682"
        echo "  Then open: http://localhost:8082"
        echo
    fi
    
    echo "To uninstall the operator:"
    if [ "${USE_HELM}" = "true" ]; then
        echo "  helm uninstall ${HELM_RELEASE_NAME} -n ${NAMESPACE}"
    else
    echo "  $KUBECTL_CMD delete -f $TEMP_DIR/manager/manager.yaml"
    echo "  $KUBECTL_CMD delete daemonset node-check-executor -n ${NAMESPACE} --ignore-not-found=true"
    echo "  $KUBECTL_CMD delete -f config/rbac/role_binding.yaml"
    echo "  $KUBECTL_CMD delete -f config/rbac/role.yaml"
    echo "  $KUBECTL_CMD delete -f config/crd/bases/nodecheck.openshift.io_nodechecks.yaml"
    fi
    echo
    echo "Configuration used:"
    echo "  Registry: ${REGISTRY}"
    echo "  Image: ${IMAGE_NAME}"
    echo "  Version: ${VERSION}"
    echo "  Namespace: ${NAMESPACE}"
    echo "  OpenShift features: ${ENABLE_OPENSHIFT_FEATURES}"
    echo "  Installation method: $([ "${USE_HELM}" = "true" ] && echo "Helm" || echo "Manifests")"
}

# Cleanup
cleanup() {
    if [ -n "$TEMP_DIR" ] && [ -d "$TEMP_DIR" ]; then
        rm -rf "$TEMP_DIR"
    fi
}

# Main function
main() {
    log_info "Starting Node Check Operator installation..."
    
    parse_args "$@"
    
    if [ "${ENABLE_OPENSHIFT_FEATURES}" != "true" ]; then
        log_info "OpenShift features disabled - operator will only deploy core controllers"
    else
        log_info "OpenShift features enabled - operator will create ConsolePlugin, Dashboard Service, and Route"
    fi
    
    check_prerequisites
    
    # If only-remove flag is set, just cleanup and exit
    if [ "${ONLY_REMOVE}" = "true" ]; then
        force_cleanup
        log_info "Uninstallation completed successfully!"
        exit 0
    fi
    
    if [ "${FORCE_REINSTALL}" = "true" ]; then
        force_cleanup
    fi

    if [ "${USE_HELM}" = "true" ]; then
        # Install using Helm
        install_with_helm
    else
        # Install using traditional manifests
    install_crd
    install_rbac
    update_manifests
    install_operator
    configure_scc
    fi
    
    # Console plugin, dashboard service and route are automatically managed by operator
    # when ENABLE_OPENSHIFT_FEATURES=true (default)
    if [ "${ENABLE_OPENSHIFT_FEATURES}" = "true" ]; then
        log_info "OpenShift features enabled - operator will create ConsolePlugin, Dashboard Service, and Route"
    else
        log_info "OpenShift features disabled - operator will only deploy core controllers"
    fi
    
    verify_installation
    
    # Create example NodeCheck (optional, but useful)
    if [ "${ENABLE_OPENSHIFT_FEATURES}" = "true" ]; then
        sleep 10
        create_example
    else
        log_info "Skipping example NodeCheck creation (OpenShift features disabled)"
    fi
    
    show_info
    
    log_info "Installation completed successfully!"
}

# Trap for cleanup
trap cleanup EXIT

# Run main
main "$@"