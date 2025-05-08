# EIDC-TF-K8s TDD Workflow Guide

This document provides a structured workflow for implementing new features in the EIDC-TF-K8s project using Test-Driven Development principles. Following this workflow will ensure consistency, quality, and efficiency.

## 1. Planning Phase

Before writing any code:

1. **Review Requirements**
   - Study the EIDC specification section relevant to your task
   - Identify all stakeholders and their needs
   - Map dependencies with other components

2. **Define Success Criteria**
   - Establish clear, measurable metrics for the feature
   - Identify potential edge cases and error conditions
   - Define performance requirements if applicable

3. **Draft Test Strategy**
   - Determine which test scenarios will be needed
   - Decide on appropriate test environment (kind vs. k3d-duo)
   - Plan verification approach and tools

## 2. Red Phase (Failing Test)

Create a test that fails in the expected way:

1. **Create Scenario Structure**
   ```bash
   # Create the scenario directory
   mkdir -p tf-k8s/scenarios/pending/TF-ID-123_FeatureName/{k8s,scripts}
   
   # Use templates to create base files
   cp .bot/templates/scenario_template.yaml tf-k8s/scenarios/pending/TF-ID-123_FeatureName/scenario_definition.yaml
   cp .bot/templates/verification_template.py tf-k8s/scenarios/pending/TF-ID-123_FeatureName/verify.py
   ```

2. **Customize Test Files**
   - Adapt the scenario definition to your feature
   - Create necessary Kubernetes manifests
   - Implement verification logic

3. **Verify Failure**
   ```bash
   # Run the test to confirm it fails as expected
   python tf-k8s/scripts/scenario-runner.py --scenario TF-ID-123_FeatureName
   ```

4. **Get Failing Test PR Reviewed**
   - Create PR with just the failing test
   - Ensure it fails for the right reason
   - Get approval before proceeding

## 3. Green Phase (Implementation)

Implement the minimal code to make the test pass:

1. **Identify Required Changes**
   - Determine which files need to be modified
   - Plan the changes systematically

2. **Make Targeted Updates**
   - Implement the feature with minimal changes
   - Add detailed comments explaining the logic
   - Include `# WHY:` annotations for non-obvious choices

3. **Verify Success**
   ```bash
   # Run the test to see if it passes
   python tf-k8s/scripts/scenario-runner.py --scenario TF-ID-123_FeatureName
   ```

4. **Regression Testing**
   ```bash
   # Run all related tests to ensure no regressions
   python tf-k8s/scripts/scenario-runner.py --group mfr-components
   ```

## 4. Refactor Phase (Optimization)

Optimize the implementation without changing its behavior:

1. **Optimize Code**
   - Look for duplication or inefficiencies
   - Improve algorithm complexity if possible
   - Enhance readability

2. **Documentation Updates**
   - Update any affected documentation
   - Add examples if appropriate
   - Update change logs

3. **Final Verification**
   ```bash
   # Run full validation suite
   bash .ci/run-validation.sh
   ```

## 5. Review & Merge

Prepare the implementation for review:

1. **Prepare PR Description**
   - Explain what changed and why
   - Reference the failing test PR
   - List any issues addressed

2. **Self-Review Checklist**
   - [ ] All tests pass
   - [ ] Code follows project style guidelines
   - [ ] All new code has comments
   - [ ] Documentation is updated
   - [ ] Security implications reviewed

3. **Submit for Review**
   - Create PR with implementation changes
   - Link to original failing test PR
   - Address review comments promptly

## Best Practices

1. **Batch Efficiently**
   - Group related file changes in logical commits
   - Create directories before populating them
   - Use templates consistently

2. **Validate Continuously**
   - Test after each significant change
   - Don't wait until the end to run validation

3. **Document As You Go**
   - Update documentation alongside code changes
   - Keep a change log of modifications
   - Explain design decisions

4. **Streamline Review Process**
   - Use self-validation to catch issues before review
   - Provide context in comments for reviewers
   - Focus PRs on single logical changes

## Common Issues & Solutions

| Issue | Solution |
|-------|----------|
| Test fails unexpectedly | Check K8s resource configs, namespaces, and wait times |
| Schema validation errors | Verify against examples in schemas/ directory |
| Pipeline doesn't process data | Check OTTL expressions and attribute references |
| Test passes locally but fails in CI | Ensure all dependencies are in requirements.txt |
| Drift detection triggers | Run generate_docs.py locally before submitting |

Following this workflow will help maintain the quality and consistency of the EIDC-TF-K8s project while ensuring efficient development.
