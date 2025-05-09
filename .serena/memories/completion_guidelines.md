# Task Completion Guidelines

When completing tasks for the NRDOT+ Internal Dev-Lab project, follow these guidelines:

## Code Changes

1. **Run Linters**:
   ```bash
   go vet ./...
   staticcheck ./...
   ```

2. **Format Code**:
   ```bash
   gofmt -s -w .
   ```

3. **Run Unit Tests**:
   ```bash
   go test ./...
   ```

4. **Test Function Blocks Locally**:
   ```bash
   docker-compose up -d <fb-name>
   # Verify metrics endpoint
   curl http://localhost:<port>/metrics
   ```

## Documentation

1. **Update Documentation** if you've added a new feature or modified existing ones:
   - Update relevant README files
   - Add or update documentation in `docs/` directory
   - Update comments in code

## Deployment and Integration Testing

1. **Build and Push Docker Images**:
   ```bash
   ./scripts/build-all.sh
   ```

2. **Deploy to Test Environment**:
   ```bash
   kubectl apply -f deploy/k8s/crds/
   helm install nrdot-devlab ./deploy/helm --values ./deploy/helm/values-lab.yaml
   ```

3. **Run Integration Tests**:
   ```bash
   ./scripts/run-integration-tests.sh
   ```

## Pull Request

1. **Create a PR** with a clear description:
   - Describe the changes made
   - Link to relevant issues
   - List any manual testing performed
   - Include any relevant screenshots or logs

2. **Wait for CI Pipeline** to complete and verify all checks pass:
   - Build
   - Unit Tests
   - Integration Tests
   - SLO Validation
   - PII Test

3. **Address Review Comments** promptly and completely

## Release Process

1. **Update Version Information** in:
   - Main version constant
   - Helm chart's version
   - Any version-dependent documentation

2. **Tag Release** once merged:
   ```bash
   git tag v2.1.x
   git push origin v2.1.x
   ```
