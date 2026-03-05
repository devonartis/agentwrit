# NIST NCCoE Public Comment — AgentAuth

Responses to the NIST NCCoE concept paper on AI agent identity and access management, grounded in implementation experience with the Ephemeral Agent Credentialing pattern.

---

## 1. General Questions to Inform Choice of Demonstration Use Case

### What enterprise use-cases are organizations currently using agents for?

Enterprise agent deployments vary significantly in complexity. I find it useful to organize them by identity complexity, not because identity is the only lens, but because it determines how you scope controls as deployments mature.

**Tier 1: Single-Agent, Bounded Scope (widely deployed).** Back-office automation (document processing, data extraction, log triage), customer-facing applications (support agents, recommendation engines, claims intake, scheduling), and developer tooling (code completion, test suggestions).

**Tier 2: Multi-Step, Multi-Resource Workflows (active pilots).** Onboarding orchestration spanning HR, IT, payroll, and access management; end-to-end customer service resolution across CRM, billing, and logistics; financial transaction processing; IT helpdesk automation; and supply chain document processing covering purchase orders, invoices, and compliance certificates across ERP systems and supplier portals.

**Tier 3: Orchestrated Multi-Agent (deployed/emerging).** Coding agents that delegate across file search, shell execution, test running, and git operations. Insurance claims processing that coordinates document extraction, fraud scoring, and policy lookup across a single workflow. In most current deployments, sub-agents inherit the invoking user's identity rather than receiving scoped credentials — a gap worth noting as these patterns scale.

**Tier 4: Autonomous, Cross-Organizational Ecosystems (emerging).** Security incident response chains approximated today by SOAR platforms, continuous compliance monitoring with auto-remediation, and — more speculatively — procurement agents negotiating directly with vendor agents across organizational boundaries.

Should the project proceed as a demonstration, Tiers 2 and 3 offer the strongest starting points — complex enough to exercise real-world patterns, common enough for findings to apply broadly.

### What risks worry you about agents?

The risk I would highlight most is **credential overexposure at scale**. Agent tasks complete in minutes, but tokens live for hours and many frameworks skip rotation and revocation entirely. That gap is unnecessary attack surface, and it compounds with concurrency. The data reflects this: 97% of non-human identities carry excessive privileges (Entro Security, 2025), machine identities outnumber humans 45:1 to 92:1, and 40% sit unused but remain enabled (OWASP NHI Top 10).

The second is **delegation exploitation** — no widely adopted mechanism exists to verify authorization lineage or enforce permission narrowing across agent hops. The December 2025 LangGrinch vulnerability (CVE-2025-68664, CVSS 9.3) demonstrated this in practice: a serialization flaw in langchain-core enabled secret extraction from environment variables, reachable from a single prompt, through the framework's normal serialization path.

The third risk concerns me most, and I believe it is the least recognized. Enterprises routinely allow agents to **assume the full identity of the user who launched them** — not as a deliberate decision, but as an unexamined default. The agent inherits the user's credentials, acts in their name, and least privilege disappears entirely. What makes this hard to see as a risk is that it mimics accountability: the audit log shows a human approved the action. But that human may not have understood what they were approving, the developer who wrote the agent determined what it would actually do, and the identity layer captured none of that distinction. Human-in-the-loop becomes accountability theater.

The deeper problem is that the security community is applying deterministic thinking to a non-deterministic actor. Traditional IAM was built for predictable principals — humans, or service accounts like EC2 or S3 with bounded, consistent call patterns you can write policy around. Security teams are aware of AI-specific threats like jailbreaks and prompt injection, and of familiar risks like data leakage. But those concerns, valid as they are, sit above an architectural problem neither addresses: an LLM-driven agent's tool calls, delegation decisions, and action sequences vary with context in ways no static policy anticipates. It is not a service account. Treating it as one leaves a gap existing IAM frameworks were never designed to close.

### What support are you seeing for new protocols such as Model Context Protocol (MCP)?

