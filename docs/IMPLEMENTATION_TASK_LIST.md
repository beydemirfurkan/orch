# Orch Implementation Task List

This document converts the systematic coding roadmap into an execution-ready engineering backlog.

Related docs:
- `docs/SYSTEMATIC_CODING_ROADMAP.md`
- `docs/QUALITY_SYSTEM_SPEC.md`
- `docs/EXECUTION_CONTRACT_SPEC.md`
- `docs/PLANNING_ENGINE_SPEC.md`
- `docs/SPRINT_BOARD_90_DAY_PLAN.md`

---

## Priority Model

- **P0**: Required to make Orch a true systematic coding engine
- **P1**: Strongly recommended for quality/reliability
- **P2**: Important for scale, UX, and model-agnostic measurement

Status legend:
- [ ] Not started
- [~] In progress
- [x] Done
- [!] Blocked

---

# Track A — Contracts and Core Data Model

## A1. Task Brief contract
- Priority: P0
- Owner: Core Runtime
- Status: [ ]
- Deliverables:
  - `TaskBrief` model
  - task type classification enum
  - risk level enum
  - assumptions/constraints/success definition fields
- Acceptance:
  - every run has a persisted task brief
  - task brief serializes to JSON cleanly
  - planner/coder/reviewer can consume it without free-form parsing

## A2. Structured Plan contract
- Priority: P0
- Owner: Planning
- Status: [ ]
- Deliverables:
  - `StructuredPlan` model
  - steps, files, invariants, forbidden changes, test requirements, acceptance criteria
- Acceptance:
  - `orch plan --json` returns structured plan
  - plan output is valid without provider runtime

## A3. Execution Contract model
- Priority: P0
- Owner: Execution Engine
- Status: [ ]
- Deliverables:
  - allowed files
  - required edits
  - prohibited actions
  - patch size budget
  - invariant/criterion obligations
- Acceptance:
  - coder receives execution contract on every run
  - contract persists in run manifest

## A4. Review Scorecard model
- Priority: P0
- Owner: Review Engine
- Status: [ ]
- Deliverables:
  - rubric dimensions
  - decision enum `accept|revise|reject`
  - findings list
- Acceptance:
  - reviewer result is structured
  - revise decision includes explicit findings

## A5. Validation Result model
- Priority: P0
- Owner: Quality
- Status: [ ]
- Deliverables:
  - named gates
  - severity
  - pass/fail state
  - actionable findings
- Acceptance:
  - validation is stored as gate list instead of a single string

## A6. Confidence Score model
- Priority: P1
- Owner: Runtime + Review
- Status: [ ]
- Deliverables:
  - score field
  - rationale field
  - contributors metadata
- Acceptance:
  - every completed run exposes a confidence score and rationale

## A7. Run Manifest persistence
- Priority: P0
- Owner: Storage
- Status: [ ]
- Deliverables:
  - SQLite columns / JSON serialization support
  - CLI rendering helpers
- Acceptance:
  - run manifest can be loaded from storage and displayed in CLI

---

# Track B — Orch-Owned Planning

## B1. Task normalizer
- Priority: P0
- Owner: Planning
- Status: [ ]
- Deliverables:
  - classify task type: feature, bugfix, tests, refactor, docs, chore
  - normalize user goal
  - derive initial risk level
- Acceptance:
  - same task text produces stable normalized brief

## B2. Deterministic file targeting heuristics
- Priority: P0
- Owner: Planning + Repo
- Status: [ ]
- Deliverables:
  - repo-aware heuristics using language/framework/package manager/test framework
  - candidate inspect/modify file ranking
- Acceptance:
  - produces ranked file list with reasons

## B3. Acceptance criteria generator
- Priority: P0
- Owner: Planning
- Status: [ ]
- Deliverables:
  - criterion templates per task type
- Acceptance:
  - every plan includes measurable acceptance criteria

## B4. Invariant generator
- Priority: P1
- Owner: Planning
- Status: [ ]
- Deliverables:
  - invariant extraction templates
- Acceptance:
  - plans explicitly define what must not break

## B5. Forbidden change generator
- Priority: P1
- Owner: Planning + Quality
- Status: [ ]
- Deliverables:
  - forbidden edits from repo policy + task type
- Acceptance:
  - plan lists forbidden changes when risk profile requires them

## B6. Planner redesign
- Priority: P0
- Owner: Agents
- Status: [ ]
- Deliverables:
  - planner becomes refinement worker, not plan owner
