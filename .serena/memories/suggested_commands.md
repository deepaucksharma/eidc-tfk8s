# Suggested Commands for NRDOT+ Internal Dev-Lab

## Development Setup

### Install prerequisites
```bash
# Install Go 1.21+ (Windows)
# Download from https://golang.org/dl/ and run the installer

# Install Docker Desktop (Windows)
# Download from https://www.docker.com/products/docker-desktop

# Install kubectl
curl -LO "https://dl.k8s.io/release/v1.28.0/bin/windows/amd64/kubectl.exe"
# Move to a directory in your PATH

# Install Helm
curl -fsSL -o get_helm.ps1 https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3.ps1
.\get_helm.ps1

# Install k3d (local Kubernetes)
curl -LO https://github.com/k3d-io/k3d/releases/download/v5.5.1/k3d-windows-amd64.exe
# Move to a directory in your PATH
```

### Project setup
```bash
# Clone the repository
git clone https://github.com/newrelic/nrdot-internal-devlab.git
cd nrdot-internal-devlab

# Download dependencies
go mod download

# Create a local Kubernetes cluster
k3d cluster create nrdot-devlab --servers 1 --agents 3 --registry-create nrdot-registry:5000
```

## Building & Running

### Build individual Function Blocks
```bash
# Build FB-RX
cd cmd/fb/rx
go build -o fb-rx.exe main.go

# Build ConfigController
cd cmd/configcontroller
go build -o config-controller.exe main.go
```

### Build Docker Images
```bash
# Build FB-RX Docker image
docker build -t nrdot-internal-devlab/fb-rx:latest -f pkg/fb/rx/Dockerfile .

# Build ConfigController Docker image
docker build -t nrdot-internal-devlab/config-controller:latest -f cmd/configcontroller/Dockerfile .
```

### Run locally (development mode)
```bash
# Run FB-RX locally
cd cmd/fb/rx
go run main.go --config-service=localhost:5000 --next-fb=localhost:5001

# Run ConfigController locally
cd cmd/configcontroller
go run main.go --grpc-port=5000
```

## Testing

### Run unit tests
```bash
# Run all tests
go test ./...

# Run tests for a specific package
go test ./pkg/fb/rx/...

# Run tests with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

### Run SLO tests
```bash
# Run SLO tests
cd pkg/test/slo
go test -v
```

## Kubernetes Deployment

### Install CRDs
```bash
kubectl apply -f deploy/k8s/crds/nrdotpluspipeline.yaml
```

### Deploy with Helm
```bash
# Install with default values
helm install nrdot-devlab ./deploy/helm

# Install with lab values
helm install nrdot-devlab ./deploy/helm --values ./deploy/helm/values-lab.yaml

# Upgrade existing deployment
helm upgrade nrdot-devlab ./deploy/helm --values ./deploy/helm/values-lab.yaml
```

### Check deployment status
```bash
kubectl get pods -n default
kubectl get nrdotpluspipelines
kubectl describe nrdotpluspipeline default-pipeline
```

## Backup & Restore

### Backup PVCs
```bash
# Run backup script
.\deploy\scripts\backup-restore\backup-pvc.sh

# Specify namespace
$env:NAMESPACE="nrdot-lab"; .\deploy\scripts\backup-restore\backup-pvc.sh
```

### Restore PVCs
```bash
# Restore a specific PVC from backup
.\deploy\scripts\backup-restore\restore-pvc.sh fb-dp-data C:\backups\fb-dp-data-20250509-120000.tar.gz
```

## Useful Kubernetes Commands

```bash
# Get logs for a pod
kubectl logs <pod-name>

# Get logs for a specific container in a pod
kubectl logs <pod-name> -c <container-name>

# Forward ports to a pod
kubectl port-forward <pod-name> 8080:2112

# Describe a pod to see its status
kubectl describe pod <pod-name>

# Exec into a pod
kubectl exec -it <pod-name> -- sh

# Watch pod status
kubectl get pods -w
```

## Git Commands

```bash
# Check status of files
git status

# Create a new branch
git checkout -b feature/new-feature

# Commit changes
git add .
git commit -m "Description of changes"

# Push to remote
git push origin feature/new-feature

# Pull latest changes
git pull
```

## Performance Testing

```bash
# Run load generator
cd pkg/test/loadgen
go run main.go --rate=100 --target=nrdot-devlab-fb-rx:4317

# Check metrics
curl http://nrdot-devlab-fb-rx:2112/metrics | findstr fb_rx
```

## Tools for Windows

Remember that Windows uses different commands than Unix/Linux systems:
- Use `dir` instead of `ls`
- Use `type` instead of `cat`
- Use `copy` instead of `cp`
- Use `move` instead of `mv`
- Use `del` instead of `rm`
- Use `mkdir` instead of `mkdir -p` for creating directories

Windows PowerShell provides additional Unix-like commands:
- `ls` (aliased to `Get-ChildItem`)
- `cat` (aliased to `Get-Content`)
- `cp` (aliased to `Copy-Item`)
- `mv` (aliased to `Move-Item`)
- `rm` (aliased to `Remove-Item`)
