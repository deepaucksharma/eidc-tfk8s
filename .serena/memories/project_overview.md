# NRDOT+ Internal Dev-Lab Project Overview

## Project Purpose

NRDOT+ (New Relic Data Observability Telemetry Plus) Internal Dev-Lab v2.1.2 is a Kubernetes-native telemetry pipeline framework for New Relic engineering teams. It provides a sandbox environment for designing, deploying, and stress-testing Function-Block (FB) modules without affecting production systems.

## Architecture

The system is built around a chain of Function Blocks (FBs), each providing a specific processing capability:

1. **FB-RX**: Data ingestion (OTLP/gRPC, OTLP/HTTP, Prometheus remote-write)
2. **FB-EN-HOST**: Host-level enrichment
3. **FB-EN-K8S**: Kubernetes metadata enrichment
4. **FB-CL**: Classification and PII handling
5. **FB-DP**: Deduplication
6. **FB-FS**: Filtering and sampling
7. **FB-AGG**: Aggregation
8. **FB-GW-PRE**: Pre-gateway queueing
9. **FB-GW**: Schema enforcement and export
10. **FB-DLQ**: Dead letter queue management

## Tech Stack

- **Backend**: Go 1.21+
- **API**: gRPC with Protocol Buffers
- **Containerization**: Docker
- **Orchestration**: Kubernetes
- **Observability**: Prometheus, OpenTelemetry, structured logging
- **Configuration**: Custom Resource Definitions (CRDs)
- **Storage**: LevelDB (DLQ, GW-PRE), BadgerDB (DP)
- **CI/CD**: GitHub Actions

## Project Structure

- **cmd/**: Command-line entry points for executables
  - **configcontroller/**: Kubernetes controller for CRDs and config management
  - **dlq-replay/**: Utility for replaying data from the dead letter queue
  - **fb/**: Main executables for each Function Block

- **pkg/**: Core function block implementations and shared code
  - **api/**: Protocol buffer definitions
  - **fb/**: Function block implementations

- **internal/**: Internal packages not intended for external use
  - **common/**: Shared utilities for logging, tracing, metrics, etc.
  - **config/**: Configuration loading and management

- **deploy/**: Deployment resources
  - **helm/**: Helm charts
  - **k8s/**: Kubernetes manifests
  - **scripts/**: Backup and other operational scripts

## Key Features

- Dynamic configuration via CRD and gRPC
- Rapid deployment with zero downtime
- Comprehensive observability (metrics, traces, logs)
- Circuit breakers and resilience patterns
- PII protection and schema enforcement
- Dead letter queue for error handling