MCP adoption is accelerating. The November 2025 spec update introduced Client ID Metadata Documents (CIMD) as the default client registration mechanism, with early implementations at Stytch and WorkOS. One thing worth noting from implementation experience: CIMD proves domain ownership, not trustworthiness. Trust policies still need to layer on top — allow lists, differentiated trust levels. The demonstration project could help practitioners understand that distinction.

### In what ways do agentic architectures introduce identity and authorization challenges?

Three stand out from implementation experience. First, **credential lifetime mismatch** — agent tasks are short-lived, but credentials routinely outlast them, creating unnecessary exposure. Second, **unpredictable resource needs** — because LLM reasoning is non-deterministic, permissions cannot be fully defined at deployment time, which breaks any static policy model. Third, **delegation chain integrity** — your paper raises this under Access Delegation (page 6), and from practical experience, it deserves first-class treatment. Multi-agent workflows are becoming the norm, and unlike the first two challenges, delegation chain integrity cannot be solved at the individual agent level; it requires verification across trust boundaries that current standards do not yet address.

### What are the core characteristics of agentic architectures?

Your Figure 1 and the flow diagram in Appendix A capture the architecture well. The core characteristics are: (1) ephemeral task lifetimes; (2) non-deterministic behavior; (3) dynamic resource needs discovered at runtime; (4) iterative reasoning loops where the agent may query tools and data sources multiple times within a single task; (5) tool and resource integration — the ability to take action against external systems, not just generate responses; (6) multi-agent delegation chains; and (7) mixed trust levels where agents interact with both internal and external tools within a single task. Your distinction from RAG-only and LLM-only architectures on page 5 is the right scope decision.

### What standards exist, or are emerging, to support identity and access management of agents?

Your Section 2 covers the major standards well. Several emerging efforts are also worth engaging with:

- **IETF WIMSE Working Group** — draft-ietf-wimse-arch-06 explicitly addresses AI agents as delegated workloads and requires each hop in a delegation chain to explicitly scope and re-bind the security context.
- **IETF OAuth/WIMSE Working Group** — AI Agent Authentication and Authorization (draft-klrc-aiagent-auth-00), published March 2026, proposes a framework for AI agent authentication and authorization using OAuth, WIMSE, and SPIFFE, and explicitly maps gaps in current standards.
- **OAuth 2.0 Token Exchange (RFC 8693)** — Enables token exchange with downscoped permissions for delegation chains.
- **NIST IR 8596 (Cyber AI Profile)** — The December 2025 draft calls for issuing AI systems unique identities under the Protect function.
- **OWASP Top 10 for Agentic Applications (2026)** — Identifies ASI03 (Identity and Privilege Abuse) and ASI07 (Insecure Inter-Agent Communication) as critical risks.

---

## 2. Identification

### How might agents be identified in an enterprise architecture?

The concept paper's identification of SPIFFE/SPIRE as a candidate technology (page 7) is well-placed. An important distinction worth noting: what I would call the Ephemeral Agent Credentialing pattern adopts SPIFFE as the **identity format** — the hierarchical URI structure and SVID credential model — but does not require SPIRE as the infrastructure implementation. SPIRE is one viable runtime alongside cloud-native IAM services, custom PKI, or MCP CIMD, and organizations should select the issuance infrastructure that fits their environment. The identity format is what matters for interoperability.

SPIFFE-compatible identifiers offer a proven structure:

```
spiffe://trust-domain/agent/{orchestration_id}/{task_id}/{instance_id}
```

For example: `spiffe://company.com/agent/orch-456/task-789/uuid-abc123`

This provides global uniqueness per agent instance with embedded task context and orchestration lineage, and — where the issuance infrastructure supports it — cryptographic binding to the agent runtime environment, preventing identity reuse or transfer.

For agents in the MCP ecosystem, CIMD provides a complementary identity mechanism based on domain ownership, better suited to cross-organizational scenarios where SPIFFE trust domains do not extend.

### What metadata is essential for an AI agent's identity?

