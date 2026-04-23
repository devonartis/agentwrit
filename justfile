# AgentAuth Core — Task Runner
# Install: brew install just

# --- Variables (override via env or `just --set key value`) ---

admin_secret  := env_var_or_default("AA_ADMIN_SECRET", "")
broker_url    := env_var_or_default("BROKER_URL", "http://localhost:8080")
host_port     := env_var_or_default("AA_HOST_PORT", "8080")

# --- Build ---

# Build both binaries to ./bin/
build:
    go build -o bin/broker ./cmd/broker
    go build -o bin/awrit ./cmd/awrit

# Build only the broker binary
build-broker:
    go build -o bin/broker ./cmd/broker

# Build only the CLI tool
build-awrit:
    go build -o bin/awrit ./cmd/awrit

# Build the Docker image locally (dev mode)
image:
    docker compose build --no-cache broker

# --- Test ---

# Unit tests (short mode)
test:
    go test -short -count=1 ./...

# Unit tests with race detector
test-race:
    go test -race -count=1 ./...

# Fast dev-loop gates (build/vet/lint/format/tests/security)
gate-task:
    ./scripts/gates.sh task

# Full CI-mirror gates (task + race + docker + smoke + sbom)
gate-full:
    ./scripts/gates.sh full

# L4 regression — runs all tests/*/regression.sh
gate-regression:
    ./scripts/gates.sh regression

# L2.5 smoke test (broker must be running)
smoke:
    ./scripts/smoke/core-contract.sh

# --- Stack (Docker) ---

# Build image and bring up the broker
up:
    ./scripts/stack_up.sh

# Tear down the broker stack
down:
    ./scripts/stack_down.sh

# Follow broker logs
logs:
    docker compose logs -f broker

# --- Stack (VPS mode — bare binary) ---

# Build and run broker directly (requires AA_ADMIN_SECRET)
run: build-broker
    ./bin/broker --admin-secret "{{admin_secret}}"

# --- Operator CLI (awrit) ---

# Initialize config and generate admin secret
init mode="dev":
    ./bin/awrit init --mode {{mode}}

# Register an app (prints client_id + client_secret)
app-register name scopes:
    ./bin/awrit app register --name {{name}} --scopes {{scopes}}

# List registered apps
app-list:
    ./bin/awrit app list

# Query audit trail (optional filters)
audit *args:
    ./bin/awrit audit {{args}}

# Release (self-revoke) a token
token-release token:
    ./bin/awrit token release --token {{token}}

# --- Utilities ---

# Check broker health
health:
    curl -sf {{broker_url}}/v1/health | jq .

# Format all Go code
fmt:
    gofmt -w .

# Run go vet
vet:
    go vet ./...

# Run linter
lint:
    golangci-lint run ./...

# Generate a random admin secret
secret:
    openssl rand -base64 32

# Show gate list (for CI parity checks)
gates:
    ./scripts/gates.sh --list-gates
