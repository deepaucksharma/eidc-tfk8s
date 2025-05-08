# TestForge-K8s (TF-K8s) v1.0
## Validation Suite for EIDC v1.2 NRDOT+

| **Field**            | **Value**                                                                                                     |
|----------------------|---------------------------------------------------------------------------------------------------------------|
| **Doc-ID**           | `tf-k8s/1.0/TF-K8s-NRDOT+-ValidationSuite-Final.md`                                                           |
| **Status**           | **FROZEN – Release Candidate**                                                                                |
| **Target EIDC**      | `eidc/1.2/EIDC-NRDOT+-FinalBlueprint.md`                                                                      |
| **Primary Owners**   | Head of Test Engineering, Lead TestForge Engineer                                                             |
| **Cluster Profiles** | kind-single (for unit/smoke tests), k3d-duo (2 worker nodes, default for full SLO/MFR validation, simulates multi-host). Windows ETW lane: Azure aks-win-etw (experimental, non-blocking for v1.0 functional freeze). |
| **Security Gates**   | Disabled by default (for functional validation). Enabled via SECURITY_GATES=true environment variable in CI, activating SEC-1/2/3 validation scenarios. |
| **CI Max Wall-Clock**| Approx. 45 minutes on a 4-parallel runner pool (functional suite).                                           |

---

## 1 · Core Objectives & Principles

**Primary Objective**: Provide definitive, automated, and continuous validation that NRDOT+ v1.2 (COMP-SC, COMP-EP, COMP-LA integration) meets or exceeds all SLOs and MFRs specified in EIDC v1.2.

**Principles**: EIDC-Driven, High-Fidelity Simulation (multi-node, varied workloads), Automation-Centric, Reproducibility (versioned configs, deterministic sampling seed), Modularity, Observability of Tests, Security-First (if gates enabled).

---

## 2 · TF-K8s Lab Architecture & Core Infrastructure

**Kubernetes Platforms**: kind v0.20+, k3d v5.x+. cluster-ctrl.sh for setup.

**Local Docker Registry**: registry:2 at localhost:5000, mirrored in Kind/k3d.

**Helm Charts** (`tf-k8s/charts/`): For NRDOT+ components, baseline collector, workload generators, validation tools.

- **nrdot-smart-collector**: Deploys COMP-SC with EIDC v1.2 pipeline from tf-k8s/collector-configs/nrdot-plus/eidc-v1.2/.
- **host-workload-generator**: Simulates diverse process activity, churn, CPU/memory/IO load patterns. Configurable via YAML. Includes pid_churner.sh, cpu_spiker.sh.
- **app-workload-java-la**: Simulates COMP-LA OTLP data, including process.runtime.jvm.start_time_ms.
- **validation-tools**: Local Prometheus, VictoriaMetrics (for TSDB storage), Grafana (for visualizing test metrics), mock-alertmanager (for deterministic ALR-P tests).

**Scenario Orchestration** (`tf-k8s/scripts/scenario-runner.py`): Manages full lifecycle of test scenarios defined in tf-k8s/scenarios/.

**Configuration Store**:

- `tf-k8s/collector-configs/`: OTel Collector config.yaml for COMP-SC (functional, secure) and baseline.
- `tf-k8s/workload-configs/`: Parameterized workload profiles.
- `tf-k8s/thresholds-eidc-v1.2.yaml`: Quantitative targets for SLOs/MFRs.
- `tf-k8s/schemas/`: Copies of EIDC metric schemas for validation.

---

## 3 · TF-K8s Scenario Structure & Execution Flow

Standardized directory structure under `tf-k8s/scenarios/`.

`scenario_definition.yaml` per scenario: metadata, EIDC items covered, Helm chart values, workload config, analysis scripts, assertion checks.

**Flow**: Setup Namespace & Tools → Deploy Baseline/NRDOT+ → Execute Workload (with NRDOTPLUS_TEST_SEED for sampling) → Collect Data (OTLP File Exporter outputs, Prometheus/VM queries, cAdvisor via Kubelet /metrics/cadvisor for COMP-SC CPU/RAM, New Relic API for ALR-R) → Analyze → Assert → Report (JSON, HTML, JUnit XML) → Teardown.

---

## 4 · Detailed Validation Scenario Catalogue (Highlights)

(Full list in `tf-k8s/1.0/scenario_catalogue.md`, mapping each to EIDC SLOs/MFRs)

