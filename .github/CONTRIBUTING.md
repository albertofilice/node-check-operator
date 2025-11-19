# Contributing to Node Check Operator

Thank you for your interest in contributing to Node Check Operator! This document provides guidelines and instructions for contributing.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/albertofilice/node-check-operator.git`
3. Create a branch: `git checkout -b feature/your-feature-name`
4. Make your changes
5. Commit your changes: `git commit -m 'Add some feature'`
6. Push to the branch: `git push origin feature/your-feature-name`
7. Open a Pull Request

## Development Setup

### Prerequisites

- Go 1.21 or later
- Docker or Podman
- kubectl or oc
- Node.js 18+ (for console plugin development)
- Access to a Kubernetes/OpenShift cluster for testing

### Building the Operator

```bash
# Build operator binary
go build -o bin/manager main.go

# Build Docker images
./scripts/build.sh --registry quay.io/YOUR_ORG --version v1.0.0
```

### Building the Console Plugin

```bash
cd console-plugin
npm install
npm run build
```

### Running Tests

```bash
# Run Go tests
go test ./...

# Run with coverage
go test -cover ./...
```

## Code Style

- Follow Go conventions and best practices
- Use `gofmt` to format code
- Run `golangci-lint` before submitting PRs
- Write clear, descriptive commit messages

## Commit Messages

Follow conventional commit format:

```
type(scope): subject

body (optional)

footer (optional)
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`

## Pull Request Process

1. Update documentation if needed
2. Add tests for new features
3. Ensure all tests pass
4. Update CHANGELOG.md if applicable
5. Request review from maintainers

## Reporting Issues

- Use the bug report template
- Provide clear steps to reproduce
- Include relevant logs and environment details
- Check existing issues before creating new ones

## Feature Requests

- Use the feature request template
- Describe the use case clearly
- Consider alternatives and trade-offs

## Questions?

- Open a discussion for questions
- Check existing issues and discussions first
- Be respectful and patient

Thank you for contributing!

