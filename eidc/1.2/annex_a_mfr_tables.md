# Annex A: Detailed Must-Have Functional Requirements (MFRs)

This document provides the complete MFR tables for EIDC v1.2, expanding on the highlights presented in the main blueprint.

## COMP-SC: NRDOT+ Smart Collector Distribution v1.2

| ID | Requirement | Description | Priority | TF-K8s Validation |
|----|-------------|-------------|----------|-------------------|
| **MFR-SC.1** | Ingestion Sources | Must ingest from hostmetrics (primary), prometheus (for COMP-EP), and otlp (for COMP-LA). | P0 | TF-MFR-SC.1_IngestionSources |
| **MFR-SC.1.1** | Hostmetrics Config | hostmetrics receiver MUST have process, processes, cpu, memory, disk, network scrapers enabled with collection_interval configurable (default: 10s). | P0 | TF-MFR-SC.1.1_HostmetricsConfig |
| **MFR-SC.1.2** | Prometheus Config | prometheus receiver MUST listen on configurable endpoint (default: 0.0.0.0:8888) with valid scrape_config. | P0 | TF-MFR-SC.1.2_PrometheusConfig |
| **MFR-SC.1.3** | OTLP Config | otlp receiver MUST support both gRPC and HTTP transports on configurable endpoints. | P0 | TF-MFR-SC.1.3_OTLPConfig |
| **MFR-SC.1.4** | Start-Time Enrichment | If process.start_time_ns is missing or zero from hostmetrics source for a PID, COMP-SC's transform/pid_enrich stage MUST attempt to derive it by reading /proc/[pid]/stat (field 22, starttime) and system boot time (/proc/stat, btime). If successful, the derived nanosecond epoch timestamp MUST be used. If enrichment fails, the fallback deduplication key (host.name, process.pid, attribute["process.custom.boot_id_ref"]) MUST be used, where process.custom.boot_id_ref is populated from /proc/sys/kernel/random/boot_id. | P0 | TF-MFR-SC.1.4_PidEnrichment_HostmetricsVariations |
| **MFR-SC.2** | Stage-5 Pipeline | Must execute the EIDC Stage-5 Optimization Pipeline in the specified logical order. | P0 | Multiple scenarios |
| **MFR-SC.2.1** | Error Mode | All transform and filter processors MUST be configured with error_mode: ignore for production use. | P0 | TF-MFR-SC.2.1_ErrorMode_Production |
| **MFR-SC.2.2** | Memory Limiting | Must employ memory_limiter processor to prevent OOM termination. | P0 | TF-SLO-RAM |
| **MFR-SC.3** | Export Schema | Must export ONLY metrics and attributes defined in schemas/SmartCollector_Output_Metrics_v1.2.yaml. | P0 | TF-MFR-SC.3_ExportSchema |
| **MFR-SC.3.1** | Temporality | All exported monotonic Sum metrics MUST be Delta temporality. | P0 | TF-MFR-SC.3.1_DeltaTemporality |
| **MFR-SC.4** | Attribute Handling | Must apply process.custom.classification, hash command_line, truncate paths. | P0 | TF-MFR-SC.4_AttributeHandling |
| **MFR-SC.4.1** | Classification | Must set process.custom.classification according to defined enum. | P0 | TF-MFR-SC.4.1_ProcessClassification |
| **MFR-SC.4.2** | Command-Line Handling | Must hash process.command_line to process.custom.command_line_hash using SHA-256, then REMOVE the original process.command_line and process.command_args attributes. | P0 | TF-MFR-SC.4.2_CommandLineHashing |
| **MFR-SC.4.3** | Path Truncation | Must truncate long paths to basenames (e.g., process.executable.path to process.executable.name). | P1 | TF-MFR-SC.4.3_PathTruncation |
| **MFR-SC.5** | Source Deduplication | For unique process instances (identified by (host.name, process.pid, process.start_time_ns) or fallback key), select data based on priority: COMP-LA > COMP-EP > hostmetrics. Lower-priority source data for that exact instance MUST be dropped. | P0 | TF-MFR-SC.5_Deduplication_MultiSourceMultiNode |
| **MFR-SC.6** | Performance | Must adhere to SLOs CPU, RAM, LAT. | P0 | TF-SLO-CPU, TF-SLO-RAM, TF-SLO-LAT |
| **MFR-SC.7** | New Relic OTLP Export | Must configure otlphttp exporter with appropriate settings for New Relic (compression: "gzip", queue with retry, etc.). | P0 | TF-MFR-SC.7_OTLPHTTPExport |

## COMP-EP: NR-Edge-Probe v1.1 (Optional)

