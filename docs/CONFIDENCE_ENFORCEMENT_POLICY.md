# Orch Confidence Enforcement Policy

## Objective

Turn confidence from a passive display metric into an active completion policy.

Core rule:

> A run should not be marked successful only because it produced a patch.
> It should complete only when confidence is high enough for the current safety policy.

---

## 1. Why This Exists

Before enforcement, confidence is only informational:
- the system shows a score
- the system logs a score
- the user can inspect the score

But completion behavior stays mostly unchanged.

That creates a gap:
- a run may finish as `completed`
- but confidence may still be too low for safe trust

Confidence enforcement closes that gap.

---

## 2. Policy Inputs

Confidence enforcement evaluates:
- final review decision
- review scorecard
- confidence score
- configured thresholds
- confidence enforcement feature flag

It runs after:
1. review scorecard generation
2. confidence scoring

---

## 3. Policy Thresholds

Configured under:
- `safety.confidence.completeMin`
- `safety.confidence.failBelow`
- `safety.featureFlags.confidenceEnforcement`

Default policy:
- `completeMin = 0.70`
- `failBelow = 0.50`

Interpretation:
- `score >= completeMin` -> completion threshold satisfied
- `failBelow <= score < completeMin` -> too weak for completion, revise instead
- `score < failBelow` -> hard failure

---

## 4. Default v1 Behavior

### Case A — confidence meets threshold
Condition:
- `score >= 0.70`

Behavior:
- review threshold gate passes
- run may complete if other gates and review are also acceptable

### Case B — low confidence
Condition:
- `0.50 <= score < 0.70`

Behavior:
- review decision is downgraded to `revise`
- review scorecard is marked `revise`
- retry loop gets new review findings
- run does not complete yet

### Case C — very low confidence
Condition:
- `score < 0.50`

Behavior:
- confidence policy returns hard error
- run fails closed
- patch may still be persisted for inspection
- run is not considered successful

---

## 5. Required Review Gates

The review stage should produce at least these gates:
- `review_scorecard_valid`
- `review_decision_threshold_met`

### `review_scorecard_valid`
Passes when a review scorecard exists.
Fails when scorecard is missing.

### `review_decision_threshold_met`
Passes when confidence meets completion threshold.
Fails when confidence is too low for completion.

---

## 6. Interaction with Retry Loop

If confidence is low but not catastrophic:
- Orch should prefer `revise`
- retry directive should instruct the coder to improve certainty
- examples:
  - strengthen tests
  - reduce scope ambiguity
  - resolve review findings
  - improve validation signal quality

If confidence is very low:
- Orch should fail closed instead of looping forever

---

## 7. Config Example

```json
{
  "safety": {
    "confidence": {
      "completeMin": 0.70,
      "failBelow": 0.50
    },
    "featureFlags": {
      "confidenceEnforcement": true
    }
  }
}
```

---

## 8. Expected User Experience

### Healthy run
- review says `accept`
- confidence is `high` or `medium`
- run completes normally

### Weak but salvageable run
- review may initially accept
- confidence policy downgrades to `revise`
- Orch retries with structured review suggestions

### Unsafe run
- confidence is `very_low`
- Orch blocks completion
- user sees that the system does not trust the result enough

---

## 9. Acceptance Criteria

This policy is implemented when:
- confidence score is computed before final completion
- review stage emits review threshold gates
- low confidence downgrades review to `revise`
- very low confidence fails the run
- policy thresholds are configurable
- behavior is covered by automated tests
