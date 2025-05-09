# NRDOT+ Internal Dev-Lab Project Purpose

NRDOT+ (New Relic Data Observability Telemetry Plus) Internal Dev-Lab v2.1.2 is a fully containerized, Kubernetes-native telemetry pipeline framework. Its primary purpose is to provide an internal-only sandbox for New Relic engineering, DevOps, and validation teams to design, deploy, and stress-test Function-Block (FB) modules in realistic environments without affecting production systems.

## Key Objectives

1. **Rapid Innovation:** Enable FB engineers to deploy or update modules (images or parameters) in <90s with zero downtime.
2. **Observability-Driven Feedback:** Provide end-to-end metrics, traces, and logs to iterate processing logic quickly.
3. **Dynamic Configuration:** Empower operators to adjust pipeline behavior at runtime via CRD and gRPC without restarts.
4. **Resilience by Design:** Validate circuit breakers, dead-letter queues, and durable buffering under failure scenarios.
5. **PII Protection Assurance:** Guarantee zero leakage of sensitive fields through audit metrics and schema enforcement.
6. **Reproducible Validation:** Automate SLO tests and chaos experiments to certify performance and reliability.
7. **Documentation & Operability:** Supply clear runbooks, backup/recovery scripts, and on-call guides to streamline lab operations.

## Key Components

The project is built around a chain of Function Blocks (FBs), each providing specific processing capabilities:

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

Additionally, it includes:
- **ConfigController**: Manages configuration for Function Blocks via CRD and gRPC
- **NRDotPlusPipeline CRD**: Defines the pipeline configuration
- **Observability Stack**: Prometheus metrics, W3C traces, JSON logs
- **Resilience Patterns**: Circuit breakers, DLQ, durable queue
- **Helm Charts**: For deployment in lab environments
