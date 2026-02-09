# Run Demo and App (Develop Branch)

Version: 1.0  
Date: 2026-02-09

## Purpose

Run the Go broker app and the Python demo stack locally on `develop`.

This guide covers:
1. Broker startup
2. Insecure demo flow (gap)
3. Secure demo flow (fix)
4. Optional dashboard launch

## Prerequisites

1. Go toolchain available (`go` command)
2. Python 3.11+ with `venv`
3. `curl` available
4. You are in repository root: `/Users/divineartis/proj/agentAuth`

## Terminal layout

Use 4 terminals:
1. `T1` broker
2. `T2` resource server
3. `T3` agents + attacks
4. `T4` dashboard (optional)

## 1) Start the broker app (T1)

```bash
cd /Users/divineartis/proj/agentAuth
AA_SEED_TOKENS=true AA_TRUST_DOMAIN=agentauth.local AA_PORT=8080 go run ./cmd/broker
```

Expected:
1. Broker starts on `:8080`
2. Output includes:
- `SEED_LAUNCH_TOKEN=...`
- `SEED_ADMIN_TOKEN=...`

Keep these two token values for secure mode commands.

Quick health check:

```bash
curl -sS http://127.0.0.1:8080/v1/health
```

## 2) Prepare Python environment (T2 or T3 once)

```bash
cd /Users/divineartis/proj/agentAuth/demo
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
```

## 3) Run insecure mode (gap demonstration)

### 3.1 Start resource server insecure (T2)

```bash
cd /Users/divineartis/proj/agentAuth/demo
source .venv/bin/activate
python -m resource_server.main --mode insecure --port 8090
```

### 3.2 Run orchestrator insecure (T3)

```bash
cd /Users/divineartis/proj/agentAuth/demo
source .venv/bin/activate
python -m agents.orchestrator \
  --mode insecure \
  --broker-url http://127.0.0.1:8080 \
  --resource-url http://127.0.0.1:8090
```

### 3.3 Run attacks insecure (T3)

```bash
cd /Users/divineartis/proj/agentAuth/demo
source .venv/bin/activate
python -m attacks \
  --mode insecure \
  --broker-url http://127.0.0.1:8080 \
  --resource-url http://127.0.0.1:8090 \
  --shared-api-key dev-key
```

Expected demo story in insecure mode:
1. Attacks are exploitable/succeed more easily
2. Simulator output reflects insecure “gap” behavior

## 4) Run secure mode (fix demonstration)

### 4.1 Restart resource server in secure mode (T2)

Stop previous server (`Ctrl+C`), then run:

```bash
cd /Users/divineartis/proj/agentAuth/demo
source .venv/bin/activate
python -m resource_server.main --mode secure --port 8090
```

### 4.2 Run orchestrator secure (T3)

Use seed tokens printed by broker in T1:

```bash
cd /Users/divineartis/proj/agentAuth/demo
source .venv/bin/activate
python -m agents.orchestrator \
  --mode secure \
  --broker-url http://127.0.0.1:8080 \
  --resource-url http://127.0.0.1:8090 \
  --launch-token "<SEED_LAUNCH_TOKEN>" \
  --admin-token "<SEED_ADMIN_TOKEN>"
```

### 4.3 Run attacks secure (T3)

```bash
cd /Users/divineartis/proj/agentAuth/demo
source .venv/bin/activate
python -m attacks \
  --mode secure \
  --broker-url http://127.0.0.1:8080 \
  --resource-url http://127.0.0.1:8090 \
  --admin-token "<SEED_ADMIN_TOKEN>"
```

Expected demo story in secure mode:
1. Attack paths are blocked/reduced
2. Simulator output reflects secure “fix” behavior

## 5) Optional: run dashboard UI (T4)

```bash
cd /Users/divineartis/proj/agentAuth/demo
source .venv/bin/activate
python -m dashboard.main --port 8070
```

Open:

```text
http://127.0.0.1:8070
```

Use dashboard for live event/status visualization while running flows.

## 6) Shutdown

1. Stop dashboard/resource server/broker with `Ctrl+C`
2. Deactivate venv when done:

```bash
deactivate
```

## Troubleshooting

1. Port in use:

```bash
lsof -nP -iTCP:8080 -sTCP:LISTEN
lsof -nP -iTCP:8090 -sTCP:LISTEN
lsof -nP -iTCP:8070 -sTCP:LISTEN
```

2. Secure mode registration failures:
- Ensure broker started with `AA_SEED_TOKENS=true`
- Ensure `--launch-token` is set from broker output

3. 401/403 in secure mode:
- Verify resource server is running with `--mode secure`
- Verify broker is reachable at `http://127.0.0.1:8080`
