# Suggested Commands for NRDOT+ Internal Dev-Lab

## Development Commands

### Building

```bash
# Build all Function Blocks
docker-compose build

# Build a specific Function Block
docker build -t nrdot-internal-devlab/fb-rx:latest -f pkg/fb/rx/Dockerfile .
```

### Running Locally

```bash
# Start all components with Docker Compose
docker-compose up -d

# Start specific components
docker-compose up -d fb-rx fb-en-host fb-cl

# Check running containers
docker-compose ps

# View logs for a specific service
docker-compose logs -f fb-rx
```

### Testing

```bash
# Run all unit tests
go test ./...

# Run tests for a specific package
go test ./pkg/fb/rx/...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run integration tests
./scripts/run-integration-tests.sh
```

### Linting and Formatting

```bash
# Format code
gofmt -s -w .

# Lint code
go vet ./...
staticcheck ./...
```

### Kubernetes Deployment

```bash
# Create a local Kubernetes cluster using k3d
k3d cluster create nrdot-devlab --agents 3 --registry-create registry.localhost:5000

# Install the CRDs
kubectl apply -f deploy/k8s/crds/

# Deploy with Helm
helm install nrdot-devlab ./deploy/helm --values ./deploy/helm/values-lab.yaml

# Check the status
kubectl get pods

# View logs for a pod
kubectl logs -f deployment/fb-rx

# Port forward to access services locally
kubectl port-forward service/fb-rx 4317:4317
```

## Windows-Specific Commands

### Git

```powershell
# Clone repository
git clone https://github.com/newrelic/nrdot-internal-devlab.git
cd nrdot-internal-devlab

# Create a new branch
git checkout -b feature/your-feature-name

# Check status
git status

# Add changes
git add .

# Commit changes
git commit -m "Description of changes"

# Push changes
git push origin feature/your-feature-name
```

### File Operations

```powershell
# List files
dir
# or for a more unix-like experience using PowerShell
ls

# Change directory
cd path\to\directory

# Create directory
mkdir new-directory

# Find files
Get-ChildItem -Recurse -Filter *.go | Select-String -Pattern "pattern"
# or shorter version
ls -r *.go | Select-String "pattern"

# Check file content
type file.go
# or
cat file.go
```

### Docker and Kubernetes

```powershell
# Docker commands work the same in Windows PowerShell
docker-compose up -d

# Kubernetes commands work the same in Windows PowerShell
kubectl apply -f deploy/k8s/crds/
```

### Go Commands

```powershell
# Go commands work the same in Windows PowerShell
go mod download
go test ./...
go build -o bin/fb-rx.exe ./cmd/fb/rx
```

## Utility Commands

### Monitoring

```bash
# Check Prometheus metrics for a Function Block
curl http://localhost:2113/metrics  # For FB-RX

# Check health endpoint
curl http://localhost:2113/health   # For FB-RX
```

### Configuration

```bash
# Apply a new CRD configuration
kubectl apply -f deploy/examples/pipeline-config.yaml

# Check the status of the CRD
kubectl get nrdotpluspipeline

# Describe the CRD for more details
kubectl describe nrdotpluspipeline my-pipeline
```

### Backup and Restore

```bash
# Run backup script for persistence volume
./deploy/scripts/backup-pvc.sh fb-dp

# Restore from backup
./deploy/scripts/restore-pvc.sh fb-dp backup-file.tar.gz
```

### DLQ Management

```bash
# Run the DLQ replay tool
docker-compose run --rm dlq-replay --dlq-path=/data --fb-rx-addr=fb-rx:5000 --dry-run

# Check DLQ metrics
curl http://localhost:2122/metrics
```
