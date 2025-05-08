# TF-K8s v1.0 Scenario Catalogue

This document provides a complete catalog of all validation scenarios for EIDC v1.2, along with their mapping to specific EIDC requirements.

## SLO Validation Scenarios (Primary)

| Scenario ID | EIDC Ref | Cluster | Description | Success Criteria |
|-------------|----------|---------|-------------|------------------|
| **TF-SLO-VOL_DataVolumeReduction_SteadyWorkload** | VOL | k3d-duo | Deploys baseline and NRDOT+ collectors side-by-side with identical hostmetrics scrape intervals. Runs a steady workload with predictable processes. Collects raw OTLP output bytes from both, with compression disabled. | ≥ 70% reduction in bytes sent to New Relic. |
| **TF-SLO-SER_ProcessCardinalityReduction_MixedWorkload** | SER | k3d-duo | Deploys baseline and NRDOT+ collectors with a high-churn workload generator that creates many short-lived processes. Uses VictoriaMetrics to count unique process.* series. | ≥ 90% reduction in unique active time series for process.* metrics. |
| **TF-SLO-ALR_Recall_CPUSpikeReplay** | ALR-R | kind | Injects 50 synthetic CPU spike events via cpu_spiker.sh with precise start/stop times. Uses the New Relic API to verify that alerting conditions correctly detect each spike. | Recall rate ≥ 98% (at least 49/50 spikes detected). |
| **TF-SLO-ALR_Precision_NoSpikeWindow** | ALR-P | kind | After the ALR-R test completes, continues monitoring for 15 minutes with no injected spikes. Verifies that alerting conditions do not trigger false positives. | Precision rate ≥ 90% (≤ 1 false positive during the quiet window). |
| **TF-SLO-TOP5_DiagnosticIntegrity_TopCPUProcesses** | TOP5 | k3d-duo | Runs a mixed workload with several high-CPU processes. Uses host tools to identify the top 5 CPU-consuming processes by PID. Verifies that these processes are accurately represented in the NRDOT+ New Relic stream with full fidelity. | 100% of top 5 CPU processes have accurate metrics (±10% of OS value) with ≥95% datapoint coverage. |
| **TF-SLO-CPU_CollectorPerf_LoadTest_10Kdps_cAdvisor** | CPU | kind | Runs a simulated 10,000 dps load through the COMP-SC pipeline. Measures CPU utilization via cAdvisor. | P90 collector CPU ≤ 1.0 vCPU over 30-minute test. |
| **TF-SLO-RAM_CollectorPerf_LoadTest_10Kdps_HighState_cAdvisor** | RAM | kind | Runs the same load as CPU test, but with additional state tracking for 1M unique time series. Measures memory RSS via cAdvisor. Monitors otelcol_processor_dropped_metric_points from memory_limiter. | P90 collector RAM ≤ 1.0 GiB RSS over 30-minute test. Dropped datapoints ≤ 0.01%. |
| **TF-SLO-LAT_CollectorPipelineLatency_SteadyWorkload** | LAT | kind | Deploys COMP-SC with detailed telemetry. Monitors otelcol_processor_latency_microseconds metrics for end-to-end pipeline latency. | P95 pipeline latency < 1000 ms over 15-minute test. |

## MFR Validation Scenarios

| Scenario ID | EIDC Ref | Cluster | Description | Success Criteria |
|-------------|----------|---------|-------------|------------------|
| **TF-MFR-SC.1_IngestionSources** | MFR-SC.1 | kind | Verifies that COMP-SC can ingest metrics from all required sources (hostmetrics, prometheus, otlp). | All receivers properly configured and receiving data. |
| **TF-MFR-SC.1.4_PidEnrichment_HostmetricsVariations** | MFR-SC.1.4 | kind | Tests multiple scenarios: hostmetrics with start_time_ns, without start_time_ns (enrichment success), and /proc read fail (fallback key). | Correct start_time_ns derived when possible, fallback boot_id used otherwise. |
| **TF-MFR-SC.3_ExportSchema** | MFR-SC.3 | kind | Checks that the exported metrics strictly adhere to the defined schema. | Only schema-defined metrics and attributes present in output. |
| **TF-MFR-SC.4_AttributeHandling** | MFR-SC.4 | kind | Verifies proper attribute handling: classification, command line hashing, path truncation. | All attributes properly transformed according to requirements. |
| **TF-MFR-SC.5_Deduplication_MultiSourceMultiNode** | MFR-SC.5 | k3d-duo | Tests complex deduplication scenarios with overlapping PIDs from different sources and nodes. | Correct source chosen per priority, with no duplicates for unique instances. |
| **TF-MFR-EP_AllRequirements** | MFR-EP.1-4 | kind | Validates all Edge-Probe requirements: metrics endpoint, Top-K reporting, identity attributes, and performance overhead. | All requirements met per specification. |
| **TF-MFR-LA_AllRequirements** | MFR-LA.1-3 | kind | Tests Language Agent integration: OTLP emission, identity attributes, and source identification. | All requirements met per specification. |