At minimum: a unique instance identifier; the task or orchestration context (what the agent is doing and who initiated it, including the human principal who authorized the action); verifiable runtime attestation properties (a cryptographically bound claim about the execution environment, not just a platform label); the scope of authorized resources; and the delegation chain if the agent was invoked by another agent. Optional but valuable additions include the agent framework and version, the model being used, and behavioral baseline information — for example, expected tool call patterns or resource access behavior that can anchor anomaly detection.

### Should agent identity metadata be ephemeral (e.g., task dependent) or is it fixed?

This is an important question, and the answer should be emphatic: **ephemeral**. Fixed identities for agents recreate the same problems as shared service accounts — they accumulate risk over time, cannot be scoped to specific tasks, and provide no per-task accountability. Each agent instance should receive a unique identity at spawn time, bound to its specific task, retired when the task completes. The agent's code or model version can carry a separate persistent identifier for inventory purposes, but the runtime identity must be ephemeral.

This is the principle most validated in practice: when credentials die with the task, the threat model simplifies dramatically.

### Should agent identities be tied to specific hardware, software, or organizational boundaries?

Yes, through attestation. The "secret zero" bootstrap problem — how does an agent prove its identity before it has any credentials? — is solvable through platform attestation as implemented in SPIRE and similar workload identity platforms. The identity service verifies attestable properties of the agent's runtime environment: Kubernetes namespace and service account, container image hash, cloud instance identity documents, or hardware-backed attestation via TPM or AWS Nitro enclaves. The agent proves its identity through verifiable environment properties rather than pre-shared secrets. For cross-organizational scenarios where shared attestation infrastructure does not exist, CIMD provides a domain-based trust boundary that requires no pre-existing relationship between parties.

This is an area where the NCCoE demonstration could provide significant practical value — showing organizations how to implement attestation without requiring pre-shared bootstrap secrets.

---

## 3. Authentication

### What constitutes strong authentication for an AI agent?

Strong agent authentication requires multiple layers operating simultaneously, consistent with the zero-trust principles referenced in SP 800-207. At the transport layer: mutual TLS (mTLS) where both the agent and the resource server present and validate certificates bound to their identities. At the application layer: task-scoped JWT tokens with cryptographic signatures — ECDSA P-256 for NIST-aligned deployments, or Ed25519 where interoperability requirements permit — validation on every request or on a defined short-interval basis.

The combination provides defense in depth: mTLS proves "who you are" at the transport level while token validation proves "what you're authorized to do" at the application level.

For multi-agent scenarios, bidirectional credential validation — where both communicating agents present and validate each other's credentials — prevents impersonation and is essential for maintaining trust in the multi-agent workflows that enterprise deployments are already producing.

### How do we handle key management for agents? Issuance, update, and revocation?

Key management for agents requires a shift from traditional lifecycle models, consistent with the concept paper's recognition that existing standards need adaptation.

**Issuance:** Credentials should be issued at runtime through a credential issuance service that validates the agent's identity via attestation or CIMD before issuing task-scoped tokens. Token lifetimes should match task duration plus a small grace period — 5 minutes is a practical default based on implementation experience, with an acceptable range of 1–60 minutes depending on task sensitivity.

**Update:** For short-lived tasks, credential update is unnecessary — the credential lives and dies with the task. For longer tasks, agents should request new credentials rather than refreshing existing ones, maintaining unique-credential-per-task discipline as the practical floor, with per-request issuance as the ideal.

**Revocation:** Effective revocation requires multiple granularity levels: token-level, agent-level, task-level, and delegation-chain-level — ensuring all downstream delegated credentials are invalidated when a parent credential is revoked. Push-based revocation via OCSP or webhook notifications can achieve propagation to all validators within 30 seconds; pull-based models relying on token expiry alone cannot meet this threshold.

I would strongly recommend the demonstration project test revocation under realistic conditions including partial service degradation. Revocation that only works when everything is healthy provides false confidence — and that is precisely the condition most organizations will never test for.

---

## 4. Authorization

