# Orch Sprint Board (90-Day Execution Plan)

This sprint board aligns the next 90 days with the systematic coding vision:

> Orch should not behave like another free-form coding agent.
> Orch should become the control plane that makes coding models disciplined, testable, reviewable, and reliable.

Related docs:
- `docs/SYSTEMATIC_CODING_ROADMAP.md`
- `docs/IMPLEMENTATION_TASK_LIST.md`
- `docs/QUALITY_SYSTEM_SPEC.md`
- `docs/EXECUTION_CONTRACT_SPEC.md`
- `docs/PLANNING_ENGINE_SPEC.md`

---

## Plan Summary

- Duration: 90 days
- Goal: make Orch own planning, validation, testing, review, and confidence
- Strategy: contracts first -> planning ownership -> constrained coding -> quality gates -> review/confidence -> benchmarks
- Target outcome:
  - lower model variance
  - lower scope drift
  - higher first-pass validation/test success
  - more explainable run artifacts

---

## Phase 1 - Contracts and Planning Foundation (Weeks 1-2)

### P1.1 Contract model foundation
- Owner: Core Runtime
- Estimate: 4 days
- Dependencies: None
- Scope:
  - task brief model
  - structured plan model
  - execution contract model
  - validation result model
  - review scorecard placeholders in run state
- Acceptance Criteria:
  - typed models exist and serialize cleanly
  - run state can persist new contract data
  - contract tests cover JSON roundtrips

### P1.2 Task normalizer
- Owner: Planning
- Estimate: 3 days
- Dependencies: P1.1
- Scope:
  - task type classification
  - normalized goal generation
  - risk level derivation
  - assumptions/constraints scaffolding
- Acceptance Criteria:
  - same task input produces stable normalized brief
  - ambiguous tasks are marked explicitly

### P1.3 `orch plan --json`
- Owner: CLI
- Estimate: 2 days
- Dependencies: P1.1
- Scope:
  - JSON output mode for structured planning
  - optional explainability metadata in output
- Acceptance Criteria:
  - machine-readable plan output is stable
  - output works without provider runtime

### P1.4 Acceptance criteria generator v0
- Owner: Planning
- Estimate: 1 day
- Dependencies: P1.2
- Scope:
  - acceptance criteria templates per task type
  - test requirement seed generation
- Acceptance Criteria:
  - every generated code task has acceptance criteria
  - every generated code task has test requirements

---

## Phase 2 - Orch-Owned Planning Engine (Weeks 3-4)

### P2.1 Deterministic file targeting
- Owner: Planning + Repo
- Estimate: 4 days
- Dependencies: Phase 1
- Scope:
  - repo-aware candidate file ranking
  - inspect/modify/test/config intent tagging
- Acceptance Criteria:
  - plan includes ranked file lists with reasons
  - targeting is benchmarkable and repeatable

### P2.2 Structured plan compiler
- Owner: Planning
- Estimate: 4 days
- Dependencies: P2.1
- Scope:
  - compile final plan from normalized brief + heuristics
  - steps, risks, invariants, forbidden changes, tests
- Acceptance Criteria:
  - final plan shape is Orch-owned
  - plan remains valid without model refinement

### P2.3 Planner redesign
- Owner: Agents
- Estimate: 2 days
- Dependencies: P2.2
- Scope:
  - planner role shifts from plan owner to refinement worker
  - optional refinement pass only
- Acceptance Criteria:
  - planner cannot return an arbitrary free-form final plan
  - Orch validates and finalizes the final structured plan

---

## Phase 3 - Constrained Coding and Scope Control (Weeks 5-6)

### P3.1 Execution contract builder
- Owner: Execution Engine
- Estimate: 3 days
- Dependencies: Phase 2
- Scope:
  - allowed files
  - inspect files
  - required edits
  - prohibited actions
  - patch budget
- Acceptance Criteria:
  - every coding run gets an execution contract
  - execution contract persists in run state

### P3.2 Coder input/output redesign
- Owner: Agents
- Estimate: 4 days
- Dependencies: P3.1
- Scope:
  - coder receives task brief + structured plan + execution contract
  - coder returns diff + change summary + criterion mapping + assumptions
- Acceptance Criteria:
  - coder output is machine-checkable
  - empty patch returns a structured reason

### P3.3 Scope guard and minimal diff enforcement
- Owner: Quality + Patch Engine
- Estimate: 3 days
- Dependencies: P3.2
- Scope:
  - out-of-scope file detection
  - unrelated edit detection
  - patch budget enforcement
- Acceptance Criteria:
  - out-of-scope changes fail before tests
  - large/unrelated diffs are visible and classifiable

---

## Phase 4 - Validation Gates and Bounded Fix Loops (Weeks 7-8)

### P4.1 Gate framework
- Owner: Quality
- Estimate: 3 days
- Dependencies: Phase 3
- Scope:
  - pluggable validation gate interface
  - structured gate results
  - stage-aware gate aggregation
- Acceptance Criteria:
  - validation no longer returns a single opaque error
  - gate results are persisted and visible in logs

### P4.2 Mandatory v1 gates
- Owner: Quality
- Estimate: 4 days
- Dependencies: P4.1
- Scope:
  - patch hygiene
  - scope compliance
  - plan compliance
  - syntax/build gate adapters
- Acceptance Criteria:
  - failed gate names and reasons are explicit
  - retry logic can consume gate outputs directly

### P4.3 Bounded fix-loop contract
- Owner: Orchestrator
- Estimate: 3 days
- Dependencies: P4.2
- Scope:
  - retry prompt built from failed gates and failed tests
  - repeated-error detection
  - unresolved failure summary improvements
