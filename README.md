# Orch

[![Go Version](https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**Orch is a local-first control plane for deterministic AI coding.**

Instead of treating the model as the owner of the development workflow,
Orch treats the model as a **bounded execution engine** inside a runtime that Orch controls.

In one line:

> **Orch standardizes the coding workflow around the model, instead of trusting the model to invent the workflow.**

That means:

- **Orch owns planning**
- **Orch owns validation**
- **Orch owns testing**
- **Orch owns review**
- **Orch owns completion policy**
- **the LLM remains swappable**

Core pipeline:

`Task -> Task Brief -> Plan -> Execution Contract -> Code -> Validate -> Test -> Review -> Confidence -> Patch`

The goal is not “let the model do whatever it wants”.
The goal is to make AI coding **structured, auditable, fail-closed, and less model-fragile**.

### GitHub one-screen pitch

- **Structured planning before coding**
- **Explicit quality gates before completion**
- **Explainable runs with persisted artifacts**

### Why Orch?

Most AI coding tools still behave like this:
- the model decides the plan
- the model decides scope
- the model decides what to test
- the model decides whether the result is “good enough”

That creates inconsistent quality.
Different models, prompts, or context windows can produce very different behavior for the same task.

Orch exists to reduce that variance.

It pushes the workflow into a runtime with:
- structured artifacts
- named validation gates
- bounded retries
- review rubric enforcement
- confidence-based completion decisions
- persistent run history for audit and explanation

So the product promise is not “magic autonomous coding”.
It is **more disciplined AI coding with clearer quality control**.

---

## Product Positioning

### Orch = control plane

Orch is responsible for:
- normalizing the task
- compiling a structured plan
- building an execution contract
- enforcing scope boundaries
- running validation gates
- running tests
- scoring review quality
- computing confidence
- deciding whether a run can complete
- persisting artifacts for audit and explanation

### LLM = replaceable execution engine

The model is responsible for:
- producing a patch inside Orch’s constraints
- responding to retry directives
- contributing review text signals

But the model is **not** the workflow owner.

That distinction is the whole product thesis.

### How Orch differs from agentic coding tools

| Topic | Typical agentic coding tool | Orch |
|---|---|---|
| Workflow owner | Usually the model | Orch runtime |
| Planning | Often prompt-shaped, model-led | Structured, Orch-owned artifact |
| Scope control | Soft guidance | Explicit execution contract + scope guard |
| Validation | Often ad hoc | Named validation gates |
| Testing | Best effort | Required test-stage gates |
| Review | Free-form model opinion | Rubric + Orch decision layer |
| Completion | “Patch exists” is often enough | Review + confidence policy must pass |
| Explainability | Mostly chat transcript | Persisted run artifacts + `orch explain` |
| Telemetry | Limited | `orch stats` over run artifacts |
| Model swap impact | Can significantly change behavior | Process stays more stable by design |

---

## What Orch Does Today

Current implemented foundations include:

- structured task brief generation
- structured planning
- `orch plan --json`
- execution contract generation
- scope guard / allowed-file enforcement
- patch hygiene + plan compliance validation
- test-stage gates
- review rubric engine
- confidence scoring
- confidence enforcement policy
- test failure classification
- bounded retry directives
- SQLite-backed project / session / run persistence
- explainability via `orch explain`
- telemetry via `orch stats`

This means Orch already behaves more like a **quality-enforcing runtime** than a simple prompt wrapper.

### Best fit for

Orch is a better fit when you want:
- a repeatable CLI workflow instead of agent improvisation
- explicit quality gates before completion
- persistent run artifacts for audit/debugging
- a system that can keep its process discipline even if the model changes

It is a worse fit if your main goal is fully free-form, conversational, unconstrained agent behavior.

---

## How Orch Works

A run is not considered successful just because a patch exists.

A healthy run should look like this:

1. Orch normalizes the task into a structured brief
2. Orch compiles a plan with acceptance criteria and constraints
3. Orch builds an execution contract for the coder
4. The coder produces a bounded patch
5. Orch validates patch integrity and scope compliance
6. Orch runs required tests
7. Orch evaluates the result with a review scorecard
8. Orch computes confidence from objective signals
9. Orch either:
   - completes,
   - requests revision, or
   - fails closed

This is the core difference between Orch and “agent just edited some files”.

---

## Key Concepts

### Structured planning
Orch generates and persists a structured `TaskBrief` and `Plan` instead of relying only on raw model text.

### Execution contract
The coder works inside an explicit contract:
- allowed files
- inspect files
- required edits
- prohibited actions
- acceptance criteria
- invariants
- patch budget

### Validation gates
Each important quality check becomes a named gate, such as:
- `patch_parse_valid`
- `patch_hygiene`
- `scope_compliance`
- `plan_compliance`
- `required_tests_executed`
- `required_tests_passed`
- `review_scorecard_valid`
- `review_decision_threshold_met`

### Review rubric
Review is not just free-form commentary.
Orch computes a structured scorecard over:
- requirement coverage
- scope control
- regression risk
- readability
- maintainability
- test adequacy

### Confidence enforcement
Confidence is not only displayed.
It can actively affect completion behavior.

Default policy:
- `score >= 0.70` -> completion can proceed
- `0.50 <= score < 0.70` -> revise
- `score < 0.50` -> fail

---

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

---

## Quick Start

Initialize Orch in a repository:

```bash
./orch init
```

Set up provider auth.
API key mode:

```bash
export OPENAI_API_KEY="your_api_key"
./orch auth login openai --method api
```

Or account mode (OAuth):

```bash
./orch auth login openai --method account --flow auto
```

Validate runtime readiness:

```bash
./orch doctor
```

Generate a structured plan only:

```bash
./orch plan "add redis caching to user service"
./orch plan "add redis caching to user service" --json
```

Run the full pipeline:

```bash
./orch run "add redis caching to user service"
```

Explain the latest run:

```bash
./orch explain
```

Show quality stats across recent runs:

```bash
./orch stats
./orch stats --limit 100
```

Inspect and apply the latest patch:

```bash
./orch diff
./orch apply
./orch apply --force --approve-destructive
```

---

## Interactive Mode

Start interactive mode:

```bash
./orch
```

Important behavior:

- **plain text goes to chat mode**
- **`/plan ...` runs structured planning**
- **`/run ...` runs the full pipeline**

So typing something like:

```text
selam
```

does **not** start the coding pipeline.
It is treated as chat.

Useful interactive commands:

```text
/help
/plan add health endpoint
/run fix auth timeout bug
/logs
/explain
/stats
/session current
```

---

## Command Surface

### Core workflow

```bash
./orch init
./orch plan "task"
./orch run "task"
./orch diff
./orch apply
./orch logs [run-id]
./orch explain [run-id]
./orch stats --limit 50
```

### Sessions

```bash
./orch session list
./orch session create feature-auth
./orch session create feature-auth --worktree-path ../orch-feature-auth
./orch session select feature-auth
./orch session current
./orch session runs feature-auth --status completed --contains auth --limit 20
./orch session close feature-auth
```

### Provider and auth

```bash
./orch auth login openai --method api
./orch auth login openai --method account --flow auto
./orch auth list
./orch auth status
./orch auth logout openai
./orch provider
./orch provider list
./orch provider list --json
./orch provider set openai
./orch model
./orch model set coder gpt-5.3-codex
./orch doctor
```

---

## Persistence and Auditability

Orch persists runtime state under `.orch/`.

Important files:
- `.orch/config.json`
- `.orch/repo-map.json`
- `.orch/orch.db`
- `.orch/runs/<run-id>.state`
- `.orch/latest.patch`
- `.orch/latest-run-id`

Artifacts stored per run can include:
- task
- task brief
- plan
- execution contract
- patch
- validation results
- retry directive
- review result
- review scorecard
- confidence report
- test failure classifications
- logs

This persistence is what powers `orch explain` and `orch stats`.

---

## Safety and Quality Model

Orch is designed to be fail-closed by default.

Safety / quality behaviors include:
- read-only planning behavior
- destructive apply approval
- repository lock per execution root
- bounded retries
- structured validation gates
- explicit review decisioning
- confidence-based completion policy

Example `safety` config:

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
    "confidence": {
      "completeMin": 0.70,
      "failBelow": 0.50
    },
    "featureFlags": {
      "permissionMode": true,
      "repoLock": true,
      "retryLimits": true,
      "patchConflictReporting": true,
      "confidenceEnforcement": true
    }
  }
}
```

---

## Architecture

### Runtime at a glance

```text
User Task
   |
   v
