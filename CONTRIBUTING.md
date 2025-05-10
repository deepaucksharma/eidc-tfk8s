# Contributing to NRDOT+ Internal Dev-Lab

Thank you for your interest in contributing to the NRDOT+ Internal Dev-Lab project! This document provides guidelines and instructions for development.

## Development Environment Setup

### Prerequisites

- Go 1.21 or later
- Docker and Docker Compose
- Kubernetes cluster (k3d, minikube, or a remote cluster)
- kubectl configured to access your cluster
- Helm 3.x

### Local Setup

1. Clone the repository:
   ```bash
   git clone git@github.com:newrelic/nrdot-internal-devlab.git
   cd nrdot-internal-devlab
   ```

2. Install Go dependencies:
   ```bash
   go mod download
   ```

3. Set up a local Kubernetes cluster using k3d:
   ```bash
   k3d cluster create nrdot-devlab --agents 3 --registry-create registry.localhost:5000
   ```

4. Install the CRDs:
   ```bash
   kubectl apply -f deploy/k8s/crds/
   ```

## Building and Testing

### Building Function Blocks

Each Function Block is built as a separate Docker image. To build all Function Blocks:

```bash
# From the project root
./scripts/build-all.sh
```

To build a specific Function Block:

```bash
# For example, to build FB-RX
docker build -t nrdot-internal-devlab/fb-rx:latest -f pkg/fb/rx/Dockerfile .
```

### Running Tests
Run unit tests:

```bash
go test ./...
```

Run integration tests (requires a Kubernetes cluster):

```bash
./scripts/run-integration-tests.sh
```

### Updating Test Matrix and Documentation

When implementing or modifying a requirement, you must update the test matrix in `docs/appendices/appendix-a-test-matrix.md` and link your test to the appropriate row.

#### How to Update the Test Matrix

1. Open `docs/appendices/appendix-a-test-matrix.md`
2. Find the row corresponding to the requirement you're implementing or modifying
3. Update the Status and Test(s) columns
4. If you're adding a new test, link it in the Test(s) column

Example row:
```
| FR-DP-04 | P0 | PII Hashing & Sanitization | Passing | [deploy/test/tf-k8s/schema_pii_enforcement/FR-DP-04_pii_hashing_all_fields.yaml](../../../deploy/test/tf-k8s/schema_pii_enforcement/FR-DP-04_pii_hashing_all_fields.yaml) |
```

#### Writing TF-K8s Scenarios

TF-K8s scenarios are stored in the `deploy/test/tf-k8s/` directory, organized by suite. When writing a new scenario:

1. Create a new YAML file in the appropriate suite directory
2. Name the file according to the convention: `<id>_<short-desc>.yaml` (e.g., `FR-DP-04_pii_hashing_all_fields.yaml`)
3. Include validation steps to verify the requirement is met

#### Running the Test Matrix Builder

After updating the test matrix, you can run the test matrix builder to update the PRD:

```bash
python tools/test-matrix-builder/test_matrix_builder.py
```

This will update the PRD with the latest test matrix. The CI pipeline will also run this automatically.
### Local Deployment

Deploy the entire stack using Docker Compose:

```bash
docker-compose up -d
```

Deploy the entire stack using Helm:

```bash
helm install nrdot-devlab ./deploy/helm --values ./deploy/helm/values-lab.yaml
```

## Development Workflow

1. **Create a Feature Branch:**
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Implement your changes**

3. **Run Tests:**
   - Ensure unit tests pass
   - Run SLO validation tests

4. **Submit a Pull Request:**
   - Include a clear description of changes
   - Link to relevant issues
   - Ensure CI pipeline passes

## Function Block Development

### Creating a New Function Block

1. Create a new directory in `pkg/fb/` for your Function Block
2. Implement the required interfaces (see `pkg/fb/interfaces.go`)
3. Add a Dockerfile and other necessary files
4. Update the Helm chart to include your new Function Block

### Function Block Interface

Each Function Block must implement the `FunctionBlock` interface and the `ChainPushService` gRPC service.

### Configuration

Function Blocks receive configuration from the ConfigController via gRPC. Each FB should:

1. Connect to the ConfigController on startup
2. Watch for configuration changes
3. Apply configuration dynamically when possible
4. Report configuration status back to the ConfigController

## Code Style and Standards

- Follow the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use [gofmt](https://golang.org/cmd/gofmt/) to format code
- Document all public functions and types
- Add unit tests for new functionality

## Observability

All Function Blocks must expose:

- Prometheus metrics on port 2112
- Health and readiness endpoints
- Structured JSON logs
- W3C-compatible traces

## Additional Resources

- [Architecture Documentation](./docs/architecture/)
- [API Documentation](./docs/api/)
- [Runbooks](./docs/runbooks/)
