# Repository Setup Guide

This document provides information for setting up the Node Check Operator repository on GitHub/GitLab.

## Repository Description

```
Kubernetes/OpenShift operator for comprehensive bare metal node monitoring. Performs in-depth system, hardware, disk, network, and cluster-level health checks with OpenShift Console integration, web dashboard, and Prometheus metrics.
```

## Tags / Keywords

```
kubernetes
openshift
operator
node-monitoring
bare-metal
health-checks
prometheus
metrics
console-plugin
helm-chart
golang
controller-runtime
infrastructure-monitoring
cluster-monitoring
node-health
system-monitoring
hardware-monitoring
```

## Topics (GitHub)

```
kubernetes-operator
openshift-operator
node-monitoring
bare-metal-monitoring
health-checks
prometheus-metrics
openshift-console-plugin
helm-chart
golang
kubernetes-controller
infrastructure-as-code
cluster-management
system-monitoring
hardware-monitoring
network-monitoring
```

## Repository Metadata

- **Language**: Go
- **License**: (Specify your license, e.g., Apache-2.0, MIT)
- **Platform**: Kubernetes, OpenShift
- **Architecture**: AMD64, ARM64

## Security Checklist

Before pushing to the repository, ensure:

- [x] All container registry references use `quay.io/rh_ee_afilice`
- [x] No hardcoded credentials or API keys
- [x] No private domain names or IP addresses
- [x] `.gitignore` properly configured to exclude:
  - `node_modules/`
  - `dist/` (build artifacts)
  - IDE files
  - Temporary files

## Repository Information

- **GitHub Repository**: https://github.com/albertofilice/node-check-operator
- **Container Registry**: quay.io/rh_ee_afilice
- **Public Repository**: Yes

## Initial Commit

Recommended initial commit message:

```
Initial commit: Node Check Operator

- Kubernetes/OpenShift operator for comprehensive node monitoring
- OpenShift Console Plugin integration
- Prometheus metrics export
- Helm chart support
- Multi-architecture builds (AMD64/ARM64)
```

## GitHub Repository Settings

1. **General**:
   - Description: Use the description above
   - Topics: Add all topics listed above
   - Website: (Optional) Link to documentation

2. **Features**:
   - Enable Issues
   - Enable Discussions (optional)
   - Enable Wiki (optional)
   - Enable Projects (optional)

3. **Security**:
   - Enable dependency graph
   - Enable Dependabot alerts
   - Enable Dependabot security updates

4. **Actions**:
   - Enable GitHub Actions

## License

Choose and add an appropriate license file (e.g., `LICENSE` or `LICENSE.txt`).

Common choices:
- Apache-2.0 (recommended for operators)
- MIT
- GPL-3.0

