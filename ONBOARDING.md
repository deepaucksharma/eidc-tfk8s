# Onboarding to the TDD Process

## Ticket: Enable guard-rails for Command Line Hashing in Edge-Probe

**Type:** Spec

**EIDC Reference:** annex_c_security.md#PII

**TF-K8s Scenario:** TF-MFR-EP.6_CommandLineHashing

## Description

This ticket implements the Test-Driven Development process for adding command line hashing functionality to the Edge-Probe component. Command line hashing is required to protect potentially sensitive information in process command lines while still allowing for debugging and monitoring capabilities.

## Expected Failing Test

The test will initially fail because:
1. The Edge-Probe schema does not include a `process_command_line_hash` field
2. The Edge-Probe implementation does not hash command lines
3. The security annex doesn't specify the hashing algorithm requirements

## Success Criteria

1. Edge-Probe schema updated to include `process_command_line_hash` field
2. Security annex updated with hashing algorithm requirements
3. Edge-Probe implementation modified to hash command lines
4. Test scenario passing

## Steps to Complete

1. Bot will move test to enabled/ but marked to fail
2. Bot will create PR with schema and documentation changes
3. Human will implement the Edge-Probe changes
4. Test will pass, showing test-driven process success

## Notes

This serves as a demonstration of the TDD process outlined in `.bot/PROCESS.md`.