| **EIDC Ref.** | **Scenario ID** | **Cluster** | **Key Purpose & Method** | **Key Pass Criteria** |
|---------------|-----------------|-------------|--------------------------|------------------------|
| **VOL** | TF-SLO-VOL_DataVolumeReduction_SteadyWorkload | k3d-duo | Compare uncompressed OTLP byte output of NRDOT+ vs. Baseline under steady workload. | ≥ 70% reduction. |
| **SER** | TF-SLO-SER_ProcessCardinalityReduction_MixedWorkload | k3d-duo | Compare unique process.* series (VictoriaMetrics count(count by())) NRDOT+ vs. Baseline under high churn/diversity workload. | ≥ 90% reduction. |
| **ALR-R** | TF-SLO-ALR_Recall_CPUSpikeReplay | kind | Inject N CPU spikes; verify New Relic alert fires (via API polling). | Recall ≥ 98%. |
| **ALR-P** | TF-SLO-ALR_Precision_NoSpikeWindow | kind | Monitor mock-alertmanager for false positives during a 15-min no-spike period. | Precision ≥ 90% (≤1 FP). |
| **TOP5** | TF-SLO-TOP5_DiagnosticIntegrity_TopCPUProcesses | k3d-duo | Identify OS Top-5 CPU PIDs. Verify their core metrics are present, unaggregated, accurate (±10% vs. OS ps values), and have ≥95% datapoint coverage in NRDOT+ New Relic stream. | 100% integrity. |
| **CPU** | TF-SLO-CPU_CollectorPerf_LoadTest_10Kdps_cAdvisor | kind | 10k dps load; measure COMP-SC P90 CPU via cAdvisor. | ≤ 1.0 vCPU. |
| **RAM** | TF-SLO-RAM_CollectorPerf_LoadTest_10Kdps_HighState_cAdvisor | kind | 10k dps load + 1M series state; measure COMP-SC P90 RAM via cAdvisor. Check otelcol_processor_dropped_metric_points for memory_limiter. | ≤ 1.0 GiB RSS; drops ≤0.01%. |
| **MFR-SC.1.4** | TF-MFR-SC.1.4_PidEnrichment_HostmetricsVariations | kind | Test pid_enrich logic: hostmetrics with start_time_ns, without start_time_ns (enrich success), /proc read fail (fallback key used). Verify correct process.custom.boot_id_ref. | Correct start_time_ns / fallback key used, no dedup errors. |
| **MFR-SC.5 (Dedup)** | TF-MFR-SC.5_Deduplication_MultiSourceMultiNode | k3d-duo | COMP-LA, COMP-EP, hostmetrics report overlapping PIDs (some with same PID value on different nodes, some with same PID value + start time on same node). Test source priority & fallback keys. | Correct source chosen, no duplicates for unique instances. |
| **Filter Patch** | TF-REG-Filter_HighCard_Memory_Scale | k3d-duo | Feed 1M distinct label sets through filter/intelligent_filter_sample. | COMP-SC RSS stays ≤ 1 GiB (as per RAM SLO), no OOM, pipeline intact. |
| **Aggregation Logic** | TF-MFR-SC.2_AggregationLogic_DiskIONetworkIO | kind | Simulate multiple processes generating disk/network I/O. Verify process.aggregated.other.disk.io sums deltas correctly (not double-delta). | Aggregated values match expected sum of individual deltas. |
| **OTTL Linting** | TF-LINT-OTTL_StrictErrorMode | kind | Run COMP-SC with error_mode: propagate for all transform/filter processors under a diverse workload. | No OTTL errors logged, pipeline processes data. |
| **(Optional) SEC-1** | TF-SEC-1_SBOM_Validation | kind | Deploy COMP-SC/EP. Run Grype scan on images. Check for CRITICAL CVEs per policy. Verify SBOM presence. | SBOM present, no un-waived critical CVEs. |
| **(Optional) SEC-2** | TF-SEC-2_ImageSignature_Validation | kind | Pull COMP-SC/EP images. Verify Cosign signatures against GitHub OIDC issuer. | Signatures valid. |

---

## 5 · CI/CD Integration & Reporting

**CI Workflow** (`.github/workflows/nrdotplus_validation.yml`):

- **Triggers**: PRs to main, nightly, release tags.
- **Matrix Strategy**: Parallel jobs per (Scenario Group, Cluster Profile). Functional suite runs by default. SECURITY_GATES=true enables SEC scenarios.
- **Job Steps**: Setup Kind/k3d & Registry → Build/Pull NRDOT+ Images (verify sigs if SECURITY_GATES=true) → Execute scenario-runner.py → Upload Artifacts.
- **Reporting**: Structured JSON, HTML summaries, JUnit XML. Nightly results dashboard.
- **Flake-Retry**: Retry once for known transient network/CI issues (Exit Code 88). Persistent flakes fail the build.

---

## 6 · "EZ-Mode" Config Deployment in TF-K8s

The `quickstart/collector-functional.yaml` is used as the base for most TF-K8s COMP-SC deployments, with scenario-specific overrides applied via Helm values.

`quickstart/collector-secure.yaml` is used if SECURITY_GATES=true.

---

## 7 · Launch Checklist (TF-K8s Milestones for EIDC 1.2 Release)

| **Day Ref.** | **Milestone** | **Gate** |
|--------------|---------------|----------|
| **T0 - 10d** | TF-K8s v1.0 IOC: Core framework operational. Key SLO scenarios (VOL, SER, CPU, RAM) runnable on k3d-duo. cAdvisor integration complete. | Demo of core SLO scenario execution. |
| **T0 - 5d** | All Functional SLO & P0 MFR Scenarios Implemented: Full functional test coverage as per EIDC 1.2. Includes multi-node dedup, pid_enrich. | Code complete for all functional scenarios. |
| **T0 - 2d** | (Optional) All SEC Scenarios Implemented: If security gates are part of the release. | Code complete for SEC scenarios. |
| **T0 - 1d** | CI Suite Stability Run: Full functional TF-K8s suite passes 3/3 times on k3d-duo against RC builds. | CI dashboard green for functional suite. |
| **T0** | TF-K8s v1.0 Tagged & EIDC v1.2 Validation Confirmed: Official report confirms functional EIDC 1.2 validated. (Optional) SEC gates pass. | Sign-off on TF-K8s validation report for EIDC 1.2 freeze. |

---

*(end of file)*