## Special Validation Scenarios

| Scenario ID | EIDC Ref | Cluster | Description | Success Criteria |
|-------------|----------|---------|-------------|------------------|
| **TF-REG-Filter_HighCard_Memory_Scale** | Filter Patch | k3d-duo | Tests the high-cardinality filter fix by feeding 1M distinct label sets through filter/intelligent_filter_sample. | Collector RAM stays ≤ 1.0 GiB, no OOM, pipeline processes data correctly. |
| **TF-MFR-SC.2_AggregationLogic_DiskIONetworkIO** | Aggregation Logic | kind | Validates the process.aggregated.other.disk.io aggregation to ensure it correctly sums deltas without double-delta errors. | Aggregated values match expected sum of individual deltas. |
| **TF-LINT-OTTL_StrictErrorMode** | OTTL Error Handling | kind | Runs COMP-SC with error_mode: propagate for all processors to validate OTTL expressions. | No OTTL errors logged, pipeline processes data correctly. |

## Security Validation Scenarios (Optional)

| Scenario ID | EIDC Ref | Cluster | Description | Success Criteria |
|-------------|----------|---------|-------------|------------------|
| **TF-SEC-1_SBOM_Validation** | SEC-1 | kind | Deploys COMP-SC/EP. Runs Grype scan on images and checks for CRITICAL CVEs per policy. Verifies SBOM presence. | SBOM present, no un-waived critical CVEs older than 14 days. |
| **TF-SEC-2_ImageSignature_Validation** | SEC-2 | kind | Pulls COMP-SC/EP images. Verifies Cosign signatures against GitHub OIDC issuer. | Signatures valid against the expected issuer. |
| **TF-SEC-3_EdgeProbeHardening** | SEC-3 | kind | Verifies that COMP-EP runs with the specified seccomp profile and minimal capabilities. | Container starts and functions correctly with restricted privileges. |

## Scenario Groups for CI Matrix

For CI parallelization, scenarios are grouped into logical sets:

| Group | Scenarios | Purpose |
|-------|-----------|---------|
| **slo-core** | TF-SLO-VOL, TF-SLO-SER, TF-SLO-TOP5 | Core data reduction and fidelity validation |
| **slo-perf** | TF-SLO-CPU, TF-SLO-RAM, TF-SLO-LAT | Performance characteristic validation |
| **slo-alert** | TF-SLO-ALR_Recall, TF-SLO-ALR_Precision | Alert quality validation |
| **mfr-basic** | TF-MFR-SC.1, TF-MFR-SC.3, TF-MFR-SC.4 | Basic functional requirement validation |
| **mfr-advanced** | TF-MFR-SC.1.4, TF-MFR-SC.5, TF-MFR-SC.2_AggregationLogic | Advanced functional requirement validation |
| **mfr-components** | TF-MFR-EP, TF-MFR-LA | Component integration validation |
| **regression** | TF-REG-Filter_HighCard, TF-LINT-OTTL | Regression testing for specific fixes |
| **security** | TF-SEC-* | Security validation (optional) |

## Scenario Execution Flow

Each scenario follows a standardized execution flow:

1. **Setup**: Create namespace, deploy required components
2. **Preparation**: Configure workloads, set environment variables
3. **Execution**: Run the test workload for the specified duration
4. **Collection**: Gather metrics and logs from relevant sources
5. **Analysis**: Process the collected data to evaluate success criteria
6. **Assertion**: Compare results against thresholds
7. **Reporting**: Generate structured output
8. **Teardown**: Clean up resources

The full execution logic is defined in each scenario's `scenario_definition.yaml` file.

---

*(end of file)*