# Code Style and Conventions

## General Style

- Follow the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use [gofmt](https://golang.org/cmd/gofmt/) to format code
- Document all public functions and types

## Naming Conventions

- Use camelCase for variable names
- Use PascalCase for exported names (public types, functions, constants)
- Use snake_case for file names
- Prefix interface names with 'I' (e.g., IFunctionBlock)
- Use descriptive names that convey meaning and purpose

## Package Organization

- Main executable packages in `cmd/`
- Core reusable code in `pkg/`
- Internal utilities in `internal/`
- Deployment resources in `deploy/`
- Each Function Block (FB) has its own directory under `pkg/fb/`

## Error Handling

- Use standardized error codes from `pkg/fb/interfaces.go`
- Wrap errors with context using `fmt.Errorf("context: %w", err)`
- Log errors with structured fields using the logger from `internal/common/logging`
- Use circuit breakers for handling downstream failures

## Concurrency Patterns

- Use contexts for cancellation and timeouts
- Protect shared state with mutexes (e.g., `configMu`)
- Use channels for signaling and graceful shutdown
- Follow the graceful shutdown pattern in main functions

## Configuration

- Function Blocks receive configuration from the ConfigController via gRPC
- Each FB should validate its configuration
- Configuration should be applied dynamically when possible
- Use the `config.FBConfig` for common configuration parameters

## Observability

- Log in structured JSON format using `internal/common/logging`
- Expose Prometheus metrics on port 2112 using `internal/common/metrics`
- Implement W3C-compatible traces using `internal/common/tracing`
- Include batch_id in logs and traces for correlation
- Implement health and readiness endpoints

## Testing

- Write unit tests for all packages
- Run integration tests with a Kubernetes cluster
- Validate SLOs and PII handling