- Acceptance:
  - final plan shape comes from Orch compiler

## B7. `orch plan --json`
- Priority: P0
- Owner: CLI
- Status: [ ]
- Deliverables:
  - JSON output mode
  - optional `--verbose-reasons`
- Acceptance:
  - machine-readable planning works in CI/scripts

---

# Track C — Constrained Coding Runtime

## C1. Execution contract builder
- Priority: P0
- Owner: Execution Engine
- Status: [ ]
- Deliverables:
  - plan + context -> execution contract
- Acceptance:
  - every coder run has a bounded scope and patch budget

## C2. Coder input redesign
- Priority: P0
- Owner: Agents
- Status: [ ]
- Deliverables:
  - task brief + plan + execution contract + selected context
- Acceptance:
  - coder prompt/request is structured and role-specific

## C3. Coder output redesign
- Priority: P0
- Owner: Agents
- Status: [ ]
- Deliverables:
  - unified diff
  - changed file summary
  - criterion mapping
  - assumptions
- Acceptance:
  - coder output can be validated without NLP guessing

## C4. Scope guard
- Priority: P0
- Owner: Quality
- Status: [ ]
- Deliverables:
  - detect changes outside allowed files
  - identify unrelated edits
- Acceptance:
  - out-of-scope patch fails before testing

## C5. Minimal diff enforcer
- Priority: P1
- Owner: Execution Engine
- Status: [ ]
- Deliverables:
  - formatting churn detection
  - opportunistic refactor detection
- Acceptance:
  - patch bloat is classified and rejected when unnecessary

## C6. Controlled scope expansion
- Priority: P1
- Owner: Orchestrator
- Status: [ ]
- Deliverables:
  - explicit mechanism for justified plan expansion
- Acceptance:
  - scope expansion is logged and reviewable

---

# Track D — Validation and Quality Gates

## D1. Gate framework
- Priority: P0
- Owner: Quality
- Status: [ ]
- Deliverables:
  - gate interface
  - gate runner
  - structured result aggregator
- Acceptance:
  - validation pipeline is composable and ordered

## D2. Patch hygiene gate
- Priority: P0
- Owner: Patch Engine
- Status: [ ]
- Deliverables:
  - binary/sensitive/protected/generated-file checks
- Acceptance:
  - hygiene failures are explicit and test-covered

## D3. Plan compliance gate
- Priority: P0
- Owner: Quality
- Status: [ ]
- Deliverables:
  - required files changed?
  - forbidden changes violated?
  - acceptance criteria addressed?
- Acceptance:
  - plan compliance is visible as a first-class gate

## D4. Syntax/parse gate
- Priority: P1
- Owner: Quality
- Status: [ ]
- Deliverables:
  - language adapters for syntax checks
- Acceptance:
  - syntax failures do not wait until full test stage

## D5. Build/typecheck gate
- Priority: P1
- Owner: Quality
- Status: [ ]
- Deliverables:
  - Go build / JS-TS typecheck adapter contract
- Acceptance:
  - compile failures are normalized into stable categories

## D6. Static analysis gate
- Priority: P1
- Owner: Quality
- Status: [ ]
- Deliverables:
  - lint / vet / static warnings integration
- Acceptance:
  - gate can run in strict or warning mode

## D7. Validation profiles
- Priority: P1
- Owner: Quality
- Status: [ ]
- Deliverables:
  - Go profile
  - JS/TS profile
- Acceptance:
  - runtime chooses validation profile deterministically

---

# Track E — Test Intelligence and Fix Loops

## E1. Test matrix model
- Priority: P0
- Owner: Testing
- Status: [ ]
- Deliverables:
  - unit/integration/e2e/smoke requirement model
- Acceptance:
  - plan contains explicit test requirements

## E2. Targeted test selector
- Priority: P1
- Owner: Testing + Repo
- Status: [ ]
- Deliverables:
  - changed file -> likely test mapping
- Acceptance:
  - selector can choose targeted tests before full suite escalation

## E3. Test failure classifier
- Priority: P0
- Owner: Testing
- Status: [ ]
- Deliverables:
  - compile/assertion/timeout/flaky/missing coverage classification
- Acceptance:
  - retry loop receives structured failure reasons

## E4. Bounded fix-loop prompt builder
- Priority: P0
- Owner: Orchestrator
- Status: [ ]
- Deliverables:
  - gate failure + test failure -> targeted retry contract
- Acceptance:
  - retry requests are deterministic and failure-specific

