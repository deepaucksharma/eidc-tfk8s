#!/bin/bash
# Run integration tests for NRDOT+ Internal Dev-Lab

set -e  # Exit on error

echo "Running integration tests..."

# Make sure dependencies are up to date
go mod tidy

# Run unit tests
echo "=== Running Unit Tests ==="
go test -v ./pkg/test/... ./pkg/fb/cl/... ./pkg/fb/gw/... ./pkg/fb/rx/...

# Check if Kubernetes is available
if command -v kubectl &> /dev/null && kubectl get nodes &> /dev/null; then
    echo "=== Running Kubernetes Integration Tests ==="
    # Apply CRDs if needed
    if [ -d "deploy/k8s/crds" ]; then
        kubectl apply -f deploy/k8s/crds/
    fi
    
    # Run e2e tests
    go test -v ./pkg/test/e2e/...
else
    echo "WARNING: Kubernetes is not available, skipping Kubernetes integration tests"
    echo "To run full integration tests, set up a local Kubernetes cluster:"
    echo "  k3d cluster create nrdot-devlab --agents 3"
    echo "  kubectl apply -f deploy/k8s/crds/"
fi

echo "Integration tests completed!"