- Acceptance Criteria:
  - retries are deterministic and targeted
  - no free-form “try again” loop remains

---

## Phase 5 - Test Intelligence, Review, and Confidence (Weeks 9-10)

### P5.1 Test matrix and failure classifier
- Owner: Testing
- Estimate: 4 days
- Dependencies: Phase 4
- Scope:
  - test requirements model
  - targeted test selection hooks
  - failure category classification
- Acceptance Criteria:
  - required tests are explicitly tracked
  - test failures are classified into stable categories

### P5.2 Review rubric engine
- Owner: Review
- Estimate: 4 days
- Dependencies: Phase 4
- Scope:
  - scorecard dimensions
  - accept/revise/reject thresholds
  - finding summaries
- Acceptance Criteria:
  - review results are structured scorecards
  - revise decisions contain actionable findings

### P5.3 Confidence scoring v1
- Owner: Runtime + Review
- Estimate: 2 days
- Dependencies: P5.1, P5.2
- Scope:
  - confidence score from validation/test/review/retry signals
  - CLI display of score and band
- Acceptance Criteria:
  - every completed run shows confidence
  - confidence rationale is persisted

---

## Phase 6 - Benchmarks, Explainability, and Hardening (Weeks 11-12)

### P6.1 Golden benchmark task suite
- Owner: Bench
- Estimate: 4 days
- Dependencies: Phase 5
- Scope:
  - create initial benchmark tasks
  - scoring criteria and expected scope definitions
- Acceptance Criteria:
  - benchmark suite covers core task categories
  - replay results can be compared across models

### P6.2 Explainability and stats
- Owner: CLI + Runtime
- Estimate: 3 days
- Dependencies: Phase 5
- Scope:
  - `orch stats`
  - `orch explain <run-id>` design groundwork or first implementation
  - improved run summaries
- Acceptance Criteria:
  - user can understand why a run passed, failed, or got low confidence

### P6.3 Hardening and rollout policy
- Owner: Runtime + Docs
- Estimate: 2 days
- Dependencies: P6.1, P6.2
- Scope:
  - shadow -> soft -> hard gate rollout
  - release checklist
  - benchmark regression review
- Acceptance Criteria:
  - new gates have rollout mode
  - release candidate process includes benchmark validation

---

## Sprint 1 (Start Immediately)

Primary objective: create the contract foundation and begin Orch-owned planning.

- [ ] Add Task Brief / Structured Plan / Execution Contract / Validation Result models
- [ ] Extend run state + persistence for structured artifacts
- [ ] Implement task normalizer v0
- [ ] Add `orch plan --json`
- [ ] Add acceptance criteria generator v0

### Sprint 1 Task Breakdown

#### S1-T1 Contract types and tests
- Owner: Core Runtime
- Estimate: 1.5 days
- Dependency: None
- Done Criteria:
  - typed models added
  - JSON roundtrip tests added

#### S1-T2 Run state/storage extension
- Owner: Storage
- Estimate: 1.5 days
- Dependency: S1-T1
- Done Criteria:
  - run state includes structured contract fields
  - JSON/SQLite persistence updated or staged for new fields

#### S1-T3 Task normalizer v0
- Owner: Planning
- Estimate: 1 day
- Dependency: S1-T1
- Done Criteria:
  - task type + risk level + normalized goal generation works

#### S1-T4 `orch plan --json`
- Owner: CLI
- Estimate: 1 day
- Dependency: S1-T1
- Done Criteria:
  - JSON plan output available from CLI

#### S1-T5 Acceptance criteria generator v0
- Owner: Planning
- Estimate: 1 day
- Dependency: S1-T3
- Done Criteria:
  - generated plans include acceptance criteria and test requirements

#### S1-T6 Docs + examples update
- Owner: Docs
- Estimate: 0.5 day
- Dependency: S1-T1..S1-T5
- Done Criteria:
  - README/docs mention structured planning direction

---

## Sprint 2 Preview

- deterministic file targeting
- structured plan compiler
- planner redesign
- execution contract builder

## Sprint 3 Preview

- coder I/O redesign
- scope guard
- patch budget enforcement
- plan compliance gate

## Sprint 4 Preview

- gate framework
- test failure classifier
- bounded fix-loop contract
- review rubric

---

## Operating Rules

- Every task must include owner, estimate, dependency, acceptance criteria, and test strategy.
- Any runtime behavior change must add at least one unit or integration test.
- New quality gates should launch in shadow mode first unless they close an active safety gap.
- The model may be swapped; the process contract may not be bypassed.
- Any scope expansion must be explicit and logged.

---

## KPI Tracking

Primary:
- task success rate
- first-pass validation rate
- first-pass test pass rate
- review acceptance rate
- unplanned file touch rate
- retry exhaustion rate
- model variance score

Secondary:
- average run duration
- patch size median
- confidence accuracy
- post-apply defect rate

---

## Risks and Mitigations

- **Over-design risk** -> keep schemas v0-small, expand iteratively.
- **Over-constraining models** -> introduce shadow/soft enforcement before hard fail.
- **Validation slowdown** -> separate targeted and full validation profiles.
- **Benchmark blind spots** -> start with small but representative golden tasks.
- **Prompt regressions** -> move logic into contracts and gates, not prompt wording.

---

## Definition of 90-Day Success

This 90-day plan is successful if by the end of Phase 6:

1. Orch owns the plan structure
2. coding runs use execution contracts
3. validation is gate-based and structured
4. review is rubric-based
5. confidence is visible to the user
6. benchmark tasks exist to measure model variance
7. Orch is meaningfully closer to being a systematic coding engine rather than a free-form agent shell
