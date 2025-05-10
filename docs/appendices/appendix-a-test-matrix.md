---
title: "Appendix A: Requirement to Test Matrix"
updated: "2025-05-10"
toc: false
---

# Requirement to Test Matrix

| ID | Priority | Requirement | Status | Test(s) |
|----|----------|-------------|--------|---------|
| FR-RX-01 | P0 | Data Ingestion | Passing | [deploy/test/tf-k8s/ingestion/data_formats.yaml](../../../deploy/test/tf-k8s/ingestion/data_formats.yaml) |
| FR-EN-01 | P1 | Host Enrichment | Passing | [deploy/test/tf-k8s/enrichment/host_metadata.yaml](../../../deploy/test/tf-k8s/enrichment/host_metadata.yaml) |
| FR-EN-02 | P1 | Kubernetes Enrichment | Passing | [deploy/test/tf-k8s/enrichment/k8s_metadata.yaml](../../../deploy/test/tf-k8s/enrichment/k8s_metadata.yaml) |
| FR-CL-01 | P1 | Telemetry Classification | Passing | [deploy/test/tf-k8s/classification/labeling.yaml](../../../deploy/test/tf-k8s/classification/labeling.yaml) |
| FR-DP-01 | P1 | Deduplication | Passing | [deploy/test/tf-k8s/deduplication/basic_dedup.yaml](../../../deploy/test/tf-k8s/deduplication/basic_dedup.yaml) |
| FR-DP-02 | P2 | Stateful Detection | In CI | [deploy/test/tf-k8s/deduplication/stateful_dedup.yaml](../../../deploy/test/tf-k8s/deduplication/stateful_dedup.yaml) |
| FR-DP-03 | P2 | Optimized Storage | Passing | [deploy/test/tf-k8s/deduplication/badgerdb_perf.yaml](../../../deploy/test/tf-k8s/deduplication/badgerdb_perf.yaml) |
| FR-DP-04 | P0 | PII Hashing & Sanitization | Passing | [deploy/test/tf-k8s/schema_pii_enforcement/FR-DP-04_pii_hashing_all_fields.yaml](../../../deploy/test/tf-k8s/schema_pii_enforcement/FR-DP-04_pii_hashing_all_fields.yaml) |
| FR-FS-01 | P1 | Configurable Filtering | Passing | [deploy/test/tf-k8s/filtering/pattern_matching.yaml](../../../deploy/test/tf-k8s/filtering/pattern_matching.yaml) |
| FR-FS-02 | P2 | Sampling Control | Passing | [deploy/test/tf-k8s/filtering/sampling_rates.yaml](../../../deploy/test/tf-k8s/filtering/sampling_rates.yaml) |
| FR-AGG-01 | P2 | Metric Aggregation | In CI | [deploy/test/tf-k8s/aggregation/metrics_agg.yaml](../../../deploy/test/tf-k8s/aggregation/metrics_agg.yaml) |
| FR-GW-01 | P0 | Schema Enforcement | Passing | [deploy/test/tf-k8s/schema_pii_enforcement/schema_validation.yaml](../../../deploy/test/tf-k8s/schema_pii_enforcement/schema_validation.yaml) |
| FR-GW-02 | P1 | Export Endpoints | Passing | [deploy/test/tf-k8s/gateway/export_endpoints.yaml](../../../deploy/test/tf-k8s/gateway/export_endpoints.yaml) |
| FR-DLQ-01 | P1 | Dead Letter Processing | Passing | [deploy/test/tf-k8s/dlq/capture_failures.yaml](../../../deploy/test/tf-k8s/dlq/capture_failures.yaml) |
| FR-DLQ-02 | P2 | Replay Capability | In CI | [deploy/test/tf-k8s/dlq/replay_messages.yaml](../../../deploy/test/tf-k8s/dlq/replay_messages.yaml) |
| NFR-OPS-01 | P0 | Rapid Deployment | Passing | [deploy/test/tf-k8s/operations/deployment_speed.yaml](../../../deploy/test/tf-k8s/operations/deployment_speed.yaml) |
| NFR-OBS-01 | P0 | Complete Instrumentation | Passing | [deploy/test/tf-k8s/observability/instrumentation_coverage.yaml](../../../deploy/test/tf-k8s/observability/instrumentation_coverage.yaml) |
| NFR-CFG-01 | P1 | Dynamic Configuration | Passing | [deploy/test/tf-k8s/configuration/runtime_updates.yaml](../../../deploy/test/tf-k8s/configuration/runtime_updates.yaml) |
| NFR-RSL-01 | P0 | Circuit Breaking | Passing | [deploy/test/tf-k8s/resilience/circuit_breaker.yaml](../../../deploy/test/tf-k8s/resilience/circuit_breaker.yaml) |
| NFR-RSL-02 | P1 | DLQ Integration | Passing | [deploy/test/tf-k8s/dlq/integration_all_blocks.yaml](../../../deploy/test/tf-k8s/dlq/integration_all_blocks.yaml) |
| NFR-RSL-03 | P2 | Resource Limits | Passing | [deploy/test/tf-k8s/operations/resource_constraints.yaml](../../../deploy/test/tf-k8s/operations/resource_constraints.yaml) |
| NFR-SEC-01 | P0 | Zero PII Leakage | Passing | [deploy/test/tf-k8s/schema_pii_enforcement/pii_leakage_prevention.yaml](../../../deploy/test/tf-k8s/schema_pii_enforcement/pii_leakage_prevention.yaml) |
| NFR-SEC-02 | P2 | Isolated Environments | Not Started | [deploy/test/tf-k8s/security/environment_isolation.yaml](../../../deploy/test/tf-k8s/security/environment_isolation.yaml) |
| NFR-TEST-01 | P1 | SLO Validation | Passing | [deploy/test/tf-k8s/slo/validation.yaml](../../../deploy/test/tf-k8s/slo/validation.yaml) |
| NFR-TEST-02 | P1 | Chaos Resilience | In CI | [deploy/test/tf-k8s/resilience/chaos_tests.yaml](../../../deploy/test/tf-k8s/resilience/chaos_tests.yaml) |