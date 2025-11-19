# Quick Start - Node Check Operator

## Quick Installation

### 1. Prerequisites

#### Build
```bash
# Only Docker or Podman required!
docker --version    # or
podman --version
```

#### Installation
```bash
# Only kubectl or oc required!
kubectl version     # or oc version
```

### 2. Build and Installation

#### Local Build (Development)
```bash
# Complete build (multi-architecture)
./scripts/build.sh

# Automatic installation
./scripts/install.sh
```

#### Build with Registry (Production)
```bash
# Build and push to registry
./scripts/build.sh --registry quay.io --image-name my-org/node-check-operator --version v1.0.0 --push

# Installation from registry
./scripts/install.sh --registry quay.io --image-name my-org/node-check-operator --version v1.0.0
```

#### Build on Raspberry Pi for Deploy on AMD64
```bash
# On Raspberry Pi: Multi-architecture build
./scripts/build.sh --registry quay.io --image-name my-org/node-check-operator --version v1.0.0 --push

# On AMD64 cluster: Installation
./scripts/install.sh --registry quay.io --image-name my-org/node-check-operator --version v1.0.0
```

### 3. Verify Installation
```bash
# Check that the operator is active
kubectl get pods -n node-check-operator-system

# Check NodeChecks
kubectl get nodecheck
```

### 4. Access the Dashboard

#### OpenShift Console Plugin (Recommended)
- Open OpenShift Console
- Navigate to **Monitoring > Node Check**
- Or go to `/nodecheck`

#### Standalone Dashboard
```bash
# Port-forward for direct access
kubectl port-forward -n node-check-operator-system svc/node-check-operator-dashboard 8082:8082
# Open http://localhost:8082
```

## Create a NodeCheck

```bash
# Create an example NodeCheck
kubectl apply -f config/samples/nodecheck_v1alpha1_nodecheck.yaml

# Verify status
kubectl get nodecheck
kubectl describe nodecheck nodecheck-sample
```

## Useful Commands

```bash
# Check operator logs
kubectl logs -n node-check-operator-system deployment/node-check-operator-controller-manager

# Verify console plugin
kubectl get consoleplugin -n node-check-operator-system

# Uninstall everything
kubectl delete -f config/manager/manager.yaml
kubectl delete consoleplugin node-check-console-plugin
kubectl delete deployment node-check-console-plugin -n node-check-operator-system
```

## Troubleshooting

### Build Fails
```bash
# Verify dependencies
go mod tidy
cd console-plugin && npm install && cd ..

# Retry build
./scripts/build.sh
```

### Installation Fails
```bash
# Verify cluster connection
kubectl cluster-info

# Verify Docker images
docker images | grep node-check

# Retry installation
./scripts/install.sh
```

### Plugin Not Appearing
```bash
# Verify ConsolePlugin
kubectl get consoleplugin -n node-check-operator-system

# Verify plugin deployment
kubectl get pods -n node-check-operator-system -l app=node-check-console-plugin

# Restart console if necessary
kubectl rollout restart deployment/console -n openshift-console
```

## Development

### Build Operator Only
```bash
make build
make docker-build
```

### Build Plugin Only
```bash
make plugin-build
make plugin-docker-build
```

### Local Testing
```bash
# Test operator
make run

# Test plugin (requires OpenShift Console)
cd console-plugin && npm run dev
```

## Support

- **Documentation**: README.md
- **Examples**: config/samples/
- **Scripts**: scripts/
- **Configuration**: config/
