# Agent Team Prompt: Compliance Fix + Sidecar Sprawl Design

## Shared Prompt (all 5 agents get this)

You are on team compliance-fix-plan. You share one task with 4 other teammates. One of you is the devil's advocate.

**YOUR GOAL:** Collaborate to produce ONE design solution + implementation plan. Save it to `/Users/divineartis/proj/agentAuth/compliance_review/design-solution-implementation-plan.md`

Two problems to solve:
1. Fix every partial compliance item found in the existing reviews to make them fully compliant
2. Solve sidecar port sprawl (N apps = N sidecars = N ports)

**MANDATORY: HOW TO READ CODE**
The working directory is on a branch that is being thrown away. Do NOT use the Read tool on any Go source file in the repo. You MUST use `git -C /Users/divineartis/proj/agentAuth show develop:<path>` for every code file you read. Explore the develop branch thoroughly — use `git -C /Users/divineartis/proj/agentAuth ls-tree -r --name-only develop` to see the full file tree, then read whatever files are relevant to your analysis. The only code that matters is on the develop branch.

**Read these files normally (they are not code, so Read tool is fine):**
- All files in `/Users/divineartis/proj/agentAuth/compliance_review/` (existing compliance reviews with partial findings)
- `/Users/divineartis/proj/agentAuth/super_archive/plans/Security-Pattern-That-Is-Why-We-Built-AgentAuth.md` (source of truth)

**After reading the code and the compliance reviews, discuss with your teammates via SendMessage. Agree on solutions. The devil's advocate has veto power — nothing gets written until they approve.**

---

## Team

- security-architect
- system-designer
- code-planner
- integration-lead
- devils-advocate — challenge everything, find holes, question assumptions, identify what the team missed. Veto power — nothing gets written to the plan until you are satisfied.

Everyone works on everything together. Once the team agrees and the devil's advocate approves, write the final plan to the output file.
