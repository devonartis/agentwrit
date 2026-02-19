# AgentAuth MVP Container Deployment Specification v1.0

## Document metadata
- Version: v1.0
- Status: Proposed
- Date: 2026-02-09
- Owner: AgentAuth team

## Purpose
Define a production-like deployment baseline for MVP using Docker images so teams can run, observe, and validate AgentAuth as a real service, not only as local `go run` processes.

## Scope
In scope:
1. Docker image requirements for broker MVP.
2. Container runtime contracts (env, ports, health, security posture).
3. Logging and observability requirements for container deployments.
4. MVP release gate requirements for containerized validation.

Out of scope:
1. Full Kubernetes production platform design.
2. Full SSO/IAM federation architecture.
3. Long-term persistence architecture selection.

## Normative requirements

### 1. Image build and release
1. The broker MUST be buildable as a versioned Docker image for every MVP release.
2. The image tag MUST include a release version and immutable git SHA tag.
3. The image build MUST be reproducible from repository source without manual patching.
4. The release process MUST publish image build metadata (version, commit, build date).

### 2. Runtime contract
1. The container MUST expose the broker HTTP port (default `8080`).
2. The container MUST run as non-root where technically feasible.
3. Required runtime configuration MUST be environment-variable based (`AA_*` contract).
4. Secrets (for example `AA_ADMIN_SECRET`) MUST be injected at runtime, never baked into images.
5. A container healthcheck MUST call `/v1/health` and fail when endpoint is unavailable.

### 3. Logging and observability contract
1. Broker logs MUST be emitted to stdout/stderr in structured format.
2. Startup logs MUST include version and listen address.
3. HTTP request logs MUST include, at minimum:
   - request_id
   - method
   - path
   - status
   - latency_ms
4. Error logs MUST include request correlation fields when available.
5. `/v1/metrics` MUST be available in containerized runs for scrape/testing.
6. Container docs MUST include exact commands to view logs (`docker logs` and/or `docker compose logs -f`).

### 4. Developer and demo operations
1. MVP must provide a `docker compose` flow to start broker and supporting demo components.
2. A new developer MUST be able to run the stack and see broker logs in under 10 minutes.
3. Documentation MUST include:
   - start command
   - stop command
   - health verification command
   - log tail command
   - basic troubleshooting table

### 5. MVP release gates (containerized)
Before MVP release sign-off:
1. Image build gate MUST pass.
2. Container startup gate MUST pass (`/v1/health` reachable).
3. Live workflow gate MUST pass against containerized broker.
4. Log-evidence gate MUST pass:
   - startup log line present
   - request log lines present for exercised endpoints
   - error log line present for a controlled failing request

## Recommended implementation plan
1. Add `Dockerfile` for broker with a minimal runtime base image.
2. Add `docker-compose.yml` for local MVP stack.
3. Add container smoke script (`scripts/live_test_compose.sh`) that validates health, key endpoints, and log evidence.
4. Add release checklist entries for container gates.
5. Update README and USER_GUIDE with container-first quickstart section.

## Acceptance criteria
1. `docker build` produces a runnable broker image.
2. `docker compose up` starts broker with health endpoint passing.
3. Live MVP flow succeeds against compose deployment.
4. Logs clearly show startup, request traffic, and controlled error path.
5. New developer can run stack + inspect logs using documented commands only.

## Risks and tradeoffs
1. Containerization adds release complexity but improves deployment realism.
2. If HTTP request logging is missing, container deployment alone does not solve observability clarity.
3. In-memory state model remains an MVP limitation even in containers.

## Change control
Any deviation from this spec for MVP release must be documented in release notes with rationale and explicit risk acceptance.

