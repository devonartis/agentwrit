# How AgentAuth Works — Process Maps for Everyone

**Date:** 2026-02-28
**Who is this for:** Anyone. No technical knowledge required.

---

## What Problem Does AgentAuth Solve?

AI programs (called "agents") need permission to do things — read files, call APIs, access databases. The problem is: **how do you give an AI temporary, limited permission without handing it the keys to everything?**

Think about it like a hotel:
- You don't give guests a master key. You give them a room key that works for one room, for the length of their stay.
- If a guest causes problems, you can deactivate their key instantly.
- You keep a log of every door that was opened and by whom.

**AgentAuth is the key card system for AI agents.** It issues temporary passes, limits what each pass can do, and logs everything.

---

## The Four People Involved

### 1. The Building Manager (Operator)

This is the person who runs the AgentAuth system. They decide:
- Which apps are allowed to use the system
- What each app is allowed to do (read data? write data? both?)
- When to shut someone down if something goes wrong

**They use a tool called `aactl`** — think of it as the management console.

**They are the only person with the master key (admin secret).** Nobody else gets it.

### 2. The Tenant (3rd Party Developer)

This is a developer who wants their app to use AgentAuth. They:
- Ask the Building Manager to register their app
- Receive a one-time entry pass (activation token)
- Build their app to request room keys through a helper (the sidecar)

**They never get the master key.** They only get what they need.

### 3. The App (Running Software)

This is the actual software running in production. It:
- Starts up and connects to its helper (sidecar)
- Asks the helper for room keys whenever an AI agent needs one
- Handles problems (helper is down, key expired, permission denied)

**The app never talks to the main security desk (broker) directly.** It always goes through its helper.

### 4. The Guest (AI Agent)

This is the AI program that needs to do work. It:
- Receives a temporary key card (JWT token)
- Uses the key card to access what it needs
- Returns the key when done

**The guest can only access what the key card allows.** If the key says "read files in project-42," that's all it can do. It can't write files. It can't read project-43.

---

## How It All Fits Together

```
┌─────────────────────────────────────────────────────────────┐
│                                                             │
│   BUILDING MANAGER (Operator)                               │
│   "Register my-app with read + write permissions"           │
│         │                                                   │
│         │ gives one-time entry pass                         │
│         ▼                                                   │
│   TENANT (3rd Party Developer)                              │
│   "Here's my app, configured with the entry pass"           │
│         │                                                   │
│         │ deploys                                           │
│         ▼                                                   │
│   ┌─────────────────────────────────────┐                   │
│   │  APP (Running Software)             │                   │
│   │                                     │                   │
│   │  "I need a key for my AI agent" ──► HELPER (Sidecar)    │
│   │                                     │    │              │
│   │                                     │    │ gets key     │
│   │                                     │    │ from desk    │
│   │                                     │    ▼              │
│   │                                     │  SECURITY DESK    │
│   │  ◄── here's the key ──────────────  │  (Broker)        │
│   │                                     │                   │
│   │  gives key to AI agent              │                   │
│   │         │                           │                   │
│   │         ▼                           │                   │
│   │  GUEST (AI Agent)                   │                   │
│   │  uses key to access resources       │                   │
│   └─────────────────────────────────────┘                   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## Process 1: Registering a New App

**Who:** Building Manager (Operator)
**When:** A new developer wants to connect their app to the system
**How often:** Once per app

### What Happens, Step by Step

```
STEP 1: Developer contacts the Operator
┌──────────────────────────────────────────────────────┐
│                                                      │
│  Developer: "I'm building a code review app.         │
│  It needs to read source code and write review       │
│  comments. Can you register it?"                     │
│                                                      │
│  Operator decides what permissions make sense.       │
│  The developer asks. The operator approves.          │
│                                                      │
└──────────────────────────────────────────────────────┘
                         │
                         ▼
