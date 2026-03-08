# Orch Sprint Board (90-Day Execution Plan)

This document converts the roadmap into an implementation-ready sprint backlog.

## Plan Summary

- Duration: 90 days (6 phases)
- Priority: safety and determinism > quality > integrations > UX polish
- Target outcome: increase task completion rate from 40% to 65%+

## Phase 1 - Safety and Deterministic Foundations (Weeks 1-2)

### P1.1 Permission mode and destructive-action gate
- Owner: Core CLI
- Estimate: 4 days
- Dependencies: None
- Scope:
  - read-only behavior for `plan` mode
  - policy checks for file writes, patch apply, and shell execution
  - explicit approval requirement for destructive actions
- Acceptance Criteria:
  - write/apply actions are blocked in read-only mode
  - error messages are short and actionable
  - policy decisions are recorded in execution logs

### P1.2 Retry policy completion (test/validation/review)
- Owner: Orchestrator
- Estimate: 4 days
- Dependencies: P1.1 (partial)
- Scope:
  - max 2 auto-fix retries for validation and test failures
  - max 2 retries for reviewer `revise`
  - best-patch summary after retry exhaustion
- Acceptance Criteria:
  - no infinite loops
  - retry counters are visible in run state
  - terminal output includes unresolved failure summary

### P1.3 LLM resiliency
- Owner: Agents Infra
- Estimate: 2 days
- Dependencies: P1.2
- Scope:
  - exponential backoff for timeout/rate-limit/model-access errors (max 3)
  - error categorization in logs
- Acceptance Criteria:
  - transient failures do not fail immediately
  - deterministic fail behavior after final attempt

### P1.4 Repository lock (`.orch/lock`)
- Owner: Runtime
- Estimate: 2 days
- Dependencies: None
- Scope:
  - lock acquire at run start
  - lock release on success/failure/panic
  - stale lock detection and cleanup flow
- Acceptance Criteria:
  - parallel `orch run` calls are blocked per repo
  - stale locks can be recovered safely

## Phase 2 - Tooling Engine Hardening (Weeks 3-4)

### P2.1 Tool contract standardization
- Owner: Tools
- Estimate: 3 days
- Dependencies: Phase 1
- Scope:
  - unified request/response schema for `read/glob/grep/edit/write/bash/apply_patch`
  - consistent error codes and message shape
- Acceptance Criteria:
  - all tools return a shared contract format
  - logs show uniform call structure

### P2.2 Safe shell policy
- Owner: Tools + Security
- Estimate: 3 days
- Dependencies: P2.1
- Scope:
  - timeout, output truncation, and command classification
  - policy barriers for high-risk commands
- Acceptance Criteria:
  - oversized command output is redirected to file automatically
  - timeout does not deadlock runs

### P2.3 Patch pipeline hardening
- Owner: Patch Engine
- Estimate: 4 days
- Dependencies: P2.1
- Scope:
  - parser robustness improvements
  - conflict detection and reporting
  - best-patch fallback behavior
- Acceptance Criteria:
  - conflicts stop apply and list impacted files
  - invalid diffs fail with explicit reason

## Phase 3 - Session, Project, and Worktree (Weeks 5-7)

### P3.1 Project/session data model
- Owner: Storage + Orchestrator
- Estimate: 5 days
- Dependencies: Phase 2
- Scope:
  - project/session/run entity model
  - evolution from run-only to session-aware persistence
- Acceptance Criteria:
  - multiple sessions can exist under one project
  - session history is queryable

### P3.2 Session lifecycle commands
- Owner: CLI
- Estimate: 4 days
- Dependencies: P3.1
- Scope:
  - session list/create/select/close commands
  - `run` flow becomes session-aware
- Acceptance Criteria:
  - CLI runs in current active session context
  - session switching does not break existing runs

### P3.3 Worktree isolation
- Owner: Git/Runtime
- Estimate: 4 days
- Dependencies: P3.1
- Scope:
  - session-level worktree option
  - isolated patch/test execution flow
