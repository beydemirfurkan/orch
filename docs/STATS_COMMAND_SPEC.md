# Orch Stats Command Spec

## Objective

Provide a compact quality telemetry view over recent Orch runs.

Command:

```bash
orch stats
orch stats --limit 100
```

---

## What It Summarizes

The command aggregates recent run state artifacts and reports:
- total runs analyzed
- completed / failed / in-progress counts
- review accept / revise counts
- average confidence
- confidence band distribution
- average retry count
- classified test failure code distribution
- latest run id and status

---

## Why It Matters

If Orch is a control plane, it should not only execute runs.
It should also expose system quality signals over time.

`orch stats` is the first telemetry surface for that.

---

## Current v1 Behavior

- reads `.orch/runs/*.state`
- sorts runs by `started_at` descending
- analyzes the latest N runs via `--limit`
- prints a terminal summary

---

## Future Extensions

Later versions can include:
- session-scoped stats
- JSON output
- confidence trend charts
- retry hotspot reporting
- per-model / per-provider variance analysis
