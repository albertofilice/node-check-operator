# CRD Manual Updates

## Important Note

**This CRD is maintained manually and is NOT auto-generated.**

When adding new checks or modifying the NodeCheck API:

1. **Update the Go types** in `api/v1alpha1/nodecheck_types.go`
2. **Update this CRD manually** in `config/crd/bases/nodecheck.openshift.io_nodechecks.yaml`
3. **Update the backend API handlers** in `pkg/dashboard/api/handlers.go` (structures and conversion logic)
4. **Update the frontend TypeScript interfaces** in `console-plugin/src/pages/NodeCheckOverview.tsx`
5. **Update the controller** in `controllers/nodecheck_executor_controller.go` (execution and mapping)
6. **Update examples** in `config/samples/` and `examples/`
7. **Update install.sh** script if it creates example NodeChecks

## Checklist for Adding New Checks

- [ ] Add field to Go struct in `api/v1alpha1/nodecheck_types.go`
- [ ] Add field to CRD spec in `config/crd/bases/nodecheck.openshift.io_nodechecks.yaml`
- [ ] Add field to CRD status (if needed) in `config/crd/bases/nodecheck.openshift.io_nodechecks.yaml`
- [ ] Add check execution in `controllers/nodecheck_executor_controller.go`
- [ ] Add result mapping in `controllers/nodecheck_executor_controller.go`
- [ ] Add to API structures in `pkg/dashboard/api/handlers.go`
- [ ] Add to API conversion in `pkg/dashboard/api/handlers.go`
- [ ] Add to frontend interface in `console-plugin/src/pages/NodeCheckOverview.tsx`
- [ ] Add to frontend rendering in `console-plugin/src/pages/NodeCheckOverview.tsx`
- [ ] Update examples in `config/samples/` and `examples/`
- [ ] Update `scripts/install.sh` if it creates example NodeChecks

