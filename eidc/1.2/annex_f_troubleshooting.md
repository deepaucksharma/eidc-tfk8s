# Annex F – Troubleshooting Guide

This troubleshooting guide provides solutions for common issues that may arise with NRDOT+ v1.2 and its components.

---

## F-1 · Diagnostic Tools

### Log Collection

Collect all relevant logs from the collector and edge-probe:

```bash
kubectl logs -n nrdotplus-system -l app=nrdotplus-collector -c otelcol --tail=5000 > collector.log
kubectl logs -n nrdotplus-system -l app=nrdotplus-edge-probe --tail=1000 > edge-probe.log
```

### Metrics Inspection

Use the built-in zpages extension to inspect metrics flow:

```bash
kubectl port-forward -n nrdotplus-system svc/nrdotplus-collector 55679:55679
# Then open http://localhost:55679/debug/pipelinesz in your browser
```

### Core Collectors

These tools are essential for diagnosing issues:

- zpages metrics flow
- otelcol_processor_* metrics
- otelcol_exporter_* metrics
- `/proc` access check script

---

## F-2 · Common Issues & Solutions

### 1. Missing `process.start_time_ns`

**Symptoms:**
- Duplicate process entries
- Log warnings about "fallback deduplication"
- High series count

**Solutions:**
1. Check Permissions:
   ```bash
   kubectl exec -it -n nrdotplus-system deploy/nrdotplus-collector -- ls -la /proc/1/stat
   ```
   If permission denied, adjust security context in deployment.

2. Verify PID read capability:
   ```bash
   kubectl exec -it -n nrdotplus-system deploy/nrdotplus-collector -- \
     /opt/otelcol-contrib/bin/pid_time_check -pid 1
   ```
   Should return nanosecond timestamp.

### 2. High Memory Usage

**Symptoms:**
- Memory_limiter dropping data
- OOMKilled pods
- Warning logs about "approaching memory_limit"

**Solutions:**
1. Enable delta temporality:
   ```yaml
   processors:
     cumulativetodelta:
       include:
         match_type: regexp
         metric_names: [".*"]
   ```

2. Increase sampling rate:
   ```yaml
   # In filter/intelligent_filter_sample
   - value_double < 0.001  # Increase threshold
   ```

3. Adjust memory_limiter:
   ```yaml
   memory_limiter:
     check_interval: 5s
     limit_percentage: 80
     spike_limit_percentage: 15
   ```

### 3. OTLP Export Failures

**Symptoms:**
- Export errors in logs
- Data not appearing in New Relic
- "Failed to upload" messages

**Solutions:**
1. Check connectivity:
   ```bash
   kubectl exec -it -n nrdotplus-system deploy/nrdotplus-collector -- \
     curl -v https://otlp.nr-data.net:4318/health
   ```

2. Verify API key permissions

3. Enable retry queue:
   ```yaml
   exporters:
     otlphttp:
       retry_on_failure:
         enabled: true
         initial_interval: 5s
         max_interval: 30s
         max_elapsed_time: 5m
   ```

### 4. Edge-Probe Not Working

**Symptoms:**
- No metrics from Edge-Probe
- "Connection refused" in collector logs

**Solutions:**
1. Check endpoint configuration:
   ```bash
   kubectl exec -it -n nrdotplus-system deploy/nrdotplus-edge-probe -- \
     curl localhost:9999/metrics
   ```

2. Verify seccomp profile:
   ```bash
   kubectl describe pod -n nrdotplus-system -l app=nrdotplus-edge-probe
   # Look for seccomp annotations
   ```

3. Check capabilities:
   ```yaml
   securityContext:
     capabilities:
       add: ["BPF", "PERFMON"]  # Ensure these are present
   ```

---

## F-3 · Performance Tuning

### Collector CPU Optimization

1. Batch settings:
   ```yaml
   batch:
     send_batch_size: 1000
     timeout: 1s
   ```

2. Worker counts:
   ```yaml
   # For high CPU hosts
   otlphttp:
     sending_queue:
       num_consumers: 4
   ```

### Memory Optimization

1. Label cardinality settings:
   ```yaml
   # In filter/intelligent_filter_sample
   - attributes["process.custom.classification"] == "other_ephemeral" and
     Mod(Hash(attributes["process.pid"]), 10) != 0  # 1-in-10 sampling
   ```

2. Schema enforcement:
   ```yaml
   # In filter/final_schema
   metrics:
     metric:
       - not SchemaMatch(name)  # Drop non-schema metrics
   ```

---

## F-4 · Common TF-K8s Issues

| Scenario ID | Common Issue | Resolution |
|-------------|--------------|------------|
| `TF-SLO-VOL` | Compression affecting ratio | Set `comparison_compress: false` in values.yaml |
| `TF-SLO-CPU` | cAdvisor metrics missing | Use `kubectl port-forward kubelet 10250:10250` |
| `TF-SLO-SER` | Baseline count too low | Increase workload.churn_rate |
| `TF-MFR-SC.5` | Dedup test flakiness | Set NRDOTPLUS_TEST_SEED environment variable |

---

## F-5 · Getting Support

1. File tickets at: [github.com/deepaucksharma/eidc-tfk8s/issues](https://github.com/deepaucksharma/eidc-tfk8s/issues)
2. Include:
   - Full collector config
   - Logs with `DEBUG=true`
   - Relevant TF-K8s scenario failures
   - Reproduction steps

---

*(end of file)*