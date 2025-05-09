# Task Completion Checklist for NRDOT+ Internal Dev-Lab

When completing a development task for the NRDOT+ Internal Dev-Lab project, ensure the following steps are taken:

## 1. Code Quality

- [ ] Code follows the Go style guide and project conventions
- [ ] All exported functions, types, and variables have documentation comments
- [ ] No hardcoded configuration values (use flags, environment variables, or configuration files)
- [ ] Error handling is robust and follows project conventions
- [ ] No unnecessary dependencies have been introduced

## 2. Testing

- [ ] Unit tests cover the new functionality
- [ ] Tests pass locally (`go test ./...`)
- [ ] Integration tests have been added for new features if applicable
- [ ] Performance testing has been done for performance-critical code
- [ ] SLO tests have been run if applicable

## 3. Observability

- [ ] Appropriate metrics have been added/updated
- [ ] Traces are properly propagated through the code
- [ ] Log messages are clear, structured, and at appropriate levels
- [ ] Health and readiness checks work correctly

## 4. Documentation

- [ ] Code is properly documented with comments
- [ ] README/documentation has been updated if necessary
- [ ] Any new configuration options are documented
- [ ] Runbooks have been updated if operational procedures changed

## 5. Deployment and Integration

- [ ] Docker image builds successfully
- [ ] Helm charts have been updated if necessary
- [ ] CRD definitions have been updated if necessary
- [ ] New component works correctly with other components in the pipeline

## 6. Security

- [ ] No sensitive information is leaked through logs or metrics
- [ ] PII handling follows the project's data protection guidelines
- [ ] No security vulnerabilities have been introduced
- [ ] Proper authentication and authorization are in place if applicable

## 7. Code Reviews and Approvals

- [ ] Code has been reviewed by at least one other team member
- [ ] All review comments have been addressed
- [ ] Required approvals have been obtained
- [ ] Security review has been completed if necessary

## 8. Versioning and Changelog

- [ ] Version numbers have been updated if necessary
- [ ] CHANGELOG.md has been updated with new features/fixes
- [ ] Git commit messages are clear and follow project conventions

## 9. Final Verification

- [ ] Build passes in CI/CD pipeline
- [ ] Integration tests pass in CI/CD pipeline
- [ ] Changes have been tested in a lab environment
- [ ] Component can be deployed alongside the rest of the system
