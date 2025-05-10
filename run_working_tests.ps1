# Comprehensive test runner for NRDOT+ Internal Dev-Lab (PowerShell version)
# This script runs only the working tests

# Print banner
Write-Host "=====================================================" -ForegroundColor Cyan
Write-Host "   NRDOT+ Internal Dev-Lab Test Runner (Working Tests Only)" -ForegroundColor Cyan
Write-Host "=====================================================" -ForegroundColor Cyan

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

# Run the test harness
Write-Host "=====================================================" -ForegroundColor Cyan
Write-Host "Running test harness..." -ForegroundColor Cyan
Write-Host "=====================================================" -ForegroundColor Cyan
go run test_harness.go

Write-Host "=====================================================" -ForegroundColor Cyan
Write-Host "Test run completed!" -ForegroundColor Green
Write-Host "=====================================================" -ForegroundColor Cyan
