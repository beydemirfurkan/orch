# Orch Explain Command Spec

## Objective

Provide a command that explains why a run passed, failed, or was downgraded using Orch's structured artifacts.

Command:

```bash
orch explain [run-id]
```

If `run-id` is omitted, Orch should explain the latest run.

---

## What It Shows

The command should summarize:
- task brief
- structured plan summary
- execution contract highlights
- validation and review/test gates
- test failure classifications
- review scorecard
- confidence reasons and warnings
- final review decision
- latest retry directive if present

---

## Why It Matters

Orch is moving toward a deterministic control plane.
That means users need an equally deterministic explanation surface.

`orch explain` turns persisted artifacts into a human-readable explanation layer.

---

## Current v1 Behavior

- reads `.orch/runs/<run-id>.state`
- falls back to latest run when no id is passed
- prints structured reasoning sections in terminal

---

## Future Extensions

Later versions can include:
- explain why specific files were selected
- explain confidence formula contributions numerically
- explain retry loop history in more detail
- explain model/provider metadata
