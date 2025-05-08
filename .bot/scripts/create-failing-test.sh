#!/bin/bash
# Script to streamline the creation of failing tests for TDD

set -e

# Function to show usage
function show_usage() {
  echo "Usage: create-failing-test.sh <scenario-id> <eidc-reference>"
  echo
  echo "Creates a new failing test scenario based on templates."
  echo
  echo "Arguments:"
  echo "  scenario-id     ID of the scenario (e.g., TF-MFR-EP.6_CommandLineHashing)"
  echo "  eidc-reference  Reference to EIDC section (e.g., annex_c_security.md#PII)"
  echo
  echo "Example:"
  echo "  create-failing-test.sh TF-MFR-EP.6_CommandLineHashing annex_c_security.md#PII"
  exit 1
}

# Check arguments
if [ $# -lt 2 ]; then
  show_usage
fi

SCENARIO_ID=$1
EIDC_REF=$2
REPO_ROOT=$(git rev-parse --show-toplevel)
SCENARIO_DIR="tf-k8s/scenarios/pending/$SCENARIO_ID"
TODAY=$(date +%Y-%m-%d)

# Check if scenario already exists
if [ -d "$REPO_ROOT/$SCENARIO_DIR" ]; then
  echo "Error: Scenario directory already exists: $SCENARIO_DIR"
  exit 1
fi

# Create directory structure
echo "Creating directory structure for $SCENARIO_ID..."
mkdir -p "$REPO_ROOT/$SCENARIO_DIR/k8s"

# Extract feature name from scenario ID
FEATURE_NAME=$(echo $SCENARIO_ID | sed 's/.*_//g' | sed 's/_/ /g')

# Create scenario definition from template
echo "Creating scenario definition..."
cat "$REPO_ROOT/.bot/templates/scenario_template.yaml" | \
  sed "s/TF-\[PREFIX\]-\[NUMBER\]_\[FeatureName\]/$SCENARIO_ID/g" | \
  sed "s/\[Feature Name\]/$FEATURE_NAME/g" | \
  sed "s/\[YYYY-MM-DD\]/$TODAY/g" | \
  sed "s|\[eidc/1.2/path/to/referenced/file.md#Section\]|eidc/1.2/$EIDC_REF|g" \
  > "$REPO_ROOT/$SCENARIO_DIR/scenario_definition.yaml"

# Create verification script from template
echo "Creating verification script..."
cp "$REPO_ROOT/.bot/templates/verification_template.py" "$REPO_ROOT/$SCENARIO_DIR/verify.py"
chmod +x "$REPO_ROOT/$SCENARIO_DIR/verify.py"

# Create K8s resources from template
echo "Creating Kubernetes resource templates..."
cp "$REPO_ROOT/.bot/templates/k8s_component_template.yaml" "$REPO_ROOT/$SCENARIO_DIR/k8s/component.yaml"

# Validate the created test
echo "Validating the new failing test..."
python "$REPO_ROOT/.bot/scripts/validate-failing-test.py" "$REPO_ROOT/$SCENARIO_DIR" -v

echo
echo "Successfully created failing test skeleton in: $SCENARIO_DIR"
echo
echo "Next steps:"
echo "1. Customize the scenario definition for your specific test"
echo "2. Add necessary Kubernetes resources in k8s/ directory"
echo "3. Implement the verification script to check for the failing condition"
echo "4. Run: python tf-k8s/scripts/scenario-runner.py --scenario $SCENARIO_ID"
echo
echo "After your test is reviewed and merged, work on the implementation to make it pass!"