### How can zero-trust principles be applied to agent authorization?

The concept paper correctly identifies zero-trust as applicable to agent authorization, referencing SP 800-207. In practice this means every request is independently authenticated and authorized: validate the mTLS certificate and JWT signature (authentication), then verify token expiration, scope match against the requested resource, revocation status, delegation chain integrity, and binding to the human principal who authorized the task (authorization) — all on every single request. A validation latency target of under 50ms is achievable because JWT validation is a local cryptographic operation and revocation checks can use locally cached lists, provided cache freshness policy is treated as a security design decision rather than a performance convenience.

### Can authorization policies be dynamically updated when an agent context changes?

Yes, and ephemeral credentialing makes this natural. Because credentials are short-lived and task-scoped, the system is continuously re-evaluating authorization. When an agent needs access to an additional resource, it requests a new credential, and the policy engine evaluates against current policy. Policy changes take effect within the TTL of outstanding credentials — provided token lifetimes are kept short, as recommended; organizations still relying on long-lived tokens will experience meaningful policy lag.

The sub-question about sensitivity of aggregated data deserves explicit attention in the demonstration — it is a genuinely hard problem. Credential scoping alone does not fully solve it. The underlying challenge is data aggregation risk — recognized in NIST privacy frameworks — where an agent accessing individually low-sensitivity records may collectively constitute high-sensitivity access. The demonstration could explore how scope taxonomies encode sensitivity classifications, and how policy engines such as NGAC — already referenced in Section 2 — evaluate aggregate access patterns rather than individual resource requests.

### How do we establish "least privilege" for an agent, especially when its required actions might not be fully predictable when deployed?

Task-scoped credentials are more effective than role-based access for agents. A scope taxonomy using the format `action:resource:identifier` enables granular enforcement — for example `read:customers:12345` or `write:orders:67890` — where the action component directly encodes the least-privilege boundary. The challenge the question identifies — that agent actions are not fully predictable — is real and worth highlighting in the demonstration.

A practical approach: define scope boundaries at the task-type level (a "customer lookup" task can read customer records but not write them), then allow runtime scope requests within those boundaries. The policy engine is the constraint, not the agent's own judgment about what it needs.

### What are the mechanisms for an agent to prove its authority to perform a specific action?

The agent presents its task-scoped token, and the resource server validates: (1) cryptographic signature — proving issuance by a trusted credential service; (2) scope includes the requested action; (3) token has not expired; (4) token has not been revoked; (5) if a delegation chain is present, each link is valid and permissions were properly attenuated; and (6) the delegation chain traces back to a known, authorized initiating principal — whether human, system, or upstream orchestrator — that was explicitly authorized to initiate the task. This provides independently verifiable proof of authority without requiring the resource server to trust the agent's own claims about itself.

### How might an agent convey the intent of its actions?

Embedding purpose metadata in the credential itself — `task_id`, `orchestration_id` — links the credential to its original purpose. Audit logs then capture both the action taken and the task context, enabling reviewers to evaluate whether what the agent did was consistent with what it was authorized to do. For delegation chains, context grounding validates each downstream agent's actions against the original task intent. Full intent verification, however, requires complementary controls — including intent classification (comparing declared task purpose against observed tool calls and resource access patterns) and behavioral monitoring — which would be valuable to include in the demonstration scope.

### How do we handle delegation of authority for "on behalf of" scenarios?

This is the question I consider most important and most underserved by current standards. The concept paper raises it under Access Delegation (page 6), and it deserves first-class treatment in the demonstration project.

The approach that has proven effective: when Agent A delegates to Agent B, the delegation token includes the delegator's identity, the original initiating principal, the complete delegation chain with cryptographic signatures at each hop, and the attenuated scope. The critical rule is **scope attenuation** — permissions can only be narrowed at each delegation hop, never expanded. Resource servers verify the complete chain: each link's signature, scope narrowing at each hop, revocation status of every agent in the chain, and that the chain traces to a legitimate initiating principal. A maximum delegation depth of 5 hops is a practical implementation guideline that balances flexibility with security.

