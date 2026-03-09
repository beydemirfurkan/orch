# Orch Planning Engine Spec

## Objective

Define the Orch-owned planning engine that produces deterministic, structured plans before code generation.

Core rule:

> The planner model may refine the plan, but Orch owns the final plan structure and planning policy.

---

## 1. Planner Role Redefinition

Current anti-pattern:
- user prompt goes to model
- model invents plan shape
- quality depends on model behavior

Target model:
- Orch normalizes the task
- Orch compiles a draft plan using heuristics and policy
- model optionally refines details
- Orch validates and finalizes the plan

This makes planning deterministic and model-agnostic.

---

## 2. Inputs to Planning Engine

The planning engine should consume:
- user task text
- repo map
- language/framework/package manager signals
- current session/worktree context
- existing safety/policy config
- optional prior run history for same session

---

## 3. Planning Outputs

The engine must output a structured plan.

### Required fields
- summary
- task type
- risk level
- files to inspect
- files to modify
- ordered steps
- acceptance criteria
- test requirements

### Recommended fields
- invariants
- forbidden changes
- assumptions
- rollback notes
- planning reasons / evidence

---

## 4. Task Normalization

The first step is converting free-form user input into a normalized brief.

### Task types
- feature
- bugfix
- test
- refactor
- docs
- chore

### Risk levels
- low
- medium
- high

### Output example
```json
{
  "task_type": "bugfix",
  "normalized_goal": "remove race condition in auth service without API regression",
  "risk_level": "high",
  "constraints": [],
  "assumptions": []
}
```

---

## 5. File Targeting Heuristics

Planning should not rely only on model intuition.

Heuristics should consider:
- repo language and framework
- filename keyword match with task terms
- package/module names
- test file adjacency
- config relevance
- import/reference relationships when available
- session worktree context

### Ranking output
File candidates should ideally include:
- path
- reason
- confidence
- intended action (`inspect|modify|test|config`)

---

## 6. Acceptance Criteria Generation

Every plan must contain measurable acceptance criteria.

### Examples

For bugfix:
- original failure condition no longer occurs
- existing public behavior remains unchanged
- relevant regression tests pass

For feature:
- target behavior exists and is reachable
- integration points are updated
- tests cover the new behavior

For refactor:
- behavior unchanged
- readability/structure improved
- tests still pass

---

## 7. Invariant Generation

Invariants define what must not break.

Examples:
- exported APIs remain unchanged
- config semantics remain unchanged
- persistence schema unchanged unless explicitly requested
- auth/permission checks remain intact

Invariants should be derived from:
- task type
- repo domain
- path sensitivity
- risk level

---

## 8. Forbidden Change Generation

Forbidden changes narrow the allowed solution space.

Examples:
- no dependency upgrades
- no config changes
- no broad formatting-only edits
- no unrelated test rewrites
- no public API renames

---

## 9. Planner Refinement Workflow

Suggested planning flow:
1. normalize task
2. analyze repo
3. rank candidate files
4. compile deterministic draft plan
5. optionally ask model to refine risks/steps/tests
6. validate plan structure
7. finalize plan

Model refinement should be optional, not foundational.

---

## 10. Plan Validation Rules

A plan is invalid if:
- it has no acceptance criteria
- it has no step list
- modify scope is empty without explanation for a code task
- test requirements are missing
- it contains contradictory instructions

---

## 11. CLI Requirements

### `orch plan`
Human-readable summary:
- summary
- inspect files
- modify files
- steps
- risks
- tests

### `orch plan --json`
Machine-readable output for scripts and future UIs.

### Future option
`orch plan --explain`
- why these files?
- why this risk level?
- why these tests?

---

## 12. Acceptance Criteria for This Spec

This spec is implemented when:
- Orch can generate a structured plan without relying on a provider
- model refinement is optional and bounded
- every plan includes acceptance criteria and test requirements
- file targeting is deterministic enough to benchmark
- `orch plan --json` returns a stable machine-readable plan
