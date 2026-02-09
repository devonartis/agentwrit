# Contributing to AgentAuth

Thank you for your interest in contributing to AgentAuth. This document explains the process for contributing changes.

## Getting Started

1. Fork the repository
2. Clone your fork locally
3. Create a feature branch from `develop`
4. Make your changes
5. Submit a pull request to `develop`

## Development Setup

### Go Broker

```bash
go build ./...
go test ./...
```

### Python Demo

```bash
cd demo
pip install -r requirements.txt
python -m pytest -v
```

## Branch Naming

Feature branches follow the pattern: `feature/<topic>`

Examples:
- `feature/add-rate-limiting`
- `feature/fix-token-renewal`

## Commit Messages

This project uses [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(scope): add new capability
fix(scope): correct a bug
docs(scope): update documentation
test(scope): add or modify tests
refactor(scope): restructure without behavior change
```

Examples:
- `feat(token): add configurable max TTL`
- `fix(authz): handle expired token edge case`
- `docs(api): add delegation endpoint examples`

## Pull Request Process

1. Ensure all tests pass: `go test ./...`
2. Run the linter: `golangci-lint run ./...`
3. Update documentation if your change affects:
   - API endpoints (`docs/API_REFERENCE.md`, `docs/api/openapi.yaml`)
   - User workflows (`docs/USER_GUIDE.md`)
   - Architecture (`docs/DEVELOPER_GUIDE.md`)
4. Update `CHANGELOG.md` under `[Unreleased]`
5. Submit your PR against the `develop` branch

## Code Standards

- Follow existing naming conventions (`IdSvc`, `TknSvc`, `ValMw`, etc.)
- Use `internal/obs` for structured logging
- Return RFC 7807 `application/problem+json` for HTTP errors
- Write tests for all new functionality
- Prefer stdlib over external dependencies unless the dependency prevents significant reinvention

## Security

If you discover a security vulnerability, please do **not** open a public issue. Instead, follow the [Security Policy](SECURITY.md) for responsible disclosure.

## Questions

Open an issue for questions about contributing or the codebase.
