# Orch

Orch is a CLI orchestration engine that uses AI agents to execute coding tasks inside a repository.

## Safety Controls

- `plan` mode is treated as read-only by the tool policy layer for destructive tools.
- Destructive patch apply requires explicit approval: `orch apply --force --approve-destructive`.
- Repository lock file `.orch/lock` prevents concurrent `orch run` operations per repository.
- Retry limits are configurable in `.orch/config.json` under `safety.retry`.
- Retry exhaustion stores unresolved failure summary and best-patch summary in run state.
- Shell commands use timeout and output truncation safeguards; large output is redirected to `.orch/runs/*-output-*.log`.

## Sessions

- Session-aware persistence is backed by SQLite at `.orch/orch.db`.
- Use `orch session list|create|select|close|current|runs` to manage execution contexts.
- `orch session create <name> --worktree-path <path>` sets a session-specific execution root.
- `orch session runs <name-or-id> --status <status> --contains <text> --limit <n>` filters session run history.
- `orch run` executes in the active session context and tags runs with `project_id/session_id`.

Example safety block in `.orch/config.json`:

```json
{
  "safety": {
    "dryRun": true,
    "requireDestructiveApproval": true,
    "lockStaleAfterSeconds": 3600,
    "retry": { "validationMax": 2, "testMax": 2, "reviewMax": 2 },
    "featureFlags": {
      "permissionMode": true,
      "repoLock": true,
      "retryLimits": true,
      "patchConflictReporting": true
    }
  }
}
```

## Language Guard

This repository enforces English-only source and docs content via:

- `scripts/check-language.sh`
- `.github/workflows/language-guard.yml`

Run it locally:

```bash
bash scripts/check-language.sh
```

### Allowlist Exceptions

If you must keep an intentional non-English match, add a **single-line regex** to:

- `.language-guard-allowlist`

Rules:

- Keep exceptions minimal and specific.
- Prefer path-anchored patterns over broad terms.
- Add a comment above each exception explaining why it is required.

Example:

```text
# Allow a fixed test fixture string in one file
^internal/fixtures/sample\.txt:\d+:.*sample-data.*$
```