- Acceptance Criteria:
  - concurrent sessions do not conflict on same repo

## Phase 4 - GitHub and PR Operations (Weeks 8-9)

### P4.1 GitHub command set (minimum)
- Owner: Integrations
- Estimate: 4 days
- Dependencies: Phase 2
- Scope:
  - create PR, read PR comments, and link issues
- Acceptance Criteria:
  - branch + PR flow can complete in one command
  - PR URL is returned in CLI output

### P4.2 Automated PR summary
- Owner: Orchestrator + Reviewer
- Estimate: 2 days
- Dependencies: P4.1
- Scope:
  - generate PR body from plan + changes + test/review outputs
- Acceptance Criteria:
  - PR body is concise, accurate, and traceable

## Phase 5 - MCP and LSP Integrations (Weeks 10-11)

### P5.1 MCP client layer
- Owner: Integrations
- Estimate: 4 days
- Dependencies: Phase 2
- Scope:
  - MCP server config, connection handling, timeout/error policy
- Acceptance Criteria:
  - stable calls with at least 2 MCP server profiles

### P5.2 LSP-powered context quality
- Owner: Repo/Context
- Estimate: 4 days
- Dependencies: P5.1 (optional)
- Scope:
  - symbol/reference/definition-driven file targeting
  - improved planner context precision
- Acceptance Criteria:
  - measurable increase in target-file selection quality on large repos

## Phase 6 - Metrics, Stabilization, Release (Week 12)

### P6.1 Stats and KPI collection
- Owner: Runtime + Docs
- Estimate: 3 days
- Dependencies: All phases
- Scope:
  - completion rate, review acceptance, retry rate, failure taxonomy
  - baseline `orch stats` output
- Acceptance Criteria:
  - metrics are visible via a single command
  - MVP targets are reportable

### P6.2 Export/import and release hardening
- Owner: CLI + Storage
- Estimate: 2 days
- Dependencies: P6.1
- Scope:
  - session/run export-import
  - release candidate checklist and regression smoke suite
- Acceptance Criteria:
  - exported data can be imported and resumed
  - release smoke checks pass before cut

## Sprint 1 (Start Immediately)

- [ ] Permission mode + destructive-action gate
- [ ] Retry policy: validation/test/review max retry
- [ ] `.orch/lock` + stale lock handling
- [ ] Patch conflict detection + best-patch summary

### Sprint 1 Task Breakdown

#### S1-T1 Permission middleware
- Owner: Core CLI
- Estimate: 1.5 days
- Dependency: None
- Done Criteria: write/apply/bash blocked in `plan` mode

#### S1-T2 Orchestrator retry state
- Owner: Orchestrator
- Estimate: 1.5 days
- Dependency: S1-T1
- Done Criteria: max retry enforcement + terminal fail state

#### S1-T3 Lock manager
- Owner: Runtime
- Estimate: 1 day
- Dependency: None
- Done Criteria: lock acquire/release + stale lock cleanup

#### S1-T4 Patch conflict reporting
- Owner: Patch Engine
- Estimate: 1 day
- Dependency: None
- Done Criteria: conflict file list + clear error output

#### S1-T5 Tests and docs update
- Owner: QA + Docs
- Estimate: 1 day
- Dependency: S1-T1..S1-T4
- Done Criteria: integration tests + updated command docs

## Operating Rules

- Every task must include owner, estimate, dependency, and acceptance criteria.
- Each PR should include at most 2 tasks from the same phase.
- Any runtime behavior change must add at least one integration test.
- Safety-related changes should be guarded with feature flags.

## KPI Tracking

- Task completion rate
- Patch apply success rate
- Review acceptance rate
- Average run duration
- Post-retry failure rate

## Risks and Mitigations

- Scope growth -> no off-phase work without phase-gate approval.
- Tool complexity -> define contract first, then implement.
- Integration fragility -> feature flags and canary rollout.
- Performance regression -> benchmark gate at sprint end.
