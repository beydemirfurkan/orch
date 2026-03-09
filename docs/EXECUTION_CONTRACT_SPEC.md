# Orch Execution Contract Spec

## Objective

Define how Orch constrains the coding model during code generation.

Core principle:

> The model does not decide the workflow boundary. Orch provides the workflow boundary as an execution contract.

---

## 1. Why This Exists

Without an execution contract, coding models tend to:
- inspect too many files
- modify unrelated files
- refactor opportunistically
- skip tests or testability concerns
- overfit to prompt style instead of repo rules

The execution contract exists to bound behavior and reduce variance.

---

## 2. Contract Responsibilities

An execution contract must define:
- what files may be changed
- what files should be inspected
- what changes are required
- what changes are prohibited
- what acceptance criteria must be satisfied
- what invariants must remain true
- what patch budget is allowed

---

## 3. Contract Schema

```json
{
  "task_id": "task-123",
  "plan_id": "plan-123",
  "allowed_files": ["internal/auth/service.go"],
  "inspect_files": ["internal/auth/service.go", "internal/auth/service_test.go"],
  "required_edits": [
    "eliminate race condition in auth state mutation"
  ],
  "prohibited_actions": [
    "do not rename exported APIs",
    "do not change unrelated formatting",
    "do not touch config files"
  ],
  "acceptance_criteria": [
    "concurrent access no longer causes inconsistent auth state",
    "existing auth behavior remains unchanged"
  ],
  "invariants": [
    "public auth API signatures remain unchanged"
  ],
  "patch_budget": {
    "max_files": 3,
    "max_changed_lines": 120
  },
  "scope_expansion_policy": {
    "allowed": true,
    "requires_reason": true,
    "max_extra_files": 1
  }
}
```

---

## 4. Input Sources

The execution contract is built from:
1. normalized task brief
2. structured plan
3. selected repo context
4. policy config
5. repo/language profile

The contract is produced by Orch, not by the model.

---

## 5. Contract Rules

### Allowed Files
- files the model may modify
- if patch touches files outside this set, scope gate fails

### Inspect Files
- files the model may read for context
- may be broader than allowed files

### Required Edits
- concrete required outcomes
- each required edit must map to plan steps or acceptance criteria

### Prohibited Actions
Examples:
- no unrelated refactor
- no rename without explicit plan support
- no dependency changes unless task requires it
- no config changes unless task requires it

### Acceptance Criteria
- must be measurable
- review and completion policy depend on these

### Invariants
- system truths that must not be broken
- can be generic or task-specific

### Patch Budget
- max files
- max changed lines
- optional per-file limits in future

### Scope Expansion Policy
- if additional files are necessary, Orch may allow controlled expansion
- reason must be logged
- expansion must be visible in run manifest

---

## 6. Coder Request Contract

The coder agent should receive:
- task brief
- structured plan summary
- execution contract
- selected file contents/context
- output format requirements

The model must be instructed to return:
1. unified diff patch
2. changed file summary
3. acceptance criteria mapping
4. assumptions / unresolved concerns

---

## 7. Coder Output Schema

```json
{
  "raw_diff": "diff --git ...",
  "changed_files": [
    {
      "path": "internal/auth/service.go",
      "reason": "protect state mutation with synchronization"
    }
  ],
  "criterion_mapping": [
    {
      "criterion": "concurrent access no longer causes inconsistent auth state",
      "evidence": "guarded state mutation path"
    }
  ],
  "assumptions": [],
  "warnings": []
}
```

---

## 8. Enforcement Points

Execution contract should be enforced at multiple points.

### Before coding
- ensure contract is valid and complete

### During coding request build
- ensure only selected context is sent
- include patch budget and prohibited actions

### After coding
- parse patch
- reject out-of-scope files
- reject patch budget violations
- reject prohibited changes

### Before completion
- verify acceptance criteria mapping
- verify invariants not knowingly violated

---

## 9. Interaction with Retry Loops

When validation/test/review fails, retry should not regenerate a brand-new free-form task.

Retry contract should include:
- previous contract
- failed gates
- failed tests
- required fixes
- unchanged invariants
- previous mistakes to avoid

This keeps retries bounded and deterministic.

---

## 10. Shadow Rollout Strategy

### Stage 1
- generate contract
- log it only

### Stage 2
- warn on scope violations

### Stage 3
- hard fail on out-of-scope patch or prohibited action

Recommended first hard-enforcement rules:
- out-of-scope file changes
- patch budget exceeded
- sensitive file modifications

---

## 11. Acceptance Criteria for This Spec

This spec is implemented when:
- every coding run has a persisted execution contract
- coder input is contract-driven
- out-of-scope patch changes are blocked automatically
- retries use structured failure-aware contract updates
- review can trace patch changes back to contract criteria
