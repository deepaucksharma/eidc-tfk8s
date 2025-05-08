# Annex D: Glossary of Terms

This document provides definitions for key terms and acronyms used in the EIDC v1.2 and TF-K8s v1.0 documentation.

## Core Terminology

| Term | Definition |
|------|------------|
| **EIDC** | Edge Intel Design Charter - The specification for NRDOT+ design and implementation. |
| **TF-K8s** | TestForge-K8s - The validation framework for NRDOT+ components running in Kubernetes. |
| **NRDOT+** | New Relic Distro for OpenTelemetry Plus - New Relic's enhanced distribution of the OpenTelemetry Collector. |
| **COMP-SC** | Component - Smart Collector - The main NRDOT+ collector component implementing the EIDC v1.2 pipeline. |
| **COMP-EP** | Component - Edge-Probe - Optional lightweight process monitor that supplements COMP-SC with high-fidelity data. |
| **COMP-LA** | Component - Language Agent - Framework for application runtime integration with NRDOT+. |
| **SLO** | Service Level Objective - Specific, measurable targets for NRDOT+ performance and effectiveness. |
| **MFR** | Must-Have Functional Requirement - Non-negotiable functional capabilities required in NRDOT+. |
| **OTTL** | OpenTelemetry Transformation Language - Expression language used in OTel processors for transformations. |

## Technical Terms

| Term | Definition |
|------|------------|
| **CPU util %** | CPU utilization as a ratio (0-1) not scaled to 100%. Represented by `process.cpu.utilization`. |
| **Δ Sum** | Delta Sum - An OTLP Sum metric with `aggregation_temporality=DELTA`. |
| **Other aggregate** | Host-level roll-up of low-importance processes after Stage 4 filtering. |
| **Per-process key** | The unique identifier for a process instance: `(host.name, pid, start_time_ns)` or fallback `(host.name, pid, boot_id_ref)`. |
| **process.start_time_ns** | Nanosecond precision UTC Unix epoch timestamp when a process started. Critical for deduplication. |
| **boot_id_ref** | A fallback reference derived from `/proc/sys/kernel/random/boot_id` when start_time_ns is unavailable. |
| **Stage-5 Pipeline** | The 6-stage processing pipeline defined in EIDC for optimizing process telemetry. |
| **eBPF** | Extended Berkeley Packet Filter - A Linux kernel technology used by COMP-EP to efficiently monitor process activity. |

## Metrics & Validation Terms

| Term | Definition |
|------|------------|
| **VOL** | Volume Reduction SLO - Target reduction in total metric bytes ingested. |
| **SER** | Series Reduction SLO - Target reduction in unique active time series. |
| **ALR-R** | Alert Recall SLO - Target for correctly detecting true CPU saturation events. |
| **ALR-P** | Alert Precision SLO - Target for avoiding false positive alerts. |
| **TOP5** | Top 5 CPU Processes Diagnostic Integrity SLO - Target for accurately capturing the most important processes. |
| **k3d-duo** | A 2-node Kubernetes cluster profile used for multi-node validation scenarios. |
| **NRDOTPLUS_TEST_SEED** | A deterministic seed value used for reproducible sampling in TF-K8s. |
| **True Positive (TP)** | An actual CPU spike event that correctly triggers an alert. |
| **False Positive (FP)** | An alert that fires when no actual CPU spike occurred. |
| **False Negative (FN)** | An actual CPU spike event that fails to trigger an alert. |

## Security Terms

| Term | Definition |
|------|------------|
| **SBOM** | Software Bill of Materials - A comprehensive inventory of components in a software artifact. |
| **CycloneDX** | An SBOM specification format used for documenting software components and dependencies. |
| **Grype** | A vulnerability scanner for container images and filesystems. |
| **Cosign** | A tool for signing and verifying software artifacts, including container images. |
| **OIDC** | OpenID Connect - An identity layer on top of OAuth 2.0, used for keyless signing. |
| **seccomp** | Secure Computing Mode - A Linux kernel feature for restricting the syscalls a process can make. |
| **CAP_BPF** | Linux capability that grants permission to create and manipulate eBPF programs and maps. |
| **CAP_PERFMON** | Linux capability that grants permission to use the Linux perf_event_open syscall. |
| **CAP_SYS_ADMIN** | Linux capability that grants a broad set of administrative privileges. |
| **SHA-256** | Cryptographic hash function that generates a 256-bit (32-byte) hash value. |

---

*(end of file)*