| ID | Requirement | Description | Priority | TF-K8s Validation |
|----|-------------|-------------|----------|-------------------|
| **MFR-EP.1** | Prometheus Endpoint | Must expose Prometheus metrics prefixed nr_edge_* at localhost:9999/metrics (default). | P0 | TF-MFR-EP.1_PrometheusEndpoint |
| **MFR-EP.2** | Top-K Process Reporting | Must report Top-K processes (default: Top-10) by CPU utilization, refreshed every 10s (configurable). | P0 | TF-MFR-EP.2_TopKProcesses |
| **MFR-EP.3** | Process Identity Attributes | MUST include process.pid (int64) and process.start_time_ns (int64, nanosecond epoch UTC) for all process data, derived from kernel sources. | P0 | TF-MFR-EP.3_ProcessIdentityAttrs |
| **MFR-EP.4** | Performance Overhead | Must consume ≤2% of one host CPU core and ≤50MiB RSS memory. | P0 | TF-MFR-EP.4_PerformanceOverhead |
| **MFR-EP.5** | eBPF Minimality | If using eBPF, must use minimal privileges and capabilities. | P1 | TF-SEC-3_EdgeProbeHardening |

## COMP-LA: NRDOT+ Language Agent Integration Spec v1.1

| ID | Requirement | Description | Priority | TF-K8s Validation |
|----|-------------|-------------|----------|-------------------|
| **MFR-LA.1** | OTLP Emission | Must emit application's own core process metrics (CPU utilization, Memory RSS) via OTLP. | P0 | TF-MFR-LA.1_OTLPEmission |
| **MFR-LA.2** | Process Identity Attributes | MUST include process.pid (int64) and a process start time attribute (e.g., process.runtime.jvm.start_time_ms) convertible to process.start_time_ns by COMP-SC. | P0 | TF-MFR-LA.2_ProcessIdentityAttrs |
| **MFR-LA.3** | Source Identification | MUST include attributes identifying source as NRDOT+ Language Agent (e.g., instrumentation.provider="newrelic-nrdotplus-java", telemetry.sdk.language). | P0 | TF-MFR-LA.3_SourceIdentification |

## EIDC-TF-K8s Validation Mapping (Complete)

| EIDC Item | TF-K8s Scenario | Cluster | Blocking? |
|-----------|-----------------|---------|-----------|
| VOL | TF-SLO-VOL_DataVolumeReduction_SteadyWorkload | k3d-duo | ✓ |
| SER | TF-SLO-SER_ProcessCardinalityReduction_MixedWorkload | k3d-duo | ✓ |
| ALR-R | TF-SLO-ALR_Recall_CPUSpikeReplay | kind | ✓ |
| ALR-P | TF-SLO-ALR_Precision_NoSpikeWindow | kind | ✓ |
| TOP5 | TF-SLO-TOP5_DiagnosticIntegrity_TopCPUProcesses | k3d-duo | ✓ |
| CPU | TF-SLO-CPU_CollectorPerf_LoadTest_10Kdps_cAdvisor | kind | ✓ |
| RAM | TF-SLO-RAM_CollectorPerf_LoadTest_10Kdps_HighState_cAdvisor | kind | ✓ |
| LAT | TF-SLO-LAT_CollectorPipelineLatency_SteadyWorkload | kind | ✓ |
| MFR-SC.1 | TF-MFR-SC.1_IngestionSources | kind | ✓ |
| MFR-SC.1.4 | TF-MFR-SC.1.4_PidEnrichment_HostmetricsVariations | kind | ✓ |
| MFR-SC.2 | Multiple scenarios | kind, k3d-duo | ✓ |
| MFR-SC.3 | TF-MFR-SC.3_ExportSchema | kind | ✓ |
| MFR-SC.4 | TF-MFR-SC.4_AttributeHandling | kind | ✓ |
| MFR-SC.5 | TF-MFR-SC.5_Deduplication_MultiSourceMultiNode | k3d-duo | ✓ |
| MFR-SC.7 | TF-MFR-SC.7_OTLPHTTPExport | kind | ✓ |
| MFR-EP.1-4 | TF-MFR-EP_AllRequirements | kind | Optional |
| MFR-LA.1-3 | TF-MFR-LA_AllRequirements | kind | ✓ |
| SEC-1 | TF-SEC-1_SBOM_Validation | kind | Optional |
| SEC-2 | TF-SEC-2_ImageSignature_Validation | kind | Optional |
| SEC-3 | TF-SEC-3_EdgeProbeHardening | kind | Optional |
| SEC-4 | TF-MFR-SC.4.2_CommandLineHashing | kind | ✓ |

---

*(end of file)*