# Annex C: Security Requirements

This document provides detailed security requirements for NRDOT+ v1.2 components.

## SEC-1: Software Bill of Materials (SBOM)

**Requirement:**
Release OCI images for COMP-SC and COMP-EP MUST embed a CycloneDX v1.5 compliant SBOM. The CI pipeline MUST use Grype to scan for CVEs, and builds will fail on CRITICAL CVEs older than 14 days unless specifically waived in SEC-WAIVER.yaml.

**Implementation Details:**
- The SBOM must be attached to the container image as an attestation using the `cyclonedx-bom` predicate type
- The SBOM must include all direct and transitive dependencies
- The CI pipeline must generate the SBOM during the build process using syft or equivalent tool
- Grype scan results must be attached to build artifacts for auditing

**Waiver Process:**
- SEC-WAIVER.yaml must include:
  - CVE ID
  - Justification for the waiver
  - Expiration date for the waiver
  - Approver name and role
  - Mitigation strategy

**Validation (TF-SEC-1_SBOM_Validation):**
- Deploy COMP-SC/EP
- Verify SBOM attestation presence 
- Run Grype scan on deployed images
- Verify no un-waived CRITICAL CVEs older than 14 days

## SEC-2: Image Signing

**Requirement:**
Release OCI images MUST be Cosign-signed using keyless signing (GitHub OIDC issuer). TF-K8s MUST verify signatures before pulling images if security gates are enabled.

**Implementation Details:**
- Use keyless signing with Cosign and GitHub OIDC
- Image signature verification must be enabled in TF-K8s when SECURITY_GATES=true
- Build metadata must be included in the signature
- Signature verification errors must be treated as blocking failures

**Validation (TF-SEC-2_ImageSignature_Validation):**
- Pull COMP-SC/EP images 
- Verify Cosign signatures against GitHub OIDC issuer
- Test failure behavior with tampered images

## SEC-3: COMP-EP Hardening

**Requirement:**
COMP-EP eBPF agent (if used) MUST run in a privileged container but with a restrictive seccomp-profile (e.g., ebpf_secure.json) and minimal capabilities (e.g., CAP_BPF, CAP_PERFMON, CAP_SYS_ADMIN only if unavoidable and justified). No CAP_SYS_PTRACE or broad CAP_NET_ADMIN.

**Implementation Details:**
- Define a custom seccomp profile for the COMP-EP container
- Limit Linux capabilities to the minimum required for eBPF functionality
- Document and justify any use of CAP_SYS_ADMIN
- Implement runtime monitoring for capability usage

**Seccomp Profile Requirements:**
- Allow only necessary syscalls
- Block potentially dangerous syscalls
- Include explicit allow/deny rules for BPF operations

**Validation (TF-SEC-3_EdgeProbeHardening):**
- Deploy COMP-EP with seccomp profile and limited capabilities
- Verify functionality with limited capabilities
- Attempt operations that should be blocked by seccomp profile
- Monitor capability usage during operation

## SEC-4: PII Handling

**Requirement:**
Re-affirms MFR-SC.4: process.command_line and process.command_args MUST be hashed using SHA-256 or dropped entirely, never exported raw. Edge-Probe MUST also implement command line hashing using SHA-256 and store the hash value in the process.command_line_hash attribute.

**Implementation Details:**
- Use SHA-256 hashing for process.command_line and store as process.custom.command_line_hash in COMP-SC
- For Edge-Probe, use SHA-256 hashing for process.command_line and store as process.command_line_hash
- Remove original process.command_line and process.command_args attributes before export
- Apply consistent hashing across all COMP-SC and Edge-Probe deployments
- Ensure no PII leakage through other attributes

**Validation (TF-MFR-SC.4.2_CommandLineHashing and TF-MFR-EP.6_CommandLineHashing):**
- Generate various command lines with sensitive data
- Verify hashing is applied correctly
- Confirm original command line data is not present in exported telemetry
- Validate hash consistency across collector instances

## Additional Security Considerations

### Secure Configuration Distribution

- Default configurations must follow security best practices
- Sensitive connection strings or API keys must be stored securely
- Configuration changes should be auditable

### Authentication and Authorization

- API endpoints must require proper authentication
- Least privilege principle must be applied to service accounts
- Credential rotation mechanisms should be documented

### Network Security

- Inter-component communication should use TLS
- Firewall rules should restrict unnecessary network exposure
- Consider network policies for Kubernetes deployments

### Audit Logging

- Security-relevant events must be logged
- Logs should include sufficient context for investigation
- Log retention policies should be defined

---

*(end of file)*