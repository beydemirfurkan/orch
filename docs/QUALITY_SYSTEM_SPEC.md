# Orch Quality System Spec

## Objective

Define the quality system that Orch owns independently of the active LLM.

Core rule:

> The model may generate code, but Orch decides whether that code is acceptable.

---

## 1. Quality System Responsibilities

Orch quality system must:

1. define what a good run means
2. define mandatory gates before completion
3. classify failures into stable categories
4. produce structured findings for retry loops
5. generate a confidence score from objective signals

---

## 2. Quality Layers

### Layer Q1 — Plan Quality
Checks whether the produced plan is actionable and bounded.

Signals:
- plan has summary
- files to inspect present
- files to modify present or explicitly empty with reason
- acceptance criteria present
- invariants present when needed
- test requirements present

### Layer Q2 — Scope Quality
Checks whether generated changes stay within intended scope.

Signals:
- changed files are inside allowed files
- no unrelated edits
- no opportunistic refactor unless justified
- patch size is within budget

### Layer Q3 — Patch Hygiene
Checks patch safety and patch integrity.

Signals:
- valid unified diff
- no binary mutation
- no secret/sensitive file mutation
- no malformed hunks
- no blocked file types

### Layer Q4 — Build and Static Quality
Checks code validity before broader testing.

Signals:
- syntax/parse success
- compile/typecheck success
- static analysis pass or warning count

### Layer Q5 — Test Quality
Checks whether relevant tests passed.

Signals:
- required tests executed
- targeted tests passed
- full suite escalated when risk profile requires
- failure category available when tests fail

### Layer Q6 — Review Quality
Checks whether the final patch is acceptable against a rubric.

Rubric axes:
- requirement coverage
- scope control
- regression risk
- readability
- maintainability
- test adequacy

### Layer Q7 — Confidence Quality
Produces overall confidence from prior layers.

---

## 3. Gate Model

Each gate must return a structured result.

```json
{
  "name": "plan_compliance",
  "stage": "validation",
  "status": "pass|warn|fail",
  "severity": "low|medium|high|critical",
  "summary": "patch changed files outside allowed scope",
  "details": [],
  "actionable_items": [],
  "metadata": {}
}
```

### Required fields
- `name`
- `stage`
- `status`
- `severity`
- `summary`

### Optional fields
- `details`
- `actionable_items`
- `metadata`

---

## 4. Mandatory Gate Set for v1

### Planning stage
- `task_brief_valid`
- `structured_plan_valid`
- `acceptance_criteria_present`

### Coding/validation stage
- `patch_parse_valid`
- `patch_hygiene`
- `scope_compliance`
- `plan_compliance`

### Static stage
- `syntax_valid`
- `build_or_typecheck_valid`

### Test stage
- `required_tests_executed`
- `required_tests_passed`

### Review stage
- `review_scorecard_valid`
- `review_decision_threshold_met`

---

## 5. Failure Taxonomy

All failures should map into stable categories.

### Planning failures
- `task_ambiguous`
- `plan_incomplete`
- `plan_scope_unknown`

### Coding failures
- `empty_patch`
- `invalid_patch`
- `out_of_scope_patch`
- `unrelated_edits`
- `patch_budget_exceeded`

### Validation failures
- `sensitive_file_violation`
- `binary_file_violation`
- `plan_compliance_failure`
- `syntax_failure`
- `build_failure`
- `typecheck_failure`
- `static_analysis_failure`

### Test failures
- `test_assertion_failure`
- `test_timeout`
- `test_setup_failure`
- `flaky_test_suspected`
- `missing_required_tests`

### Review failures
- `review_revise`
- `review_reject`
- `low_confidence`

### Runtime/provider failures
- `provider_auth_error`
- `provider_timeout`
- `provider_rate_limited`
- `provider_invalid_response`
- `provider_transient_error`

---

## 6. Completion Policy

A run should be marked `completed` only if:

1. all critical gates pass
2. all required tests pass
3. review decision is `accept`
4. confidence is above the configured threshold or completion is explicitly downgraded with warning mode

Otherwise:
- `revise` loops continue within retry budget, or
- run becomes `failed`

---

## 7. Shadow / Soft / Hard Rollout

### Shadow mode
- gates run
- results logged
- no hard blocking yet

### Soft mode
- severe failures warn loudly
- selected gates block

### Hard mode
- mandatory gates fail closed
- run cannot complete when mandatory gate fails

Recommended rollout:
1. shadow for new gates
2. soft for 1 sprint
3. hard after benchmark validation

---

## 8. Confidence Scoring v1

Confidence is derived from objective signals.

### Inputs
- plan completeness
- scope compliance
- validation pass ratio
- test pass ratio
- review rubric score
- retry count
- patch size risk

### Example weighted formula
- plan completeness: 10%
- scope compliance: 20%
- validation gates: 20%
- test quality: 25%
- review rubric: 20%
- retry penalty: -5%

### Output
```json
{
  "score": 0.84,
  "band": "high",
  "reasons": [
    "all mandatory gates passed",
    "required tests passed",
    "no scope violation detected"
  ],
  "warnings": []
}
```

Suggested bands:
- `0.85 - 1.00` high
- `0.70 - 0.84` medium
- `0.50 - 0.69` low
- `< 0.50` very_low

---

## 9. Storage Impact

Run persistence should support:
- `task_brief_json`
- `structured_plan_json`
- `execution_contract_json`
- `validation_results_json`
- `review_scorecard_json`
- `confidence_json`

---

## 10. CLI / UX Requirements

CLI summaries must show:
- passed/failed gates
- failed gate reasons
- test summary
- review summary
- confidence score and band
- unresolved risks

Suggested future commands:
- `orch explain <run-id>`
- `orch stats`

---

## 11. Acceptance Criteria for This Spec

This spec is implemented when:
- validation is no longer a single opaque step
- every major quality step returns structured gates
- retry loops consume gate results directly
- completion policy uses gate state + review + confidence
- quality signals persist in storage and appear in CLI summaries