STEP 2: Operator registers the app
┌──────────────────────────────────────────────────────┐
│                                                      │
│  Operator runs:                                      │
│    aactl app register                                │
│      --name "code-review-app"                        │
│      --scopes "read:code, write:reviews"             │
│                                                      │
│  The system responds:                                │
│    "App registered. Here is a one-time entry pass:   │
│     act_7f3a...b2c1                                  │
│     This pass expires in 24 hours.                   │
│     It can only be used once."                       │
│                                                      │
│  This is like printing a new employee badge.         │
│  It has a name, permissions, and a one-time          │
│  activation code.                                    │
│                                                      │
└──────────────────────────────────────────────────────┘
                         │
                         ▼
STEP 3: Operator gives the entry pass to the developer
┌──────────────────────────────────────────────────────┐
│                                                      │
│  The operator sends the activation token to the      │
│  developer through a secure channel:                 │
│  - A secret manager (like Vault or AWS Secrets)      │
│  - An encrypted message                              │
│  - A secure internal tool                            │
│                                                      │
│  IMPORTANT: The operator NEVER shares the master     │
│  key (admin secret). Only the one-time entry pass.   │
│                                                      │
│  Think of it this way: you give a new employee       │
│  their badge, not the master key to the building.    │
│                                                      │
└──────────────────────────────────────────────────────┘
                         │
                         ▼
STEP 4: Done (for the operator)
┌──────────────────────────────────────────────────────┐
│                                                      │
│  The operator's job is finished. They can:           │
│  - See the app in the list: aactl app list           │
│  - Change its permissions later if needed            │
│  - Remove it if the developer no longer needs it     │
│  - Monitor what it's doing in the audit trail        │
│                                                      │
└──────────────────────────────────────────────────────┘
```

### What Can Go Wrong

| Problem | What Happens | Who Fixes It |
|---------|--------------|--------------|
| Operator gives wrong permissions | App can't do what it needs, or can do too much | Operator updates permissions |
| Entry pass expires before developer uses it | Developer can't connect | Operator creates a new one |
| Entry pass sent insecurely | Someone else could use it | It's single-use, so if the real dev uses it first, they're fine. If not — operator revokes and reissues. |

### GAP: This process does not work today.

Today, registering an app requires editing infrastructure files and sharing the master key. There is no `aactl app register` command. This is the #1 blocker.

---

## Process 2: Developer Integrating Their App

**Who:** 3rd Party Developer (Tenant)
**When:** After receiving the one-time entry pass from the operator
**How often:** Once per app (integration setup)

### What Happens, Step by Step

```
STEP 1: Developer receives the entry pass
┌──────────────────────────────────────────────────────┐
│                                                      │
│  The developer gets:                                 │
│  - An activation token (the one-time entry pass)     │
│  - The address of the AgentAuth broker               │
│  - Documentation on how to connect                   │
│                                                      │
│  They do NOT get:                                    │
│  - The master key (admin secret)                     │
│  - Direct access to the broker                       │
│  - Ability to change their own permissions           │
│                                                      │
└──────────────────────────────────────────────────────┘
                         │
                         ▼
STEP 2: Developer configures their app
┌──────────────────────────────────────────────────────┐
│                                                      │
│  The developer adds two settings to their app:       │
│                                                      │
│    AA_ACTIVATION_TOKEN = act_7f3a...b2c1             │
│    AA_BROKER_URL = https://broker.company.com        │
│                                                      │
│  That's it. Two settings.                            │
│                                                      │
│  The helper (sidecar) is bundled with their app.     │
│  It reads these settings on startup.                 │
│                                                      │
└──────────────────────────────────────────────────────┘
                         │
                         ▼
