# FB-EN-HOST Function Block

## Overview

The Host Enrichment Function Block (FB-EN-HOST) is responsible for enriching telemetry data with host-level information. It sits between the Receiver (FB-RX) and the Kubernetes Enrichment (FB-EN-K8S) blocks in the NRDOT+ pipeline.

## Functionality

FB-EN-HOST performs the following functions:

1. **Host Metadata Enrichment**: Adds host information such as hostname, IP address, OS, architecture, and system resources to metrics.
2. **Process Metadata Enrichment**: Adds process information from `/proc/[pid]/stat` to metrics that contain process IDs.
3. **Efficient Caching**: Maintains a cache of host and process information to reduce system calls and improve performance.

## Configuration

FB-EN-HOST is configured through the NRDotPlusPipeline CRD. The configuration is streamed to the function block via the ConfigController.

Example configuration:

```yaml
apiVersion: nrdot.newrelic.com/v1
kind: NRDotPlusPipeline
metadata:
  name: default-pipeline
spec:
  en-host:
    common:
      log_level: "info"
      metrics_enabled: true
      tracing_enabled: true
      trace_sampling_ratio: 0.1
      next_fb: "fb-en-k8s:5000"
      circuit_breaker:
        error_threshold_percentage: 50
        open_state_seconds: 30
        half_open_request_threshold: 5
    cache_ttl: "10m"
    proc_stats_enabled: true
```

## Performance Considerations

- The host info cache TTL can be adjusted based on the volatility of the data. For relatively static data like hostname and OS, a longer TTL is appropriate. For more dynamic data like resource usage, a shorter TTL might be needed.
- The component is designed to be scalable and can be deployed with multiple replicas for high availability.

## Metrics

FB-EN-HOST exposes the following Prometheus metrics:

- Standard FB metrics (batch counts, latencies, etc.)
- Cache-specific metrics (hit rate, size, etc.)
- Host metrics (CPU, memory, etc.)

## Tracing

FB-EN-HOST participates in distributed tracing, producing spans for batch processing and forwarding. It propagates trace context to the next FB in the chain.
