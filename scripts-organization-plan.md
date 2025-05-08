# Repository Organization Plan

## Current Structure Assessment

The repository currently has the following structure in the `.ci` directory:
- `.ci/check_docs_drift.py`
- `.ci/generate_docs.py`
- `.ci/nrdotplus_validation.yml`
- `.ci/validate_schemas.py`

And the following GitHub workflow files:
- `.github/workflows/check-spec.yml`
- `.github/workflows/nightly.yml`

## Recommended Organization

Based on the TDD process described in PROCESS.md and standard CI/CD practices, I recommend:

1. **CI Scripts Organization**
   - Create a `.ci/scripts/` directory for all Python scripts
   - Move existing scripts from `.ci/` to `.ci/scripts/`
   - Add new scripts from the branch synchronization process
   - Create a `.ci/templates/` directory for Jinja2 templates

2. **GitHub Workflows**
   - Add the missing workflow files from the branch synchronization:
     - `.github/workflows/coverage-drift.yml`
     - `.github/workflows/tf-k8s-functional.yml`
   - Ensure all workflow files reference the correct script paths

3. **Documentation**
   - Update any README files or documentation to reflect the new organization
   - Add a README.md to the `.ci` directory explaining the purpose of each script

## Implementation Steps

1. Create the necessary directory structure
2. Move existing scripts to their appropriate locations
3. Add new scripts from the branch synchronization
4. Update any script paths in workflow files
5. Commit the changes with a clear message explaining the reorganization

This plan follows the "single-source-of-truth" principle from PROCESS.md and provides a clear, maintainable structure for CI/CD automation.
