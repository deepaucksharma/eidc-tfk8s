# TF-K8s Validation Scorecard

**Generated on:** 2025-05-09 12:00:00

## Summary

- **Tests Passing:** 22/24 (91.7%)
- **Documentation Drift:** None
- **Open Security Issues:** 1

## Test Results by Category

| Category | Status | Pass Rate |
| -------- | ------ | --------- |
| SLO Core | ✅ | 100% |
| SLO Performance | ✅ | 100% |
| SLO Alerting | ⚠️ | 90% |
| MFR Basic | ✅ | 100% |
| MFR Advanced | ⚠️ | 95% |
| MFR Components | ✅ | 100% |
| Regression | ✅ | 100% |
| Security | ⚠️ | 66% |

## Failed Tests

1. **TF-SLO-ALR_Precision_NoSpikeWindow**: False positive rate exceeded threshold
2. **TF-SEC-1_SBOM_Validation**: Found 1 un-waived critical CVE

## Action Items

1. Investigate false positive alerts in the NoSpikeWindow test
2. Address CVE-2024-1234 in the Edge-Probe component
