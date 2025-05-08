# EIDC-TF-K8s Bot Tools and Templates

This directory contains tooling, templates, and prompts for maintaining the EIDC-TF-K8s project using Test-Driven Development (TDD) principles and AI assistance.

## Directory Structure

- **prompts/**: System prompts used for AI model interactions
  - `diff.md`: Prompt for generating PR descriptions from diffs
  - `failing-test.md`: Prompt for generating failing tests
  - `implementation.md`: Prompt for implementing features to pass tests

- **templates/**: Reusable templates for project artifacts
  - `scenario_template.yaml`: Template for test scenario definitions
  - `verification_template.py`: Template for verification scripts
  - `k8s_component_template.yaml`: Template for Kubernetes resources

- **scripts/**: Helper scripts for common workflows
  - `validate-failing-test.py`: Validates failing test for correctness
  - `create-failing-test.sh`: Creates failing test skeleton from templates

- **PROCESS.md**: The high-level TDD process description
- **WORKFLOW.md**: Detailed step-by-step workflow guide

## Quick Start

### Creating a New Failing Test

```bash
# From the repository root
.bot/scripts/create-failing-test.sh TF-MFR-SC.7_NewFeature annex_b_pipeline_config_examples.md#ProcessorRules
```

This creates a skeleton failing test in `tf-k8s/scenarios/pending/TF-MFR-SC.7_NewFeature/`.

### Validating a Failing Test

```bash
python .bot/scripts/validate-failing-test.py tf-k8s/scenarios/pending/TF-MFR-SC.7_NewFeature/
```

Checks if the test meets all requirements for TDD.

### Getting AI Assistance for Implementation

After creating a failing test and validating that it fails for the right reason, you can use the AI assistant with the implementation prompt to help implement the feature:

```bash
# Not an actual command, but conceptual flow
ai-assistant --prompt .bot/prompts/implementation.md --context "failing test: TF-MFR-SC.7_NewFeature"
```

## Best Practices

1. **Always start with a failing test** that clearly documents the expected behavior.

2. **Keep changes small and focused** - each PR should address one logical change.

3. **Use templates consistently** to maintain structure and conventions.

4. **Validate continuously** during development rather than at the end.

5. **Follow the workflow guide** in `.bot/WORKFLOW.md` for a standardized process.

## TDD Workflow Summary

1. **RED**: Create a failing test that demonstrates what's missing
2. **GREEN**: Implement the minimal code to make the test pass 
3. **REFACTOR**: Clean up and optimize without changing behavior

See `.bot/PROCESS.md` for the complete process description.

## AI-Assisted Development

The prompts in this directory are designed to guide AI assistants in generating helpful, correct, and contextual outputs for the EIDC-TF-K8s project. They enforce:

- Consistency with existing patterns
- Adherence to project requirements
- Proper explanation of changes
- Minimal, surgical implementations

When using AI assistance, always review the generated output carefully before committing.

## Maintenance

- Keep templates updated as project conventions evolve
- Ensure prompts align with current best practices
- Update validation scripts when requirements change
