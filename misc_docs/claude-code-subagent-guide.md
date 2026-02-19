# Claude Code: Task Agents & Sub-Agents Guide

## Terminology

- **Sub-agent**: A specialized AI worker spawned in its own isolated context window
- **Task tool**: The mechanism that spawns sub-agents
- These terms refer to the same thing from different angles

## Built-in Sub-Agent Types

| Type | Purpose | Tools Available |
|------|---------|----------------|
| `Explore` | Fast read-only codebase research | Glob, Grep, Read, etc. (no editing) |
| `Plan` | Research for planning mode | Read-only tools |
| `general-purpose` | Complex multi-step tasks | All tools |
| `Bash` | Terminal command execution | Bash only |
| `code-reviewer` | Code review | Read-only + search |
| `code-explorer` | Deep codebase analysis | Read-only + search |
| `code-architect` | Feature architecture design | Read-only + search |

## How Context Flows

### What sub-agents DON'T get
- The parent session's full conversation history
- They start fresh with only:
  - The prompt you provide
  - Environment details (working directory, project name)
  - CLAUDE.md content
  - MCP server access

### What sub-agents DO get
- Their own independent context window (doesn't share tokens with parent)
- Inherited permission settings (with possible restrictions)
- Access to project CLAUDE.md and configured MCP servers

### How context returns
- Only a **summary/result** returns to the parent session
- Detailed transcripts are stored separately
- Parent context is NOT consumed by verbose sub-agent output
- This isolation preserves your main context window

## Configuring Claude to Always Use Sub-Agents

### Method 1: CLAUDE.md Instructions (Recommended)

Add to your project's CLAUDE.md:

```markdown
## Sub-Agent Delegation Rules

Always delegate work to sub-agents. Follow these rules:

1. **Testing**: Use a Bash sub-agent for all test execution
2. **Research/exploration**: Use Explore sub-agent for codebase analysis
3. **Code review**: Use code-reviewer sub-agent for detailed analysis
4. **Multi-step implementation**: Use general-purpose sub-agents for independent tasks
5. **Task execution**: For TODO lists or task queues, spawn sub-agents for each independent item

### Context Preservation Strategy

Keep the main conversation focused. Sub-agents handle:
- High-volume output (tests, logs, reports)
- Focused specialization (review, debugging, research)
- Parallel operations (multiple sub-agents working independently)

Return only summaries to the main conversation.
```

### Method 2: Custom Sub-Agents in `.claude/agents/`

Create project-specific agents. Example:

**`.claude/agents/todo-runner.md`:**
```yaml
---
name: todo-runner
description: Execute and track TODO items. Use proactively when user mentions tasks, todos, or work items.
tools: Read, Bash, Grep, Write
model: sonnet
---

You are a task execution specialist. When given a TODO list or task queue:
1. Parse all items
2. Execute them in dependency order
3. Track completion status
4. Report which succeeded and which failed
```

**`.claude/agents/test-runner.md`:**
```yaml
---
name: test-runner
description: Run and fix tests automatically. Use proactively after code changes.
tools: Bash, Read, Edit, Write, Grep
---

You are a test automation expert. Run tests, parse output for failures,
and report summary to parent session.
```

### Method 3: Explicit Prompts Per Session

Tell Claude at the start of a session:

```
When working on this task, please follow this workflow:
1. For any testing, always use a sub-agent
2. For code reviews, always use the code-reviewer sub-agent
3. For debugging, always use a dedicated sub-agent
4. For TODO items, spawn sub-agents for each independent task
5. Return only summaries to the main conversation
```

### Method 4: Skills with Fork Context

Create skills that auto-delegate:

**`.claude/skills/run-tests.md`:**
```yaml
---
name: run-tests
description: Run all tests and report failures
context: fork
---

Execute the full test suite, parse output, report summary to parent session.
```

## Comparison of Methods

| Method | Effort | Automaticity | Best For |
|--------|--------|--------------|----------|
| CLAUDE.md instructions | Low | Medium | Team-wide alignment |
| Custom agents (`.claude/agents/`) | Medium | Medium-High | Project-specific workflows |
| Explicit prompts | Low | High (you control) | One-off sessions |
| Skills with fork | Medium | High | Repeatable tasks |

## Recommended Setup

Combine Methods 1 + 2:
- Add delegation rules to CLAUDE.md (applies every session)
- Create custom agents in `.claude/agents/` for your project's specific workflows
- The `description` field with "Use proactively" signals Claude to auto-delegate