STEP 3: Developer writes integration code
┌──────────────────────────────────────────────────────┐
│                                                      │
│  When the app needs an AI agent to do work:          │
│                                                      │
│    1. Call the helper on localhost:                   │
│       POST http://localhost:8081/v1/token             │
│       {                                              │
│         "agent_name": "my-agent",                    │
│         "scope": ["read:code:project-42"],           │
│         "ttl": 300                                   │
│       }                                              │
│                                                      │
│    2. Get back a temporary key:                      │
│       {                                              │
│         "access_token": "eyJhbG...",                 │
│         "expires_in": 300                            │
│       }                                              │
│                                                      │
│    3. Give the key to the AI agent                   │
│                                                      │
│    4. When the agent is done, release the key:       │
│       POST http://localhost:8081/v1/token/release     │
│                                                      │
│  The developer ONLY talks to localhost.              │
│  The helper handles everything else behind the       │
│  scenes — the developer doesn't need to know how.   │
│                                                      │
└──────────────────────────────────────────────────────┘
                         │
                         ▼
STEP 4: Developer handles errors
┌──────────────────────────────────────────────────────┐
│                                                      │
│  Things the developer's code should handle:          │
│                                                      │
│  "403 — scope ceiling exceeded"                      │
│    → You asked for permissions your app doesn't have │
│    → Fix: request only what you're allowed to        │
│    → Or: ask the operator to expand your permissions │
│                                                      │
│  "503 — broker unavailable"                          │
│    → The security desk is temporarily down           │
│    → Fix: retry after a short wait                   │
│    → The helper may serve a cached key if it has one │
│                                                      │
│  "401 — token expired"                               │
│    → The key card timed out (they only last minutes) │
│    → Fix: request a new key from the helper          │
│                                                      │
│  "500 — internal error"                              │
│    → Something unexpected went wrong                 │
│    → Fix: log it, retry, alert your team             │
│                                                      │
└──────────────────────────────────────────────────────┘
                         │
                         ▼
STEP 5: Developer tests
┌──────────────────────────────────────────────────────┐
│                                                      │
│  Developer's test checklist:                         │
│                                                      │
│  □ Can I get a token with valid permissions?         │
│  □ Am I rejected if I ask for too much?              │
│  □ Does my app handle expired tokens?                │
│  □ Does my app work if the helper is temporarily     │
│    unavailable?                                      │
│  □ Does my app release tokens when done?             │
│                                                      │
└──────────────────────────────────────────────────────┘
```

### What the Developer NEVER Does

| They Never... | Because... |
|---------------|-----------|
| Talk to the broker directly | The helper handles all broker communication |
| See or use the master key | Only the operator has it |
| Change their own permissions | They ask the operator to do that |
| Manage other apps | They only see their own app |
| Access the audit trail | That's the operator's job |

### GAP: This integration path is partially broken today.

The sidecar (helper) currently requires the admin secret to start. A 3rd party developer would need the master key — which defeats the purpose. The activation token path needs to be implemented.

---

## Process 3: The Running App — From Startup to Shutdown

**Who:** The App itself (as running software)
**When:** Every time the app starts, runs, and stops
**How often:** Continuous — this is the production lifecycle

### Startup

```
PHASE 1: App starts
┌──────────────────────────────────────────────────────┐
│                                                      │
│  The app process starts.                             │
│  The helper (sidecar) starts alongside it.           │
│                                                      │
│  The helper has:                                     │
│  - The one-time entry pass (activation token)        │
│  - The broker's address                              │
│                                                      │
└──────────────────────────────────────────────────────┘
                         │
                         ▼
