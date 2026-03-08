# Orch

[![Go Version](https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Orch is a local-first, AI-powered coding orchestrator for repositories.
It turns high-level tasks into a deterministic execution pipeline:

`Task -> Plan -> Code -> Validate -> Test -> Review -> Patch`

The project is designed for safety, auditability, and repeatable outcomes in CLI workflows.

## Core Capabilities

- Multi-step orchestration with explicit run state transitions
- Patch-first workflow with controlled apply behavior
- Built-in safety controls for destructive operations
- Session-aware execution with SQLite-backed persistence
- Repository-level locking to prevent concurrent run conflicts
- Structured tool contract with standardized error codes

## Architecture

The runtime is built around a clear orchestration pipeline and modular subsystems:

- `cmd/`: CLI command surface
- `internal/orchestrator/`: pipeline state machine and retry logic
- `internal/agents/`: planner, coder, reviewer contracts
- `internal/patch/`: parse, validate, preview, and apply pipeline
- `internal/tools/`: guarded tool execution and shell policies
- `internal/storage/`: SQLite project/session/run persistence

Primary design goals:

- Deterministic behavior over implicit automation
- Fail-safe defaults over convenience shortcuts
- Full execution traceability

## How Orch Differs from OpenCode

Orch and OpenCode solve related but different problems:

- Orch is workflow-first: it enforces a deterministic pipeline (`plan -> code -> validate -> test -> review`) and explicit run state transitions.
- Orch is execution-auditable: each run is persisted with session/project metadata, logs, retries, and patch summaries.
- Orch is safety-gated by default: read-only plan behavior, destructive action approval, repository lock, bounded retries, and guarded tool contracts.
- Orch is session-native: multiple sessions per project, active-session context, optional session worktree paths, and session-scoped run history.
- OpenCode is interaction-first: it is optimized for flexible agent-driven coding conversations, while Orch is optimized for repeatable pipeline execution.

## Installation

Requirements:

- Go `1.25+`

Build:

```bash
go build -o orch .
```

Run without building:

```bash
go run . <command>
```

## Quick Start

Start interactive mode:

```bash
./orch
```

In interactive mode, plain text runs a full task pipeline. Use `/help` to view commands.

Initialize Orch in a repository:

```bash
./orch init
```

Enable OpenAI/Codex provider execution:

```bash
export OPENAI_API_KEY="your_api_key"
```

Validate runtime/provider setup:

```bash
./orch doctor
```

Inspect provider and model mapping:

```bash
./orch provider
./orch model
./orch model set coder gpt-5.3-codex
```

Inspect current session context:

```bash
./orch session current
```

Run a task:

```bash
./orch run "add redis caching to user service"
```

Inspect and apply generated patch:

```bash
./orch diff
./orch apply
./orch apply --force --approve-destructive
```

## Session Management

Orch persists execution context in SQLite at `.orch/orch.db`.

Commands:

```bash
./orch session list
./orch session create feature-auth
./orch session create feature-auth --worktree-path ../orch-feature-auth
./orch session select feature-auth
./orch session current
./orch session runs feature-auth --status completed --contains auth --limit 20
./orch session close feature-auth
```

Notes:

- Every run is tagged with `project_id` and `session_id`
- Active session context is resolved automatically at runtime
- Optional `worktree-path` allows session-specific execution roots

## Safety Model

- `plan` mode is treated as read-only for destructive tool actions
- Destructive patch apply requires explicit approval
- Repository lock (`.orch/lock`) blocks parallel runs per execution root
- Retry limits are bounded and visible in run state
- Large shell output is truncated and redirected to `.orch/runs/*-output-*.log`

Example `safety` configuration in `.orch/config.json`:

```json
{
  "safety": {
    "dryRun": true,
    "requireDestructiveApproval": true,
    "lockStaleAfterSeconds": 3600,
    "retry": {
      "validationMax": 2,
      "testMax": 2,
      "reviewMax": 2
    },
    "featureFlags": {
      "permissionMode": true,
      "repoLock": true,
      "retryLimits": true,
      "patchConflictReporting": true
    }
  }
}
```

## Development

Run tests:

```bash
go test ./...
```

## Roadmap and Specs

- Product requirements: `docs/PRD.md`
- Sprint execution plan: `docs/SPRINT_BOARD_90_DAY_PLAN.md`
- Phase 3 SQLite/session spec: `docs/PHASE3_SQLITE_SESSION_SPEC.md`

## Contributing

Please read `CONTRIBUTING.md` before opening a pull request.

## License

This project is licensed under the MIT License. See `LICENSE`.
