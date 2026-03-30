# Design Conversation: Acceptance Test Automation + Verification

**Status:** Backburner — design idea captured, not yet implemented
**Date:** 2026-03-30
**Origin:** B5 SEC-L2b acceptance test session

## Design Idea Birth (verbatim)

> nice you need to write this up in MEMORY.md lessons learned you learned alot ... now i Do have a quesiton can we automate this to run one after another wihoug me being involve and how can i check it to be sure you followed the correct way with a deterministic hook that loops you do the work over

## Context

During B5 acceptance testing, the coding agent (Claude) had to be corrected multiple times:
1. First attempt was a bulk script dump — not individual story files
2. Banners were too technical, not executive-readable
3. Personas were wrong ("Developer" instead of "App")
4. Stories didn't reflect real-world production behavior
5. The integration.sh script runs tests but cuts corners — it doesn't produce proper evidence files with banners per the LIVE-TEST-TEMPLATE

## The Problem

The `integration.sh` script automates the PASS/FAIL checking but does NOT:
- Create individual story evidence files
- Write executive-readable banners (Who/What/Why/How/Expected)
- Use the correct personas
- Ground stories in real-world production behavior
- Follow the LIVE-TEST-TEMPLATE format

It's a CI smoke test, not an acceptance test. The two are different things.

## Design Ideas

### Option 1: Review Hook (deterministic)
A hook that runs after evidence files are committed. It checks each `story-*.md` file for:
- Banner present (Who/What/Why/How/Expected sections)
- Test Output section with actual output
- Verdict line (PASS/FAIL/SKIP with explanation)
- Persona is valid (App, Operator, Security Reviewer — not "Developer (curl)")
- Mode is specified (VPS, Container, both)

If any check fails, the hook blocks the commit and tells the agent what to fix. The agent loops until all checks pass.

### Option 2: /verify-evidence Skill
A skill invoked after evidence is claimed done. It:
1. Reads the LIVE-TEST-TEMPLATE
2. Reads each evidence file
3. Checks compliance against the template
4. Reports violations
5. User reviews the report, not every file

### Option 3: Runner Script That Produces Template-Compliant Evidence
A new script (not integration.sh) that:
1. Reads user-stories.md for the banner content
2. Starts the broker
3. Runs each story one at a time
4. Writes individual story files with proper banners
5. Captures real output
6. Writes verdict based on actual results
7. Produces README.md summary

This would replace the manual story-by-story process while maintaining template compliance.

### Key Question
integration.sh says it does the job but it cuts corners. Option 3 would be a proper replacement that actually follows the template. Options 1 and 2 are verification layers on top of the manual process.

The real answer might be: Option 3 for automation + Option 1 or 2 for verification that it was done right.

## Next Steps

- [ ] Decide which option(s) to pursue
- [ ] Design the runner script or hook
- [ ] Test against B5 evidence as the reference implementation
- [ ] Apply retroactively to B0-B4 if desired
