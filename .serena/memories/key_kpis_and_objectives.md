# NRDOT+ Internal Dev-Lab KPIs and Objectives

## Key Performance Indicators (KPIs)

| KPI ID | Metric                    | Target             | PromQL/Measurement                                                           |
| ------ | ------------------------- | ------------------ | ---------------------------------------------------------------------------- |
| KPI-1  | FB P95 latency            | <50 ms             | `histogram_quantile(0.95, rate(fb_grpc_latency_seconds_bucket[5m]))`         |
| KPI-2  | End-to-end latency        | <1000 ms           | `histogram_quantile(0.95, rate(pipeline_latency_seconds_bucket[5m]))`        |
| KPI-3  | Data volume               | ≤50 MB/day/host    | `sum(rate(fb_gw_export_bytes_total[5m])) by (host)`                          |
| KPI-4  | Series cardinality        | ≤100k series/host  | `count(fb_dp_unique_series_total) by (host)`                                 |
| KPI-5  | DLQ rate                  | <0.02%             | `rate(fb_dlq_store_total[5m])/rate(fb_gw_export_total[5m])`                  |
| KPI-6  | Circuit Breaker open time | <2 min/hr          | `sum(rate(fb_cb_open_seconds_total[1h]))`                                    |
| KPI-7  | PII leaks                 | 0                  | `NRQL: SELECT count(*) FROM Metric WHERE command_line IS NOT NULL`           |
| KPI-8  | Config rollout success    | ≥98%               | `cc_stream_ack_total{status="OK"} / cc_stream_push_total`                    |

## Project Objectives

1. **Rapid Innovation:** 
   - Deploy or update modules in <90s with zero downtime
   - Measured via CI/CD pipeline timing and deployment logs

2. **Observability-Driven Feedback:** 
   - 100% of FBs emit required metrics, traces, and logs
   - All KPIs visible in Grafana dashboards

3. **Dynamic Configuration:** 
   - ≥98% config pushes ACKed by FBs in ≤30s
   - CRD changes reflect in all FBs without restarts

4. **Resilience by Design:** 
   - No data loss during simulated outages
   - Circuit breaker pattern properly limits impact of failures
   - DLQ captures and enables replay of failed messages

5. **PII Protection:** 
   - Zero PII leakage in all validation runs
   - All sensitive fields properly hashed

6. **Reproducible Validation:** 
   - TF-K8s test suite green 3× consecutively on CI
   - SLO and resilience tests automated and reliable

7. **Documentation & Operability:** 
   - Complete runbooks for all operations
   - Backup/restore procedures validated quarterly

## Acceptance Criteria

1. **Deploy:** `helm install` on fresh k3d → all pods Ready in <15 min
2. **Config rollout:** CRD changes reflect in FB logs & CRD.status in <30s for ≥98% tries
3. **SLOs:** TF-SLO and ResilienceSuite pass 3X
4. **PII:** PII-Scrub-Test → 0 leaks
5. **Chaos:** Kill FB-GW pod, backlog drains on restart, no data lost
