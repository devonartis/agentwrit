# Tech Stack - AgentAuth

## Core Backend
- **Language:** Go (Golang) - Selected for its performance, strong concurrency primitives, and robust standard library.
- **HTTP Server:** `net/http` (Standard Library) - Provides a lightweight and standard-compliant foundation for building APIs.
- **Routing:** `http.ServeMux` (Go 1.22+ style) - Simple, native pattern matching for HTTP routes.

## Security & Identity
- **Cryptography:** Ed25519 (EdDSA) - High-performance, secure digital signatures for token signing and challenge-response authentication.
- **Identity Standard:** SPIFFE (via `go-spiffe`) - Implements the Secure Production Identity Framework for Everyone to provide standard-based agent identities.
- **Tokens:** JWT (JSON Web Tokens) with EdDSA signing.

## Persistence
- **Storage:** In-memory `SqlStore` - Currently uses a thread-safe in-memory store for nonces, tokens, and agent records, designed for easy migration to a SQL database.

## Observability
- **Metrics:** Prometheus - Industry-standard metrics collection and exposure.
- **Logging:** Structured logging (via `internal/obs`) for operational visibility and auditability.

## Infrastructure
- **Containerization:** Docker - Multi-stage builds for small, secure production images.
- **Orchestration:** Docker Compose - Used for local development and service orchestration.
