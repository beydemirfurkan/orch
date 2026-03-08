# Phase 3 Spec: SQLite-based Project/Session/Run Management

This spec defines the implementation for Phase 3 (`P3.1`, `P3.2`, `P3.3`) using SQLite as the primary persistence layer.

## Decision Summary

- Storage engine: SQLite
- Go driver: `modernc.org/sqlite` (no CGO requirement)
- ORM: none for MVP phase
- Drizzle note: Drizzle is a TypeScript ORM and is not used inside the Go runtime

Rationale:

- SQLite keeps the CLI local-first and deterministic
- No external service dependency
- Single-file database supports fast backups and portability
- `modernc.org/sqlite` simplifies build and CI by avoiding CGO toolchain requirements

## Goals

- Replace run-only persistence with project/session-aware persistence
- Add session lifecycle commands to CLI
- Make `orch run` execute in active session context
- Prepare schema and interfaces for session-level worktree isolation

## Non-goals (Phase 3)

- No distributed or remote database
- No multi-user session ownership model
- No web UI session manager
- No full migration of all legacy JSON logs to relational form in first pass

## Functional Requirements

### P3.1 Project/session data model

- A repository maps to exactly one project row (`repo_root` unique)
- A project can have multiple sessions
- A run belongs to one project and one session
- Session history is queryable from CLI
- Active session is persisted and can be resolved at process startup

### P3.2 Session lifecycle commands

New commands:

- `orch session list`
- `orch session create <name>`
- `orch session select <name-or-id>`
- `orch session close <name-or-id>`
- `orch session current`

Behavior:

- `orch run` requires an active session
- If no active session exists, create/select `default`
- Session switching must not break existing run flow

### P3.3 Worktree isolation foundation

- Session table includes optional `worktree_path`
- Runtime resolves execution root from session context
- If `worktree_path` is empty, execution root is repository root
- Full `git worktree` lifecycle command surface can be added in follow-up

## Data Model

Database file: `.orch/orch.db`

### Tables

`schema_migrations`

- `version INTEGER PRIMARY KEY`
- `applied_at TEXT NOT NULL`

`projects`

- `id TEXT PRIMARY KEY`
- `name TEXT NOT NULL`
- `repo_root TEXT NOT NULL UNIQUE`
- `created_at TEXT NOT NULL`
- `updated_at TEXT NOT NULL`

`sessions`

- `id TEXT PRIMARY KEY`
- `project_id TEXT NOT NULL REFERENCES projects(id)`
- `name TEXT NOT NULL`
- `status TEXT NOT NULL` (`active`, `closed`)
- `worktree_path TEXT NULL`
- `created_at TEXT NOT NULL`
- `closed_at TEXT NULL`
- `UNIQUE(project_id, name)`

`runs`

- `id TEXT PRIMARY KEY`
- `project_id TEXT NOT NULL REFERENCES projects(id)`
- `session_id TEXT NOT NULL REFERENCES sessions(id)`
- `task_json TEXT NOT NULL`
- `status TEXT NOT NULL`
- `plan_json TEXT NULL`
- `patch_json TEXT NULL`
- `review_json TEXT NULL`
- `test_results TEXT NULL`
- `retries_json TEXT NULL`
- `unresolved_failures_json TEXT NULL`
- `best_patch_summary TEXT NULL`
- `error TEXT NULL`
- `started_at TEXT NOT NULL`
- `completed_at TEXT NULL`

`run_logs`

- `id INTEGER PRIMARY KEY AUTOINCREMENT`
- `run_id TEXT NOT NULL REFERENCES runs(id)`
- `timestamp TEXT NOT NULL`
- `actor TEXT NOT NULL`
- `step TEXT NOT NULL`
- `message TEXT NOT NULL`

`meta`

- `key TEXT PRIMARY KEY`
- `value TEXT NOT NULL`

Meta keys:

- `active_session_id`
- `active_project_id`

## SQLite Runtime Configuration

Apply once per connection:

- `PRAGMA foreign_keys = ON;`
- `PRAGMA journal_mode = WAL;`
- `PRAGMA synchronous = NORMAL;`
- `PRAGMA busy_timeout = 5000;`

## Package and Interface Design

New package: `internal/storage`

