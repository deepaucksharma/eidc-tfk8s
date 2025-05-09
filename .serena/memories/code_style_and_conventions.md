# NRDOT+ Internal Dev-Lab Code Style and Conventions

## Go Style Guidelines

### Code Formatting
- Use `gofmt` or `go fmt` to format code
- Follow the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments) style guide
- Maximum line length of 120 characters where practical

### Naming Conventions
- Use CamelCase for exported names (public) and camelCase for non-exported names (private)
- Use descriptive names that explain the purpose of the variable, function, or package
- Acronyms should be consistently cased (e.g., `HTTPServer` or `httpServer`, not `HttpServer`)
- Package names should be concise, lowercase, and avoid underscores
- Test files should be named `{source_file}_test.go`

### Code Organization
- Each Function Block should be in its own package under `pkg/fb/`
- Common utilities should be in the `internal/common/` directory
- API definitions (protobuf, CRDs) should be in the `pkg/api/` directory
- Entry points (main packages) should be in the `cmd/` directory
- Tests should be co-located with the code they test

### Error Handling
- Return errors rather than using panic
- Error messages should be capitalized and not end with punctuation
- Use error wrapping (`fmt.Errorf("doing X: %w", err)`) to provide context
- Log errors at the point where they're handled, not where they're generated

### Logging
- Use structured JSON logging
- Include relevant context in log fields (e.g., batch_id, fb_id, error_code)
- Use appropriate log levels (info, warning, error)
- Use ISO-8601 timestamps

### Testing
- Aim for high test coverage, especially for critical components
- Write table-driven tests where appropriate
- Use testify for assertions and mocks
- Mock external dependencies for unit tests

## Documentation Conventions

### Code Documentation
- All exported types, functions, and methods must have documentation comments
- Follow Go's documentation conventions (start with the name of the thing being documented)
- Include examples for complex functions or types

### Package Documentation
- Each package should have a package-level doc comment in one of its files

### Project Documentation
- Design documents, runbooks, and user guides should be in the `docs/` directory
- Use Markdown for documentation

## Metrics and Observability

### Metric Naming
- Follow Prometheus naming conventions (snake_case)
- Prefix all metrics with `fb_` or appropriate component name
- Include appropriate labels for high cardinality dimensions

### Tracing
- Use W3C trace context propagation
- Spans should be named `<fb>-<operation>`
- Include batch_id and config_generation in span attributes

## Versioning and Releases

### Version Numbering
- Follow Semantic Versioning (MAJOR.MINOR.PATCH)
- Version should be injected at build time via ldflags

### Tagging
- Tag releases in git with format `v{version}`
