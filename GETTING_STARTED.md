# Getting Started with NRDOT+ Internal Dev-Lab

This guide will help you get started with the NRDOT+ Internal Dev-Lab project.

## Project Overview

NRDOT+ (New Relic Data Observability Telemetry Plus) Internal Dev-Lab v2.1.2 is a fully containerized, Kubernetes-native telemetry pipeline framework. It provides an internal-only sandbox for New Relic engineering, DevOps, and validation teams to design, deploy, and stress-test Function-Block (FB) modules in realistic environments without affecting production.

## Prerequisites

- Go 1.21 or later
- Docker and Docker Compose
- Kubernetes cluster (k3d, minikube, or a remote cluster)
- kubectl configured to access your cluster
- Helm 3.x

## Quick Start

### 1. Clone the Repository

```bash
git clone https://github.com/newrelic/nrdot-internal-devlab.git
cd nrdot-internal-devlab
```

### 2. Build the Project

```bash
# Download Go dependencies
go mod download

# Build all Function Blocks
docker-compose build
```

### 3. Start the Development Environment

```bash
# Start all components with Docker Compose
docker-compose up -d

# Check the status
docker-compose ps
```

### 4. Deploy to Kubernetes

```bash
# Create a local Kubernetes cluster
k3d cluster create nrdot-devlab --agents 3

# Install the Custom Resource Definition
kubectl apply -f deploy/k8s/crds/nrdotpluspipeline.yaml

# Deploy with Helm
helm install nrdot-devlab ./deploy/helm --values ./deploy/helm/values-lab.yaml

# Check the status
kubectl get pods
```

### 5. Send Test Data

```bash
# Use the telemetry-generator to send test data
go run cmd/telemetry-generator/main.go --target=localhost:4317
```

### 6. Access Dashboards

- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000 (admin/admin)

## Project Structure

- `cmd/` - Command-line entry points for all executables
- `pkg/` - Core function block implementations and shared code
- `internal/` - Internal packages not intended for external use
- `deploy/` - Deployment resources (Helm charts, K8s manifests, scripts)
- `docs/` - Documentation

## Function Blocks

The NRDOT+ pipeline consists of the following Function Blocks:

1. **FB-RX**: Receives telemetry data via OTLP/gRPC, OTLP/HTTP, and Prometheus remote-write
2. **FB-EN-HOST**: Enriches data with host-level information
3. **FB-EN-K8S**: Enriches data with Kubernetes metadata
4. **FB-CL**: Classifies and handles PII data
5. **FB-DP**: Deduplicates data
6. **FB-FS**: Filters and samples data
7. **FB-AGG**: Aggregates data
8. **FB-GW-PRE**: Pre-processes data before gateway export
9. **FB-GW**: Exports data to a destination
10. **FB-DLQ**: Handles dead-letter queue for failed processing

## Development Workflow

1. Implement changes to one or more Function Blocks
2. Run unit tests (`go test ./...`)
3. Build and test locally with Docker Compose
4. Deploy to a test Kubernetes cluster
5. Run integration tests
6. Submit a pull request

## Additional Resources

- See [CONTRIBUTING.md](./CONTRIBUTING.md) for detailed development guidelines
- See [docs/architecture](./docs/architecture) for architecture documentation
- See [docs/runbooks](./docs/runbooks) for operational guides
- Test Matrix: [docs/appendices/appendix-a-test-matrix.md](./docs/appendices/appendix-a-test-matrix.md)
- E2E Test Program: [docs/appendices/appendix-i-e2e-program.md](./docs/appendices/appendix-i-e2e-program.md)