Subcomponents:

- `db.go`: open/configure SQLite connection
- `migrate.go`: schema migrations
- `project_repo.go`
- `session_repo.go`
- `run_repo.go`
- `log_repo.go`
- `meta_repo.go`

Core service interface:

- `GetOrCreateProject(repoRoot string) (Project, error)`
- `CreateSession(projectID, name string) (Session, error)`
- `ListSessions(projectID string) ([]Session, error)`
- `SelectSession(projectID, nameOrID string) (Session, error)`
- `CloseSession(projectID, nameOrID string) error`
- `GetActiveSession(projectID string) (Session, error)`
- `SetActiveSession(projectID, sessionID string) error`
- `SaveRun(state *models.RunState, projectID, sessionID string) error`
- `ListRunsBySession(sessionID string, limit int) ([]RunRecord, error)`

## CLI Behavior Details

### `orch init`

- Ensure `.orch/` exists
- Initialize/open `.orch/orch.db`
- Run migrations
- Ensure project row for current repository
- Ensure default session exists
- Set active session to default if none

### `orch run "..."`

- Resolve current project by `repo_root`
- Resolve active session
- If missing, auto-create/select `default`
- Inject `ProjectID` and `SessionID` into run state
- Persist run state and logs to SQLite
- Continue writing current `.orch/runs/*.state` during transition window

### `orch session list`

- Show session id, name, status, created_at, active marker

### `orch session create <name>`

- Create new active session under current project
- Fail if duplicate name in same project

### `orch session select <name-or-id>`

- Set selected session as active
- Reject selecting closed session

### `orch session close <name-or-id>`

- Mark session `closed` and set `closed_at`
- If closed session is active, clear active and fallback to `default`

## Integration with Existing Runtime

- `models.RunState` gains:
  - `ProjectID string`
  - `SessionID string`
- `runstore` remains available for backward compatibility while storage migration is rolled out
- Orchestrator remains unchanged in core step logic; session awareness is added at command/runtime boundaries

## Context Management Requirements

Current gap:

- `ContextBuilder` exists but is not wired into planner/coder pipeline

Required change:

- In `stepPlan`, build context after plan generation
- Attach built context to coder input in `stepCode`
- Optionally persist session-level context summary for future ranking improvements

## Migration and Backward Compatibility

- Migration is idempotent and versioned
- If old `.orch/runs/*.state` files exist, they remain readable
- New runs are written to SQLite and optionally mirrored to old format until deprecation
- If DB open/migration fails, CLI should fail with actionable error (no silent fallback)

## Error Handling

- All storage errors return wrapped, actionable messages
- Session command errors are deterministic:
  - `session_not_found`
  - `session_closed`
  - `session_name_conflict`
  - `no_active_session`
- Database lock contention should honor `busy_timeout` and produce clear terminal messages

## Security and Reliability

- SQL statements use prepared queries/parameters only
- Foreign keys enforced
- No destructive schema drops in automatic migrations
- Migrations run inside transactions
- Backup guidance: copy `.orch/orch.db` when CLI is idle

## Testing Strategy

### Unit tests

- migration runner idempotency
- repository CRUD for project/session/run/log/meta
- active session selection rules
- closed session protection

### Integration tests

- `orch init` creates project/default session
- `orch session` command lifecycle
- `orch run` writes correct `project_id/session_id`
- session switching isolates run history queries

### Regression tests

- existing run pipeline behavior unchanged
- existing lock and safety features remain active

## Acceptance Criteria (Phase 3)

- Multiple sessions exist under one project
- Session history is queryable
- CLI runs in current active session context
- Session switching does not break existing runs
- Worktree path is session-ready and execution root resolves correctly

## Rollout Plan

1. Introduce SQLite storage package and migrations
2. Add project/session repositories and active session meta
3. Add `orch session` commands
4. Make `orch run` session-aware
5. Wire context builder into orchestrator pipeline
6. Add tests and docs
7. Keep JSON run mirror temporarily, then remove in later cleanup

## Open Follow-ups

- Add `orch session export/import` in Phase 6
- Add full worktree provisioning commands (`create --worktree`, `prune`) in P3.3 follow-up
- Add performance benchmarks for large run-log datasets
