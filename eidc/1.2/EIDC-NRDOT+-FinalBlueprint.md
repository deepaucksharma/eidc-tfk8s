<!--
================================================================================================
 Edge Intel Design Charter (EIDC) v1.2 – Final Blueprint
 Host-wide Process Telemetry Optimisation for NRDOT+
================================================================================================
-->

| **Field**            | **Value**                                                                                                                         |
|----------------------|------------------------------------------------------------------------------------------------------------------------------------|
| **Doc-ID**           | `eidc/1.2/EIDC-NRDOT+-FinalBlueprint.md`                                                                                           |
| **Status**           | **FROZEN — Release Candidate**                                                                                                     |
| **Primary Owners**   | Chief Architect (Observability Platform), Lead Product Manager (NRDOT+)                                                            |
| **Down-stream Impl.**| NRDOT+ Smart Collector Distribution v1.2 (**COMP-SC**), NR-Edge-Probe v1.1 (**COMP-EP – optional**), NRDOT+ Language Agent Spec v1.1 |
| **Base OTel Collector** | `otelcol-contrib v0.136.1-nr1` (New Relic fork – adds `SHA256()` OTTL func + high-card filter patch #31906)                        |
| **Freeze Rule**      | Charter locks after **3 consecutive green TF-K8s v1.0 nightly runs** on RC images (functional gates only).                          |
| **Change Control**   | `CCB-RFC-EIDC-<version>-<change_id>` for any change to Scope, SLOs, MFRs, Export Schema, Stage-5 logic, Base OTel version, Security.|
| **Rollback Path**    | `CCB-HOTFIX-EIDC-x.y.z+1` – revert to previous signed tag, rerun P0 SLO scenarios.                                                 |

---

## 0 · Preamble & Mission

NRDOT+ v1.2 delivers a **Smart Collector** that slashes host-process metric cost while keeping every signal needed for alerts and root-cause.  
This charter defines **what must ship** and **how well it must work**.  Validation lives in *TF-K8s v1.0*.

---

## 1 · Top-Level Success Targets (SLOs)

| ID | SLO (vs. Baseline) | Metric / Formula | Target | Window & Notes | TF-K8s Scenario |
|----|--------------------|------------------|--------|----------------|-----------------|
| **VOL** | Ingest volume reduction | `(1 – Bytes_NRDOT+ ÷ Bytes_Baseline) × 100` | **≥ 70 %** | 2 h test → 24 h projection (gzip off) | `TF-SLO-VOL` |
| **SER** | Process-series reduction | `(1 – Series_NRDOT+ ÷ Series_Baseline) × 100` | **≥ 90 %** | 15-min rollup, high-churn mix | `TF-SLO-SER` |
| **ALR-R** | Alert recall (CPU spike) | `TP / (TP + FN)` | **≥ 98 %** | 50 synthetic spikes | `TF-SLO-ALR_Recall` |
| **ALR-P** | Alert precision | `TP / (TP + FP)` | **≥ 90 %** | 15-min quiet window | `TF-SLO-ALR_Precision` |
| **TOP5** | Diagnostic integrity | Core metrics for OS Top-5 CPU PIDs present & accurate ±10 % | **100 %** | 15 min steady load | `TF-SLO-TOP5` |
| **CPU** | Collector CPU | P90 vCPU under 10 k dps | **≤ 1.0** | 30 min, cAdvisor | `TF-SLO-CPU` |
| **RAM** | Collector RSS | P90 GiB, 1 M series state | **≤ 1.0** | 30 min, drops ≤ 0.01 % | `TF-SLO-RAM` |
| **LAT** | Pipeline latency | P95 µs → ms | **< 1000 ms** | 15 min | `TF-SLO-LAT` |

**Baseline collector** = plain `otelcol-contrib v0.136.1`, hostmetrics 10 s, batch, memory_limiter, no optimisations.

---

## 2 · Must-Have Functional Requirements (MFRs) – High-Lights

Full tables live in `annex_a_mfr_tables.md`.

### 2.1 COMP-SC (Smart Collector)

* **MFR-SC.1 Ingestion** – hostmetrics, OTLP, Prometheus (Edge-Probe).  
* **MFR-SC.1.4 Start-time enrich** – derive `process.start_time_ns` via `/proc` if hostmetrics missing.  
* **MFR-SC.3 Export schema** – ONLY metrics/attrs in `schemas/SmartCollector_Output_v1.2.yaml`; all Sums → Delta.  
* **MFR-SC.5 Dedup** – per-process key `(host.name, pid, start_time_ns)`; source priority **LA > EP > hostmetrics**.  
* **MFR-SC.6 Perf** – meet CPU/RAM/LAT SLOs.

### 2.2 COMP-EP (Edge-Probe – optional)

Prometheus endpoint `localhost:9999/metrics`; Top-10 by CPU q 10 s; includes `pid` & `start_time_ns`; ≤ 2 % core, ≤ 50 MiB.

### 2.3 COMP-LA (Language Agents)

OTLP out; must tag `pid` & start-time (ms); add `instrumentation.provider="newrelic-nrdotplus-<lang>"`.

---

## 3 · Minimal Export Schema (excerpt)

| Metric | Type | Unit | Required Dims (per-process) |
|--------|------|------|-----------------------------|
| `process.cpu.utilization` | Gauge | ratio 0-1 | host.name, pid, start_time_ns, executable.name, process.class |
| `process.cpu.time` | Sum Δ | s | host.name, pid, start_time_ns, executable.name, process.class, cpu, state |
| … | … | … | … |
| `process.aggregated.other.cpu.utilization` | Gauge | ratio 0-1 | host.name, `process.class="other_aggregate"` |
| `system.cpu.utilization` | Gauge | ratio 0-1 | host.name, cpu, state |

Hashed CLIs live in `process.custom.command_line_hash`; raw strings are **never** exported.

---

## 4 · Stage-5 Optimisation Pipeline — Logical → OTel Processors

| Stage | Purpose | Processor chain (in order) |
|-------|---------|----------------------------|
| **S0** | Ingest + resource attrs | `hostmetrics / prometheus / otlp` → `resourcedetection` |
| **S1** | PID normalise & enrich | `transform/pid_enrich_normalize` |
| **S2** | Source dedup (LA > EP > hostmetrics) | `filter/deduplicate_sources` |
| **S3** | Classify + label hygiene | `transform/classify_sanitize` |
| **S4** | Intelligent filter / sampling | `filter/intelligent_filter_sample` |
| **S5** | Δ-conversion + "other" aggregation | `cumulativetodelta` → `routing` → `metricstransform/aggregate_other_metrics` |
| **S6** | Schema enforce + export | `filter/final_schema` → `batch` → `otlphttp` |

All transforms/filters run with `error_mode: ignore` (prod); TF-K8s linter switches to `propagate`.

---

## 5 · Security Annex (snapshot)

* **SEC-1 SBOM** – CycloneDX v1.5 embedded; Grype blocks CRITICAL > 14 days.  
* **SEC-2 Cosign** – keyless-signed images; TF-K8s verifies signatures when `SECURITY_GATES=true`.  
* **SEC-3 Edge-Probe** – privileged but seccomp-restricted; minimal caps.  
* **SEC-4 PII** – command-line hashed then dropped.

---

## 6 · Validation Mapping (Matrix excerpt)

| EIDC Item | TF-K8s Scenario | Cluster | Blocking? |
|-----------|-----------------|---------|-----------|
| VOL | TF-SLO-VOL | k3d-duo | ✔ |
| ALR-P | TF-SLO-ALR_Precision | kind | ✔ |
| SEC-2 | TF-SEC-2_ImageSignature | kind | **optional** |

Full matrix in `annex_a_mfr_tables.md`.

---

## 7 · Change-Log (v1.2 vs v1.1)

* Base collector → `0.136.1-nr1`; built-in `SHA256()` & filter-OOM fix.  
* Added **ALR-P** precision SLO.  
* Introduced `/proc` start-time enrichment & `boot_id` fallback.  
* Re-ordered Stage-5 (Δ before aggregation).  
* New security annex & CI gates.  
* Schema: clarified units, mean vs sum on aggregates.

---

## 8 · Glossary (sample)

| Term | Definition |
|------|------------|
| **CPU util %** | `process.cpu.utilization` ratio (0-1) not scaled to 100. |
| **Δ Sum** | OTLP Sum metric with `aggregation_temporality=DELTA`. |
| **Other aggregate** | Host-level roll-up of low-importance processes after S4. |

---

## 9 · References

1. OpenTelemetry Collector Contrib v0.136.1 release notes.  
2. NR internal RFC `NR-RFC-0234 Edge Probe`.  
3. TF-K8s scenario catalogue v1.0.  

---

*(end of file)*