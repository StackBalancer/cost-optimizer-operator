# cost-optimizer-operator

Kubernetes operator for automatic CPU and memory rightsizing based on workload policies.

## Features

- CRD with resource optimization policies
- Controller scaffolding with reconciliation loop
- Metrics collection from Kubernetes metrics API
- Usage analysis and recommendation engine
- Status reporting with resource recommendations
- Event recording for optimization activities

## Architecture

The operator follows Kubernetes operator best practices with:

- **Level-triggered reconciliation** — continuously drives actual state toward desired state
- **Idempotent operations** — safe to run multiple times
- **Metrics-driven decisions** — uses actual usage data instead of guesses
- **Policy-based optimization** — configurable CPU/memory policies per workload

### Components

1. **ResourceOptimizer CRD** — defines optimization policies for target workloads
2. **Metrics Collector** — gathers CPU/memory usage from Kubernetes metrics API
3. **Analyzer** — generates resource recommendations based on usage patterns
4. **Controller** — orchestrates the optimization lifecycle

## Usage

### 1. Deploy a test workload
```bash
kubectl apply -f k8s/test-deployment.yaml
```

### 2. Create a ResourceOptimizer
```yaml
apiVersion: optimization.stackbalancer.io/v1
kind: ResourceOptimizer
metadata:
  name: api-service-optimizer
  namespace: maintenance
spec:
  targetRef:
    kind: Deployment
    name: api-service
    namespace: production
  policy:
    cpu:
      min: "200m"
      max: "800m"
      targetUtilization: 70
    memory:
      bufferPercent: 20
```

### 3. Monitor optimization status
```bash
kubectl get resourceoptimizer -n <namespace> api-service-optimizer -o yaml
```

The status will show current conditions (`DeploymentReady`, `OptimizationReady`), resource recommendations (CPU/memory requests and limits), confidence level, and reasoning.

## Development

### Prerequisites
- Go v1.24.6+
- Docker 17.03+
- kubectl v1.11.3+
- Access to a Kubernetes v1.11.3+ cluster

### Deploy to cluster

Build and push the image:
```sh
make docker-build docker-push IMG=<some-registry>/cost-optimizer-operator:tag
```

Install CRDs:
```sh
make install
```

Deploy the controller:
```sh
make deploy IMG=<some-registry>/cost-optimizer-operator:tag
```

Apply sample resources:
```sh
kubectl apply -k config/samples/
```

### Uninstall

```sh
kubectl delete -k config/samples/
make uninstall
make undeploy
```

Run `make help` for all available targets.

## License

Copyright 2025. Licensed under the [Apache License, Version 2.0](http://www.apache.org/licenses/LICENSE-2.0).