Task Brief / Normalizer
   |
   v
Structured Plan
   |
   v
Execution Contract
   |
   v
LLM Worker (planner/coder/reviewer as bounded roles)
   |
   v
Validation Gates -> Test Gates -> Review Rubric -> Confidence Policy
   |
   v
Run Decision: complete / revise / fail
   |
   v
Persistence + Explainability + Telemetry
(.orch/orch.db, .orch/runs/*.state, orch explain, orch stats)
```

Main runtime areas:

- `cmd/` - CLI surface
- `internal/orchestrator/` - run state machine and pipeline enforcement
- `internal/planning/` - task normalization and structured planning helpers
- `internal/execution/` - execution contracts, scope guard, plan compliance, retry directives
- `internal/review/` - rubric-based review engine
- `internal/confidence/` - scoring and enforcement policy
- `internal/testing/` - test failure classification
- `internal/patch/` - patch parse, validate, apply
- `internal/tools/` - guarded tool execution policies
- `internal/storage/` - SQLite-backed persistence
- `internal/runstore/` - persisted run-state files for explainability/telemetry

---

## Product Direction

The long-term direction is:

> Orch should standardize the software delivery workflow around the model.
> The model should not define the workflow.

Put differently:

- Orch should own planning
- Orch should own validation
- Orch should own testing
- Orch should own review
- Orch should own completion policy
- the LLM should remain swappable

This is why the project is better described as:

**Control Plane for Deterministic AI Coding**

not simply “an AI coding agent”.

---

## Roadmap and Specs

Key docs:
- Product requirements: `docs/PRD.md`
- System roadmap: `docs/SYSTEMATIC_CODING_ROADMAP.md`
- Implementation tasks: `docs/IMPLEMENTATION_TASK_LIST.md`
- Quality system: `docs/QUALITY_SYSTEM_SPEC.md`
- Planning engine: `docs/PLANNING_ENGINE_SPEC.md`
- Execution contract: `docs/EXECUTION_CONTRACT_SPEC.md`
- Confidence policy: `docs/CONFIDENCE_ENFORCEMENT_POLICY.md`
- Explain command: `docs/EXPLAIN_COMMAND_SPEC.md`
- Stats command: `docs/STATS_COMMAND_SPEC.md`
- Progress log: `docs/IMPLEMENTATION_PROGRESS.md`
- Sprint board: `docs/SPRINT_BOARD_90_DAY_PLAN.md`

---

## Development

Run tests:

```bash
go test ./...
```

---

## Contributing

Please read `CONTRIBUTING.md` before opening a pull request.

## License

This project is licensed under the MIT License. See `LICENSE`.
