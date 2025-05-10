# NRDOT+ Internal Dev-Lab v2.1.2

A fully containerized, Kubernetes-native telemetry pipeline framework for New Relic engineering teams.

## Overview

NRDOT+ (New Relic Data Observability Telemetry Plus) Internal Dev-Lab v2.1.2 provides a sandbox environment for designing, deploying, and stress-testing Function-Block (FB) modules in realistic environments without affecting production systems.

## Key Features

- **Rapid Innovation:** Deploy or update modules in <90s with zero downtime
- **Observability-Driven Feedback:** End-to-end metrics, traces, and logs
- **Dynamic Configuration:** Adjust pipeline behavior at runtime via CRD and gRPC
- **Resilience by Design:** Circuit breakers, dead-letter queues, and durable buffering
- **PII Protection:** Zero leakage of sensitive fields through audit metrics and schema enforcement
- **Reproducible Validation:** Automated System-E2E (core/advanced) tests and chaos experiments. See [Appendix I: End-to-End Test Program](./docs/appendices/appendix-i-e2e-program.md)
- **Documentation & Operability:** Clear runbooks and operational guides
## Architecture

NRDOT+ is built around a chain of Function Blocks (FBs), each providing a specific processing capability:

- **FB-RX**: Data ingestion (OTLP/gRPC, OTLP/HTTP, Prometheus remote-write)
- **FB-EN-HOST**: Host-level enrichment
- **FB-EN-K8S**: Kubernetes metadata enrichment
- **FB-CL**: Classification and PII handling
- **FB-DP**: Deduplication
- **FB-FS**: Filtering and sampling
- **FB-AGG**: Aggregation
- **FB-GW-PRE**: Pre-gateway queueing
- **FB-GW**: Schema enforcement and export
- **FB-DLQ**: Dead letter queue management

## Getting Started

### Prerequisites

- Kubernetes v1.28+
- Access to internal Docker registry
- Prometheus & OTLP backend
- (Optional) Single-broker Kafka for extended DLQ tests

### Deployment

```bash
# Deploy using Helm
helm install nrdot-devlab ./deploy/helm --values ./deploy/helm/values-lab.yaml
```

## Development

See [CONTRIBUTING.md](./CONTRIBUTING.md) for development setup and guidelines.

## Documentation

- Architecture diagrams and descriptions are in [docs/architecture](./docs/architecture)
- Operational runbooks can be found in [docs/runbooks](./docs/runbooks)
- API documentation is available in [docs/api](./docs/api)
- Test Matrix: [docs/appendices/appendix-a-test-matrix.md](./docs/appendices/appendix-a-test-matrix.md)
- Test status updates are available in [docs/appendices/test-status-updates.md](./docs/appendices/test-status-updates.md)
- The Product Requirements Document (PRD) is available in [docs/prd](./docs/prd)