## E5. Retry memory
- Priority: P1
- Owner: Runtime
- Status: [ ]
- Deliverables:
  - previous attempts summary
  - anti-loop markers
- Acceptance:
  - system detects repeated failure patterns

---

# Track F — Review and Confidence

## F1. Review rubric design
- Priority: P0
- Owner: Review
- Status: [ ]
- Deliverables:
  - requirement coverage
  - scope control
  - regression risk
  - readability
  - maintainability
  - test adequacy
- Acceptance:
  - every review outputs rubric scores

## F2. Decision policy
- Priority: P0
- Owner: Review + Orchestrator
- Status: [ ]
- Deliverables:
  - scoring thresholds for accept/revise/reject
- Acceptance:
  - decisions are deterministic given scorecard + policy

## F3. Confidence scorer v1
- Priority: P1
- Owner: Confidence
- Status: [ ]
- Deliverables:
  - confidence formula using validation/test/review/retry/scope signals
- Acceptance:
  - score is reproducible and logged

## F4. CLI confidence UX
- Priority: P1
- Owner: CLI
- Status: [ ]
- Deliverables:
  - confidence display
  - risk explanation display
- Acceptance:
  - user sees confidence and unresolved risk in summaries

---

# Track G — Benchmarks and Model Agnosticism

## G1. Golden task suite
- Priority: P1
- Owner: Bench
- Status: [ ]
- Deliverables:
  - at least 20 benchmark tasks
- Acceptance:
  - tasks cover bugfix, feature, tests, refactor, config, concurrency

## G2. Replay harness
- Priority: P1
- Owner: Bench
- Status: [ ]
- Deliverables:
  - same task across different models/providers
- Acceptance:
  - outputs are scoreable and comparable

## G3. Model variance report
- Priority: P1
- Owner: Bench + Docs
- Status: [ ]
- Deliverables:
  - success diff
  - patch diff
  - retry diff
  - score diff
- Acceptance:
  - variance report can be generated from benchmark history

## G4. Benchmark CI gate
- Priority: P2
- Owner: CI
- Status: [ ]
- Deliverables:
  - regression benchmark workflow
- Acceptance:
  - release candidates are checked against benchmark thresholds

---

# Track H — Developer UX and Explainability

## H1. Run summary redesign
- Priority: P1
- Owner: CLI
- Status: [ ]
- Deliverables:
  - task brief, plan, changed files, validation/test/review/confidence summary
- Acceptance:
  - run output is explanation-first, not just status-first

## H2. `orch explain <run-id>`
- Priority: P1
- Owner: CLI + Runtime
- Status: [ ]
- Deliverables:
  - explain why files were selected
  - explain why review decision occurred
  - explain confidence
- Acceptance:
  - failed and successful runs can be explained from persisted state

## H3. `orch stats`
- Priority: P2
- Owner: Runtime + Storage
- Status: [ ]
- Deliverables:
  - KPI summary command
- Acceptance:
  - key quality metrics visible in one command

---

# First 3 Execution Milestones

## Milestone M1 — Contracts + Planning (Weeks 1-4)
- A1, A2, A3, A5, B1, B2, B3, B6, B7

## Milestone M2 — Constrained Coding + Validation (Weeks 5-8)
- C1, C2, C3, C4, D1, D2, D3, E3, E4

## Milestone M3 — Review + Confidence + Benchmarks (Weeks 9-12)
- F1, F2, F3, H1, H2, G1

---

# Recommended Start Order

1. A1 Task Brief contract
2. A2 Structured Plan contract
3. A3 Execution Contract model
4. B1 Task normalizer
5. B3 Acceptance criteria generator
6. B7 `orch plan --json`
7. C1 Execution contract builder
8. C4 Scope guard
9. D1 Gate framework
10. D3 Plan compliance gate
11. E4 Bounded fix-loop prompt builder
12. F1 Review rubric

---

# Definition of Backlog Readiness

Bir task implementasyona alınmadan önce şunlar net olmalı:
- owner
- dependency
- acceptance criteria
- test strategy
- rollout mode (shadow / soft / hard)
- persistence impact

---

# Definition of Delivery Success

Bu backlog başarılı şekilde ilerliyor sayılır eğer:
- Orch planı üretip sahipleniyorsa
- coder scope dışına çıktığında fail oluyorsa
- validation/test/review structured hale geliyorsa
- confidence kullanıcıya görünür hale geldiyse
- benchmark set ile model variance ölçülebiliyorsa
