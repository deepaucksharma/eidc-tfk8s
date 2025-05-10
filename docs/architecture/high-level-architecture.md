---
title: "NRDOT+ Internal Dev-Lab Architecture"
updated: "2025-05-10"
toc: true
---

# NRDOT+ Internal Dev-Lab Architecture

## 1. System Overview

NRDOT+ is designed as a chain of Function Blocks (FBs), each providing a specific processing capability for telemetry data. The system follows a modular, containerized architecture deployed on Kubernetes.

```
┌─────────┐    ┌───────────┐    ┌───────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐    ┌────────────┐    ┌─────────┐    ┌─────────┐
│  FB-RX  │ -> │ FB-EN-HOST│ -> │ FB-EN-K8S │ -> │ FB-CL   │ -> │ FB-DP   │ -> │ FB-FS   │ -> │ FB-AGG  │ -> │ FB-GW-PRE  │ -> │ FB-GW   │ -> │ FB-DLQ  │
└─────────┘    └───────────┘    └───────────┘    └─────────┘    └─────────┘    └─────────┘    └─────────┘    └────────────┘    └─────────┘    └─────────┘
   │                                                                                                                                               ▲
   │                                                                                                                                               │
   └───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

## 2. Function Block Chain

### 2.1 Data Flow

1. **FB-RX**: Receives telemetry data from sources (OTLP/gRPC, OTLP/HTTP, Prometheus remote-write)
2. **FB-EN-HOST**: Enriches data with host-level metadata
3. **FB-EN-K8S**: Enriches data with Kubernetes metadata
4. **FB-CL**: Classifies and labels telemetry data
5. **FB-DP**: Deduplicates telemetry data
6. **FB-FS**: Filters and samples telemetry data
7. **FB-AGG**: Aggregates metrics
8. **FB-GW-PRE**: Pre-processes data before gateway export
9. **FB-GW**: Validates schema and exports data to configured destinations
10. **FB-DLQ**: Captures and manages failed processing through dead letter queues

### 2.2 Communication

Function Blocks communicate through:
- gRPC for data streaming
- ConfigController for configuration changes
- Shared Kubernetes resources for state management

## 3. Cross-Cutting Concerns

### 3.1 Observability

All Function Blocks expose:
- Prometheus metrics on port 2112
- Structured JSON logs
- W3C-compatible traces

### 3.2 Configuration

The ConfigController provides dynamic configuration to all Function Blocks via gRPC. Function Blocks:
1. Connect to ConfigController on startup
2. Watch for configuration changes
3. Apply configuration dynamically
4. Report configuration status

### 3.3 Resilience

Resilience mechanisms include:
- Circuit breakers for all downstream dependencies
- Dead letter queues for failed processing
- Kubernetes-native scaling and self-healing
- Chaos testing for validation

### 3.4 Security

The system enforces:
- PII detection and protection
- Schema validation for all data
- Isolated environments for development, testing, and staging

## 4. Component Diagrams

### 4.1 FB-RX Component

```
┌────────────────────────────────────────────────┐
│ FB-RX                                          │
│                                                │
│  ┌──────────────┐    ┌───────────────────┐    │
│  │ OTLP/gRPC    │    │ Protocol Adapter  │    │
│  │ Receiver     │───>│                   │    │
│  └──────────────┘    │                   │    │
│                      │                   │    │
│  ┌──────────────┐    │                   │    │
│  │ OTLP/HTTP    │───>│                   │───>│ To FB-EN-HOST
│  │ Receiver     │    │                   │    │
│  └──────────────┘    │                   │    │
│                      │                   │    │
│  ┌──────────────┐    │                   │    │
│  │ Prometheus   │───>│                   │    │
│  │ Receiver     │    └───────────────────┘    │
│  └──────────────┘                             │
└────────────────────────────────────────────────┘
```

### 4.2 FB-DP Component

```
┌────────────────────────────────────────────────┐
│ FB-DP                                          │
│                                                │
│  ┌──────────────┐    ┌───────────────────┐    │
│  │ Data         │    │ Deduplication     │    │
│  │ Receiver     │───>│ Logic             │    │
│  └──────────────┘    │                   │    │
│                      │                   │───>│ To FB-FS
│                      │                   │    │
│  ┌──────────────┐    │                   │    │
│  │ BadgerDB     │<──>│                   │    │
│  │ Storage      │    │                   │    │
│  └──────────────┘    └───────────────────┘    │
│                                                │
└────────────────────────────────────────────────┘
```

## 5. Deployment Architecture

The system is deployed as Kubernetes resources:
- Each Function Block is a separate Deployment
- Helm charts provide templated deployment
- ConfigMaps store configuration
- Custom Resource Definitions (CRDs) define pipeline configurations
- Services expose endpoints
- Health checks and readiness probes ensure availability

## 6. Future Architecture Considerations

- Enhanced stateful processing capabilities
- Multi-cluster federation
- ML-based anomaly detection
- Extended PII protection measures
