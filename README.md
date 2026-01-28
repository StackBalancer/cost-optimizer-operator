# cost-optimizer-operator
Kubernetes operator for automatic CPU and memory rightsizing based on workload policies.

## Description
### Status
**Phase 1 Complete** - Controller scaffolding with CRD and initial reconciliation loop
**Phase 2 Complete** - Metrics Collection & Analysis

**Completed Features:**
- CRD with resource optimization policies
- Controller scaffolding with reconciliation loop
- Metrics collection from Kubernetes metrics API
- Usage analysis and recommendation engine
- Status reporting with resource recommendations
- Event recording for optimization activities

**Current Phase:** Phase 3 - Resource Optimization Engine

**Roadmap:**
- **Phase 3**: Implement actual resource patching
- **Phase 4**: Real-time adjustment without pod restarts
- **Phase 5**: Advanced ML-based predictions and cost analytics

## Architecture

The operator follows Kubernetes operator best practices with:

- **Level-triggered reconciliation**: Continuously drives actual state toward desired state
- **Idempotent operations**: Safe to run multiple times
- **Metrics-driven decisions**: Uses actual usage data instead of guesses
- **Policy-based optimization**: Configurable CPU/memory policies per workload

### Components

1. **ResourceOptimizer CRD**: Defines optimization policies for target workloads
2. **Metrics Collector**: Gathers CPU/memory usage from Kubernetes metrics API
3. **Analyzer**: Generates resource recommendations based on usage patterns
4. **Controller**: Orchestrates the optimization lifecycle

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

The status will show:
- Current conditions (DeploymentReady, OptimizationReady)
- Resource recommendations (CPU/Memory requests and limits)
- Confidence level and reasoning

## Development

### Prerequisites
- go version v1.24.6+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/cost-optimizer-operator:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/cost-optimizer-operator:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following the options to release and provide this solution to the users.

### By providing a bundle with all YAML files

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/cost-optimizer-operator:tag
```

**NOTE:** The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without its
dependencies.

2. Using the installer

Users can just run 'kubectl apply -f <URL for YAML BUNDLE>' to install
the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/cost-optimizer-operator/<tag or branch>/dist/install.yaml
```

### By providing a Helm Chart

1. Build the chart using the optional helm plugin

```sh
kubebuilder edit --plugins=helm/v2-alpha
```

2. See that a chart was generated under 'dist/chart', and users
can obtain this solution from there.

**NOTE:** If you change the project, you need to update the Helm Chart
using the same command above to sync the latest changes. Furthermore,
if you create webhooks, you need to use the above command with
the '--force' flag and manually ensure that any custom configuration
previously added to 'dist/chart/values.yaml' or 'dist/chart/manager/manager.yaml'
is manually re-applied afterwards.

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