PHASE 2: Helper introduces itself to the security desk
┌──────────────────────────────────────────────────────┐
│                                                      │
│  Helper → Broker: "Here is my entry pass"            │
│  Broker → Helper: "Welcome. You are code-review-app. │
│                    You can issue keys for:            │
│                    - reading code                     │
│                    - writing reviews                  │
│                    Your credentials expire in         │
│                    15 minutes (I'll renew them)."     │
│                                                      │
│  The entry pass is now consumed — it can never be    │
│  used again. The helper has its own credentials.     │
│                                                      │
│  If this fails (bad token, broker unreachable):      │
│  → Helper retries with increasing wait times         │
│  → 1 second, 2 seconds, 4 seconds... up to 60s      │
│  → App should check helper health before proceeding  │
│                                                      │
└──────────────────────────────────────────────────────┘
                         │
                         ▼
PHASE 3: Helper is ready
┌──────────────────────────────────────────────────────┐
│                                                      │
│  Helper health check:                                │
│    GET http://localhost:8081/v1/health                │
│    → { "status": "ok" }                              │
│                                                      │
│  The app can now request keys for AI agents.         │
│                                                      │
└──────────────────────────────────────────────────────┘
```

### Normal Operation

```
PHASE 4: App requests keys for AI agents (repeats as needed)
┌──────────────────────────────────────────────────────┐
│                                                      │
│  Every time the app needs an AI agent to do work:    │
│                                                      │
│  1. App → Helper: "I need a key for agent-X          │
│     that lets it read code for project-42,           │
│     valid for 5 minutes"                             │
│                                                      │
│  2. Helper checks: "Is read:code:project-42          │
│     within my allowed permissions?"                  │
│     YES → proceed                                    │
│     NO  → reject (403 error to app)                  │
│                                                      │
│  3. Helper → Broker: "Issue a key for this agent"    │
│     Broker validates and issues signed key            │
│     Helper → App: "Here's the key"                   │
│                                                      │
│  4. App gives the key to the AI agent                │
│     Agent uses it to access resources                │
│                                                      │
│  5. When agent is done:                              │
│     App → Helper: "Release this key"                 │
│     Key is destroyed immediately                     │
│                                                      │
│  This cycle repeats for every agent task.            │
│                                                      │
└──────────────────────────────────────────────────────┘

PHASE 5: Behind the scenes — Helper renews its own credentials
┌──────────────────────────────────────────────────────┐
│                                                      │
│  The app doesn't see this, but the helper is         │
│  constantly renewing its own credentials:            │
│                                                      │
│  Every ~4 minutes (before the 5-minute expiry):      │
│    Helper → Broker: "Renew my credentials"           │
│    Broker → Helper: "Here's a fresh set"             │
│                                                      │
│  This runs forever in the background.                │
│  The app doesn't need to do anything.                │
│                                                      │
│  If renewal fails (broker down):                     │
│    Helper retries with backoff                       │
│    If helper's own credentials expire:               │
│      Helper marks itself UNHEALTHY                   │
│      App gets 503 errors until broker recovers       │
│                                                      │
└──────────────────────────────────────────────────────┘
```

### Failure Scenarios (What the App Sees)

```
SCENARIO A: Broker goes down temporarily
┌──────────────────────────────────────────────────────┐
│                                                      │
│  Timeline:                                           │
│                                                      │
│  0:00  Broker goes down                              │
│  0:01  Helper tries to renew → fails                 │
│  0:02  Helper retries → fails again                  │
│        (keeps retrying with backoff)                 │
│                                                      │
│  Meanwhile, the app can still get keys IF:           │
│  - The helper has cached keys from earlier → serves  │
│    those with a warning                              │
│  - No cached keys → app gets 503 error               │
│                                                      │
│  0:05  Helper's own credentials expire               │
│        Helper marks itself UNHEALTHY                 │
│        ALL requests from app → 503                   │
│                                                      │
│  0:08  Broker comes back online                      │
│  0:08  Helper detects recovery, renews credentials   │
│        Helper marks itself HEALTHY                   │
│        Normal operation resumes                      │
│                                                      │
│  WHAT THE APP SHOULD DO:                             │
│  - Check helper health before critical operations    │
│  - Retry failed token requests with short delays     │
│  - Have a fallback plan (queue work, show message)   │
│  - NEVER hard-crash because the helper is down       │
│                                                      │
└──────────────────────────────────────────────────────┘

SCENARIO B: App asks for too much permission
┌──────────────────────────────────────────────────────┐
│                                                      │
│  App: "Give me a key with delete:code:* permission"  │
│  Helper: "403 — your app is only allowed to read     │
│           code and write reviews. I can't give you   │
│           delete permissions."                       │
│                                                      │
│  WHAT THE APP SHOULD DO:                             │
│  - Only request permissions you actually need        │
│  - If you need more, ask the operator to expand      │
│    your app's permissions                            │
│                                                      │
└──────────────────────────────────────────────────────┘

SCENARIO C: Key expires while agent is using it
┌──────────────────────────────────────────────────────┐
│                                                      │
│  Agent tries to access a resource:                   │
│  Resource: "401 — your key has expired"              │
│                                                      │
│  WHAT THE APP SHOULD DO:                             │
│  - Request a new key from the helper                 │
│  - Give the new key to the agent                     │
│  - Agent retries the request                         │
│  - Consider requesting longer TTLs if tasks take     │
│    more than 5 minutes (max allowed by ceiling)      │
│                                                      │
└──────────────────────────────────────────────────────┘
```

### Shutdown

```
PHASE 6: App stops
┌──────────────────────────────────────────────────────┐
│                                                      │
│  When the app shuts down:                            │
│                                                      │
│  1. Release any active keys:                         │
│     For each agent that still has a key →            │
│     POST /v1/token/release                           │
│     (Good practice. Keys expire anyway, but          │
│      releasing them immediately is more secure.)     │
│                                                      │
│  2. Helper shuts down:                               │
│     - Stops accepting new requests                   │
│     - Cleans up its connection to the broker         │
│     - Removes its socket file (if using one)         │
│                                                      │
│  3. Next startup:                                    │
│     - Helper will need to re-activate                │
│     - If activation token was already used:          │
│       operator must issue a new one                  │
│     - Agent identities are recreated (they're        │
│       temporary by design)                           │
│                                                      │
└──────────────────────────────────────────────────────┘
```

### GAP: Sidecar restart requires a new activation token.

Today, the sidecar uses the admin secret to re-bootstrap every time. With the activation token model, a consumed token can't be reused. This means: either the token must be refresh-able, or restarts need a fresh token from the operator. This is an open design question.

---

## Process 4: Operator Manages a Running System

**Who:** Operator (Building Manager)
**When:** Ongoing — day-to-day operations
**How often:** As needed

### Checking What's Running

```
┌──────────────────────────────────────────────────────┐
│                                                      │
│  "What apps are registered?"                         │
│  $ aactl app list                                    │
│                                                      │
│  NAME              SCOPES                STATUS       │
│  code-review-app   read:code,write:rev   active       │
│  data-analyzer     read:data:*           active       │
│  report-gen        write:reports:*       active       │
│                                                      │
│  GAP: This command doesn't exist yet (G-1, G-2)     │
│                                                      │
└──────────────────────────────────────────────────────┘
```

### Changing an App's Permissions

```
┌──────────────────────────────────────────────────────┐
│                                                      │
│  "code-review-app now also needs to read tests"      │
│                                                      │
│  $ aactl app update                                  │
│      --name "code-review-app"                        │
│      --scopes "read:code,write:reviews,read:tests"   │
│                                                      │
│  Takes effect on the next token the app requests.    │
│  Existing tokens keep their old permissions until    │
│  they expire.                                        │
│                                                      │
│  GAP: This command doesn't exist yet (G-2)           │
│                                                      │
└──────────────────────────────────────────────────────┘
```

### Narrowing Permissions (Emergency)

```
┌──────────────────────────────────────────────────────┐
│                                                      │
│  "data-analyzer is reading too much. Remove its      │
│   write access immediately."                         │
│                                                      │
│  $ aactl app update                                  │
│      --name "data-analyzer"                          │
│      --scopes "read:data:project-42"                 │
│                                                      │
│  What happens:                                       │
│  - New ceiling is set (narrower)                     │
│  - Any existing tokens that exceed the new ceiling   │
│    are revoked immediately                           │
│  - Future token requests must fit within new ceiling │
│                                                      │
│  NOTE: The ceiling-narrowing + auto-revocation       │
│  feature already works today at the sidecar level.   │
│  What's missing is the app-level command.            │
│                                                      │
└──────────────────────────────────────────────────────┘
```

### Responding to a Security Incident

```
┌──────────────────────────────────────────────────────┐
│                                                      │
│  "An agent is doing something suspicious.            │
│   Shut it down."                                     │
│                                                      │
│  OPTION 1: Kill one specific key                     │
│  $ aactl revoke --level token --target <key-id>      │
│  → Only that one key stops working                   │
│                                                      │
│  OPTION 2: Kill everything for one agent             │
│  $ aactl revoke --level agent --target <agent-id>    │
│  → ALL keys held by that agent stop working          │
│                                                      │
│  OPTION 3: Kill everything for a task                │
│  $ aactl revoke --level task --target <task-id>      │
│  → ALL keys for every agent working on that task     │
│    stop working                                      │
│                                                      │
│  OPTION 4: Kill an entire delegation chain           │
│  $ aactl revoke --level chain --target <root-agent>  │
│  → Agent A delegated to B who delegated to C?        │
│    ALL of them lose access.                          │
│                                                      │
│  Effect is INSTANT. Next request with a revoked      │
│  key → rejected with "403 — revoked"                 │
│                                                      │
└──────────────────────────────────────────────────────┘
```

### Reviewing the Audit Trail

```
┌──────────────────────────────────────────────────────┐
│                                                      │
│  "Show me everything that was denied today"          │
│  $ aactl audit events --outcome denied               │
│                                                      │
│  "Show me everything agent-X did"                    │
│  $ aactl audit events --agent-id <agent-id>          │
│                                                      │
│  "Show me all revocations this week"                 │
│  $ aactl audit events --event-type token_revoked     │
│      --since 2026-02-21                              │
│                                                      │
│  Every operation is logged with:                     │
│  - What happened (event type)                        │
│  - Who did it (agent or operator)                    │
│  - Whether it succeeded or was denied                │
│  - When it happened (timestamp)                      │
│  - A tamper-proof hash (if someone edits an old      │
│    log entry, the math breaks and you can tell)      │
│                                                      │
│  GAP: No command to verify the hash chain (G-18).    │
│  The tamper-proof log exists but there's no way to   │
│  run the verification check yet.                     │
│                                                      │
└──────────────────────────────────────────────────────┘
```

### Removing an App

```
┌──────────────────────────────────────────────────────┐
│                                                      │
│  "We're done with data-analyzer. Remove it."         │
│                                                      │
│  $ aactl app remove --name "data-analyzer"           │
│                                                      │
│  What should happen:                                 │
│  - App record deleted                                │
│  - Sidecar credentials revoked (can't renew)         │
│  - All active agent tokens for this app revoked      │
│  - Audit event recorded                              │
│                                                      │
│  GAP: This command doesn't exist yet (G-2)           │
│                                                      │
└──────────────────────────────────────────────────────┘
```

---

## Process 5: An Agent Gets a Key

**Who:** The App (on behalf of an AI Agent)
**When:** Every time an agent needs to access a resource
**How often:** Many times per day — this is the core operation

### Step by Step (Plain Language)

```
1. App says to the Helper:
   "I need a key for my-agent that lets it read
    code for project-42, valid for 5 minutes."

2. Helper checks:
   "Am I allowed to issue keys for reading code?"
   YES → continue
   NO  → tell the app "Permission denied. Your app
          isn't allowed to grant that."

3. Helper checks if this agent already has an identity:
   YES → use the existing identity
   NO  → register the agent automatically:
         - Create a unique cryptographic fingerprint
         - Prove the fingerprint is real (challenge-response)
         - Store the identity for next time

4. Helper asks the Security Desk:
   "Issue a key for this agent with these permissions."

5. Security Desk checks:
   - Is the Helper's own credential valid? ✓
   - Is the requested permission within the app's limit? ✓
   - Is the agent registered? ✓
   → Issues a signed key card (JWT)

6. Helper gives the key to the App.
   App gives the key to the Agent.
   Agent uses the key to access resources.

7. Key expires after 5 minutes.
   If the agent needs more time, the app requests a new key.
```

---

## Process 6: One Agent Gives Another Agent Limited Access (Delegation)

**Who:** AI Agent A (delegating to AI Agent B)
**When:** Agent A needs Agent B to help with a subtask
**How often:** Occasionally

### Plain Language

```
Agent A has a key that says:
  "Can read all code and write all reviews"

Agent A needs Agent B to review ONE file.
Agent A doesn't want to give B full access to everything.

So Agent A says to the Security Desk:
  "Make a key for Agent B that ONLY lets it
   read code for project-42. Nothing else.
   Valid for 2 minutes."

Security Desk checks:
  - Can Agent A read code for project-42? YES (it can read all code)
  - Is "read code for project-42" narrower than "read all code"? YES
  → Issues a restricted key for Agent B

RULES:
  - Permissions can ONLY get narrower, never wider
  - Agent A can't give Agent B write access if A only has read
  - Maximum 5 levels of delegation (A→B→C→D→E→F, no further)
  - If Agent A's key is revoked, Agent B's key dies too
```

---

## Process 7: Auditing — Proving What Happened

**Who:** Operator (or compliance auditor)
**When:** During compliance reviews, incident investigations, or routine checks
**How often:** Regularly

### What Gets Logged

Every single operation creates a log entry. Nothing is skipped:

| When This Happens... | This Gets Logged |
|----------------------|-----------------|
| Operator logs in | "admin_auth — success" |
| Operator login fails (wrong password) | "admin_auth_failed — denied" |
| New agent registered | "agent_registered — success" |
| Agent asked for too much permission | "registration_policy_violation — denied" |
| Key issued to agent | "token_issued — success" |
| Key renewed | "token_renewed — success" |
| Key released (self-revoked) | "token_released — success" |
| Key revoked by operator | "token_revoked — success" |
| Someone used a revoked key | "token_revoked_access — denied" |
| Agent tried to exceed permissions | "scope_violation — denied" |
| Agent A delegated to Agent B | "delegation_created — success" |
| Delegation was too broad | "delegation_attenuation_violation — denied" |
| App's helper activated | "sidecar_activated — success" |

### Tamper-Proof Design

Each log entry includes a hash (a mathematical fingerprint) that depends on the PREVIOUS entry's hash. If someone modifies an old entry, every entry after it has the wrong fingerprint.

```
Entry 1: Hash = fingerprint(Entry 1 data)
Entry 2: Hash = fingerprint(Entry 1 Hash + Entry 2 data)
Entry 3: Hash = fingerprint(Entry 2 Hash + Entry 3 data)

If you change Entry 1 → Entry 1 Hash changes
→ Entry 2 expects old Entry 1 Hash → MISMATCH
→ Tampering detected
```

### GAP: No way to run this verification check yet (G-18). The math is built in, but there's no button to push to verify it.

---

## QA Testing Checklist

### For Manual QA Testers — No Technical Knowledge Required

Each test describes WHAT to verify, not HOW (the how depends on your testing tools).

#### App Registration Tests

| # | What to Test | What Should Happen | Status |
|---|-------------|-------------------|--------|
| 1 | Register a new app with a name and permissions | System creates the app and returns a one-time pass | GAP — can't test yet |
| 2 | Try to register an app with no name | System rejects with a clear error | GAP — can't test yet |
| 3 | Try to register an app with invalid permissions | System rejects with a clear error | GAP — can't test yet |
| 4 | Use the one-time pass to start the helper | Helper connects and is ready | GAP — can't test yet |
| 5 | Try to use the same one-time pass again | System rejects — it was already used | GAP — can't test yet |
| 6 | List all registered apps | Shows the apps you created | GAP — can't test yet |
| 7 | Remove an app | App is gone, its keys stop working | GAP — can't test yet |

#### Getting a Key Tests

| # | What to Test | What Should Happen | Status |
|---|-------------|-------------------|--------|
| 8 | Request a key with valid permissions | Returns a key that works | Works |
| 9 | Request a key for permissions the app doesn't have | Rejected with "permission denied" | Works |
| 10 | Request a key with a specific time limit | Key expires after that time | Works |
| 11 | Use a key that has expired | Resource rejects with "unauthorized" | Works |
| 12 | Use a key that was revoked | Resource rejects with "forbidden" | Works |

#### Revocation Tests

| # | What to Test | What Should Happen | Status |
|---|-------------|-------------------|--------|
| 13 | Revoke a specific key | Only that key stops working | Works |
| 14 | Revoke everything for one agent | ALL of that agent's keys stop working | Works |
| 15 | Revoke everything for a task | ALL keys for that task stop working | Works |
| 16 | Agent releases its own key | Key immediately stops working | Works |

#### Delegation Tests

| # | What to Test | What Should Happen | Status |
|---|-------------|-------------------|--------|
| 17 | Agent A delegates narrower permissions to Agent B | Agent B gets a restricted key | Works |
| 18 | Agent A tries to delegate MORE permissions than it has | Rejected — can't escalate | Works |
| 19 | Delegation chain longer than 5 | Rejected — too deep | Works |
| 20 | Revoke the root of a delegation chain | Everyone in the chain loses access | Works |

#### Resilience Tests

| # | What to Test | What Should Happen | Status |
|---|-------------|-------------------|--------|
| 21 | Broker goes down, app requests a key | Helper serves cached key (if available) or returns 503 | Works |
| 22 | Broker comes back after outage | Helper recovers automatically, normal operation resumes | Works |
| 23 | Helper is asked for a key before it's ready | Returns "not ready" error | Works |

#### Audit Tests

| # | What to Test | What Should Happen | Status |
|---|-------------|-------------------|--------|
| 24 | Do any operation, check audit | Audit log has an entry for it | Works |
| 25 | Do a denied operation, filter by "denied" | Only denied entries show | Works |
| 26 | Filter audit by agent or task | Only matching entries show | Works |

---

## Glossary — Plain English

| Term | What It Means |
|------|--------------|
| **Broker** | The security desk. The central service that issues and checks keys. |
| **Sidecar / Helper** | A dedicated assistant that runs alongside each app. Handles all security communication so the app doesn't have to. |
| **aactl** | The operator's management console (command-line tool). |
| **JWT / Token / Key** | A temporary digital pass. Like a hotel key card — works for limited time, limited access. |
| **Scope** | What the key allows. "read:code:project-42" means "can read code in project 42." |
| **Scope Ceiling** | The maximum any key from this app can ever allow. Set by the operator. |
| **Activation Token / Entry Pass** | A one-time code given to a developer to connect their app. Used once, then gone. |
| **Admin Secret / Master Key** | The all-access credential. Only the operator has it. Never shared with developers or apps. |
| **SPIFFE ID** | A unique identity for each agent. Like a Social Security number but for AI programs. |
| **Revocation** | Killing a key immediately. Four levels: one key, one agent, one task, or an entire delegation chain. |
| **Delegation** | Agent A giving Agent B limited access. Permissions can only get narrower. |
| **Hash Chain** | A tamper-proof log. Each entry's fingerprint depends on the previous entry. Change one, they all break. |
| **Circuit Breaker** | A safety switch. If the broker is unreachable, the helper stops trying and serves cached keys instead of overloading the network. |
| **TTL** | Time To Live. How long a key lasts before it expires (usually 5 minutes). |
| **Ed25519** | The math behind the digital signatures. Makes keys impossible to forge. |
| **Nonce** | A random one-time challenge. Used to prove identity — "sign this random text to show you're real." |
