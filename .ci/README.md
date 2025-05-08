# CI/CD Scripts and Configurations

This directory contains CI/CD scripts and configurations for the EIDC-TF-K8s project.

## Directory Structure

- **scripts/**: Python scripts for CI/CD automation
  - `check_docs_drift.py`: Detects documentation drift between schema and docs
  - `generate_docs.py`: Generates documentation from schemas
  - `generate_scorecard.py`: Creates scorecard reports from test results
  - `validate_schemas.py`: Validates schemas against their definitions

- **templates/**: Template files used by the scripts
  - `metric_schema.md.j2`: Jinja2 template for metric schema documentation

- **nrdotplus_validation.yml**: Configuration for NR.+ validation

## Usage

These scripts are primarily used by GitHub Actions workflows but can also be run locally.

### Running Locally

```bash
# Validate all schemas
python .ci/scripts/validate_schemas.py

# Generate documentation
python .ci/scripts/generate_docs.py

# Generate scorecard
python .ci/scripts/generate_scorecard.py

# Check for documentation drift
python .ci/scripts/check_docs_drift.py
```

## GitHub Workflows

These scripts are used by the following GitHub workflows:

- `.github/workflows/check-spec.yml`: Validates the EIDC specification
- `.github/workflows/coverage-drift.yml`: Checks for documentation drift
- `.github/workflows/nightly.yml`: Runs nightly validation and generates scorecards
- `.github/workflows/tf-k8s-functional.yml`: Runs functional tests

## Maintaining CI Scripts

When updating these scripts:

1. Follow the TDD process outlined in `.bot/PROCESS.md`
2. Ensure backward compatibility or update all references
3. Test locally before committing
4. Update this README if scripts or workflows change significantly
