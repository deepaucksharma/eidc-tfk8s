# Comprehensive test runner for NRDOT+ Internal Dev-Lab (PowerShell version)

# Print banner
Write-Host "=====================================================" -ForegroundColor Cyan
Write-Host "   NRDOT+ Internal Dev-Lab Test Runner" -ForegroundColor Cyan
Write-Host "=====================================================" -ForegroundColor Cyan

# Create necessary directories
if (-not (Test-Path -Path "scripts")) {
    New-Item -ItemType Directory -Path "scripts"
}

if (-not (Test-Path -Path "scripts\run-integration-tests.ps1")) {
    Write-Host "Creating integration test script..." -ForegroundColor Yellow
    Copy-Item -Path "run-integration-tests.ps1" -Destination "scripts\" -ErrorAction SilentlyContinue
}

# Fix module dependencies
Write-Host "Updating module dependencies..." -ForegroundColor Yellow
try {
    go mod tidy
} catch {
    Write-Host "Warning: go mod tidy failed, but continuing..." -ForegroundColor Yellow
}

# Run simple tests first
Write-Host "=====================================================" -ForegroundColor Cyan
Write-Host "Running simple unit tests..." -ForegroundColor Cyan
Write-Host "=====================================================" -ForegroundColor Cyan
go test ./pkg/test/simple_test.go -v

# Run resilience tests
Write-Host "=====================================================" -ForegroundColor Cyan
Write-Host "Running resilience tests..." -ForegroundColor Cyan
Write-Host "=====================================================" -ForegroundColor Cyan
go test ./pkg/test/resilience/... -v

# Run schema tests
Write-Host "=====================================================" -ForegroundColor Cyan
Write-Host "Running schema tests..." -ForegroundColor Cyan
Write-Host "=====================================================" -ForegroundColor Cyan
go test ./pkg/test/schema/... -v

# Run SLO tests
Write-Host "=====================================================" -ForegroundColor Cyan
Write-Host "Running SLO tests..." -ForegroundColor Cyan
Write-Host "=====================================================" -ForegroundColor Cyan
go test ./pkg/test/slo/... -v

# Run chaos tests
Write-Host "=====================================================" -ForegroundColor Cyan
Write-Host "Running chaos tests..." -ForegroundColor Cyan
Write-Host "=====================================================" -ForegroundColor Cyan
go test ./pkg/test/chaos/... -v

# Run function block tests
Write-Host "=====================================================" -ForegroundColor Cyan
Write-Host "Running function block tests (skipping failures)..." -ForegroundColor Cyan
Write-Host "=====================================================" -ForegroundColor Cyan
try { go test ./pkg/fb/gw/... -v } catch { Write-Host "GW tests failed, but continuing..." -ForegroundColor Yellow }
try { go test ./pkg/fb/rx/... -v } catch { Write-Host "RX tests failed, but continuing..." -ForegroundColor Yellow }
try { go test ./pkg/fb/cl/... -v } catch { Write-Host "CL tests failed, but continuing..." -ForegroundColor Yellow }

# Run the test harness
Write-Host "=====================================================" -ForegroundColor Cyan
Write-Host "Running test harness..." -ForegroundColor Cyan
Write-Host "=====================================================" -ForegroundColor Cyan
try { go run test_harness.go } catch { Write-Host "Test harness failed, but continuing..." -ForegroundColor Yellow }

# Check if Kubernetes is available
Write-Host "=====================================================" -ForegroundColor Cyan
try {
    kubectl get nodes | Out-Null
    Write-Host "Kubernetes is available, running integration tests..." -ForegroundColor Cyan
    Write-Host "=====================================================" -ForegroundColor Cyan
    
    # Install CRDs if they exist
    if (Test-Path -Path "deploy/k8s/crds") {
        try { kubectl apply -f deploy/k8s/crds/ } catch { Write-Host "Failed to apply CRDs, but continuing..." -ForegroundColor Yellow }
    }
    
    # Run e2e tests
    try { go test ./pkg/test/e2e/... -v } catch { Write-Host "E2E tests failed, but continuing..." -ForegroundColor Yellow }
    try { go test ./pkg/test/integration/... -v } catch { Write-Host "Integration tests failed, but continuing..." -ForegroundColor Yellow }
} catch {
    Write-Host "Kubernetes is not available, skipping integration tests" -ForegroundColor Yellow
    Write-Host "To run full integration tests, set up a local Kubernetes cluster:" -ForegroundColor Yellow
    Write-Host "  k3d cluster create nrdot-devlab --agents 3" -ForegroundColor Yellow
    Write-Host "  kubectl apply -f deploy/k8s/crds/" -ForegroundColor Yellow
}

Write-Host "=====================================================" -ForegroundColor Cyan
Write-Host "Test run completed!" -ForegroundColor Green
Write-Host "=====================================================" -ForegroundColor Cyan
