# NRDOT+ Internal Dev-Lab Tech Stack

## Programming Languages
- **Go (Golang)**: Primary programming language (version 1.21+)

## Container and Orchestration
- **Docker**: Container runtime for all components
- **Kubernetes**: Orchestration platform (v1.28+)
- **Helm**: Package management for Kubernetes deployments

## Storage
- **BadgerDB**: Used by FB-DP for deduplication storage
- **LevelDB**: Used by FB-GW-PRE and FB-DLQ for durable queueing
- **Kafka** (optional): Alternative backend for FB-DLQ in testing scenarios

## APIs and Communication
- **gRPC**: Inter-FB communication and config streaming
- **Protocol Buffers**: Data serialization for gRPC interfaces
- **Kubernetes CRDs**: Pipeline configuration via custom resources

## Observability
- **Prometheus**: Metrics collection and storage
- **OpenTelemetry (OTLP)**: For distributed tracing
- **Structured JSON Logging**: For consistent log format

## Resilience Patterns
- **Circuit Breakers**: For handling failures gracefully
- **Dead Letter Queue (DLQ)**: For processing failed messages
- **Durable Queueing**: For data persistence during outages

## Build and CI/CD
- **Go Modules**: Dependency management
- **GitHub Actions/Jenkins**: CI/CD pipeline (TF-K8s v1.0.2 test suite)

## Libraries and Frameworks
- **client-go**: Kubernetes client for Go
- **controller-runtime**: For implementing Kubernetes controllers
- **prometheus/client_golang**: Prometheus client for Go
- **opentelemetry-go**: OpenTelemetry client for Go
- **grpc-go**: gRPC implementation for Go
