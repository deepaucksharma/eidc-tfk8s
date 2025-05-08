# EIDC v1.2 and TF-K8s v1.0 Documentation

This repository contains comprehensive documentation for Edge Intel Design Charter (EIDC) v1.2 (Final Blueprint) for host-wide process telemetry optimization, and TestForge-K8s (TF-K8s) v1.0 validation framework.

## Overview

The EIDC defines a Smart Collector implementation that optimizes host-process telemetry:
- Significantly reducing metric volume and cardinality
- Preserving critical diagnostic signals
- Ensuring robust alerting capabilities

TF-K8s provides the validation framework that uses Kubernetes environments to test and verify EIDC compliance.

## Repository Structure

- [**eidc/1.2/**](./eidc/1.2/): EIDC Documentation
  - [EIDC-NRDOT+-FinalBlueprint.md](./eidc/1.2/EIDC-NRDOT+-FinalBlueprint.md): Final Blueprint
  - [annex_a_mfr_tables.md](./eidc/1.2/annex_a_mfr_tables.md): Detailed MFR Tables
  - [annex_c_security.md](./eidc/1.2/annex_c_security.md): Security Requirements
  - [annex_d_glossary.md](./eidc/1.2/annex_d_glossary.md): Terms and Definitions
  - [schemas/](./eidc/1.2/schemas/): Output Schema Definitions

- [**tf-k8s/1.0/**](./tf-k8s/1.0/): TF-K8s Validation Suite
  - [TF-K8s-NRDOT+-ValidationSuite-Final.md](./tf-k8s/1.0/TF-K8s-NRDOT+-ValidationSuite-Final.md): Validation Plan
  - [scenario_catalogue.md](./tf-k8s/1.0/scenario_catalogue.md): Test Scenario Catalog

## Key Features

NRDOT+ v1.2 delivers a Smart Collector that addresses several critical requirements:

1. ≥70% host-process metric data volume reduction
2. ≥90% process-series cardinality reduction
3. High alert recall (≥98%) and precision (≥90%)
4. Nanosecond start-time enrichment from `/proc`
5. Intelligent process classification and filtering
6. Command-line hashing for PII protection
7. Optimized Stage-5 Pipeline implementation

## Service Level Objectives (SLOs)

This implementation meets or exceeds the following SLOs:

- **VOL**: ≥70% ingest volume reduction
- **SER**: ≥90% process-series reduction
- **ALR-R**: ≥98% alert recall for CPU spike events
- **ALR-P**: ≥90% alert precision
- **TOP5**: 100% diagnostic integrity for top CPU processes
- **CPU**: ≤1.0 vCPU collector overhead
- **RAM**: ≤1.0 GiB collector memory usage
- **LAT**: <1000ms pipeline latency

## Implementation Components

The EIDC defines three main components:

1. **COMP-SC**: NRDOT+ Smart Collector (required)
   - Core component implementing the Stage-5 optimization pipeline
   - Integrates hostmetrics, deduplication, and intelligent filtering

2. **COMP-EP**: NR-Edge-Probe (optional)
   - Lightweight process monitor providing higher-fidelity data for top processes
   - Prometheus metrics endpoint for high-priority processes

3. **COMP-LA**: NRDOT+ Language Agent Integration (optional)
   - Application runtime integration for improved process visibility
   - Highest-priority data source in the deduplication hierarchy

## Validation Framework

TF-K8s provides:
- Kubernetes-based test environments (kind, k3d)
- Standardized validation scenarios
- Automated SLO verification
- Multi-node test capability

## License

This project is licensed under the MIT License.