OAuth 2.0 Token Exchange (RFC 8693) and the IETF OAuth Working Group's Transaction Tokens draft (draft-ietf-oauth-transaction-tokens) are moving toward standardizing this capability, with WIMSE providing complementary workload identity infrastructure. The NCCoE demonstration could pilot these emerging standards and provide invaluable implementation feedback directly to the standards bodies developing them.

### How do we bind agent identity with human identity to support "human-in-the-loop" authorizations?

The delegation chain provides this binding naturally. Where a human initiated the workflow, an `original_principal` field links back to that user — for example, `user:alice@company.com` — and the audit log captures the full chain, so every agent action traces back to the authorizing human. Where no human principal exists, the chain traces to the authorized system or orchestrator that initiated the task, maintaining the same verifiable accountability structure.

For human-in-the-loop scenarios specifically, the credential issuance service can require explicit human authorization before issuing credentials for sensitive scopes — inserting that approval as a cryptographically verified step in the delegation chain rather than as a side-channel approval that the chain never records. This is what separates genuine human-in-the-loop from accountability theater: the human authorization is *in the chain*, not just in a log entry that claims it happened. This approach aligns directly with the concept paper's goal of maintaining human oversight.

---

## 5. Auditing and Non-Repudiation

### How can we ensure that agents log their actions and intent in a tamper-proof and verifiable manner?

The concept paper's Logging and Transparency area of interest (page 6) is essential. An effective implementation captures timestamp, agent identity (full SPIFFE ID), task and orchestration context, action taken, resource accessed, authorization decision and scope used, outcome (success/failure), delegation depth, and delegation chain hash — all in append-only, tamper-proof storage.

For multi-agent workflows, a `delegation_chain_hash` field cryptographically links each agent's log entries to the delegation chain, enabling full reconstruction of the authorization path during forensics. An `original_principal` field ensures audit trails always trace back to the initiating principal — human or system — who authorized the workflow.

A concrete recommendation for the demonstration: test whether audit logs can reconstruct the full story of a multi-agent workflow after the fact — which agent accessed what, under whose authority, and whether the delegation chain was valid at the time of each action. If that reconstruction fails, the audit system needs work regardless of how complete each individual log entry appears.

### How do we ensure non-repudiation for agent actions and binding back to human authorization?

Non-repudiation comes from three reinforcing properties: (1) unique agent identities ensure every action ties to a specific agent instance, not a generic service account; (2) cryptographic signatures on tokens and delegation chains prevent authorization from being forged after the fact; and (3) immutable audit logs prevent the historical record from being altered. The binding to human authorization — where a human initiated the workflow — flows through the delegation chain's `original_principal` field, which traces every agent action back to the authorizing principal.

---

## 6. Prompt Injection Prevention and Mitigation

### What controls help prevent both direct and indirect prompt injections?

Prompt injection prevention requires LLM guardrails, input validation, and output filtering — these operate at a different layer than identity and authorization. The demonstration project should be clear about this boundary: credential management does not prevent prompt injection. However, identity architecture becomes directly relevant to what happens after an injection succeeds.

### After prompt injection occurs, what controls/practices can minimize the impact of the injection?

Ephemeral credentialing provides critical blast radius containment after a successful injection. The LangGrinch vulnerability (CVE-2025-68664) demonstrates this concretely — when a prompt injection manipulates an agent's behavior, the damage is constrained by: (1) task-scoped credentials limiting access to only current task resources; (2) short TTLs making exfiltrated credentials useless within minutes; (3) anomaly-based revocation enabling immediate termination; and (4) no static secrets in the agent's environment to exfiltrate — though in practice, eliminating static secrets entirely remains an adoption challenge for many organizations.

This would be a high-impact demonstration scenario: a controlled prompt injection against agents with ephemeral versus traditional credentials, measuring the difference in blast radius. It would produce the kind of concrete, quantifiable result that helps practitioners make the case for investment.
