# Orch MVP Checklist (v1.1)

This checklist turns `orch-prd-v1.1` into an actionable delivery plan.

## P0 - Core MVP

- [ ] **CLI skeleton (Go + Cobra):** `orch init`, `orch plan`, `orch run`, `orch diff`, `orch apply`, and `orch logs` are functional.
- [ ] **Run state machine:** `created -> analyzing -> planning -> coding -> validating -> testing -> reviewing -> completed|failed` states are persisted and logged.
- [ ] **Repository analyzer + index:** file tree scan, repository summary, and `.orch/repo-map.json` generation.
- [ ] **Config bootstrap/load:** `.orch/config.json` is created and model/test/lint commands are loaded.
- [ ] **Agent contracts:** clear planner/coder/reviewer input-output schemas wired into orchestrator.
- [ ] **Patch pipeline:** `generate -> parse -> validate -> preview` flow with unified diff output.
- [ ] **Patch safety rules:** no binary updates, no `.env` updates, and patch limits (`maxFiles=10`, `maxLines=800`).
- [ ] **Dry-run by default:** `orch apply` does not apply without explicit user action after preview.
- [ ] **Test runner integration:** configured test command runs and feeds pipeline state.
- [ ] **Execution trace/logging:** `.orch/runs/<timestamp>.log` write path and concise terminal error output.

## P1 - Reliability and Error Handling

- [ ] **Iterative coder loop:** draft -> self-review -> final diff (2 iterations).
- [ ] **Reviewer revise loop:** on `revise`, retry up to 2 times, then fail with best patch summary.
- [ ] **Test/validation auto-fix loop:** up to 2 retries with bounded retry context.
- [ ] **LLM API resiliency:** exponential backoff for timeout/rate-limit/access issues (max 3 attempts).
- [ ] **Conflict handling:** detect apply conflicts, stop apply, and return affected file list.
- [ ] **Repository lock:** prevent parallel runs in same repo (`.orch/lock`).
- [ ] **Deterministic tool wrapper:** normalized and logged read/search/run/apply calls.

## P2 - Developer Experience and Performance

- [ ] **Terminal UX polish:** spinner, step-level status, summary block, quiet-by-default output.
- [ ] **`orch plan` quality:** clear file targeting, risk analysis, test strategy, and change steps.
- [ ] **`orch diff` readability:** changed file count, line deltas, and patch preview clarity.
- [ ] **Config override flow:** explicit and validated command/model override behavior.
- [ ] **Performance goals:** startup `<100ms`, small task target `<30s`.

## Security and Policy Checklist

- [ ] Patches are never auto-applied.
- [ ] API keys are never written to files; environment variables only.
- [ ] Sensitive file protections are active (`.env`, `secrets`, and equivalents).
- [ ] Patch limit violations fail fast with clear error output.
- [ ] Full stack traces are written to logs, not dumped to terminal.

## MVP Definition of Done

- [ ] End-to-end flow works for a task like `orch run "add redis caching to user service"`.
- [ ] Successful runs produce a viewable patch via `orch diff` and controlled apply via `orch apply`.
- [ ] Failed runs honor retry policy and return a best-patch summary.
- [ ] Every step is traceable in logs (audit trail).
- [ ] MVP target metric (%40 task completion baseline) is measurable on at least one example repo.

## Recommended Implementation Order

1. CLI + state machine + logging foundation
2. Repo analyzer + config + planning command
3. Agent integration + patch pipeline + diff
4. Test runner + reviewer + retry policies
5. Apply safety layer + conflict/lock
6. Terminal UX + performance tuning
