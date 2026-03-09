# Pi-Inspired Frontend Architecture for Orch

## Objective

Bring the strongest parts of pi's terminal experience and input/runtime structure into Orch **without** giving up Orch's core thesis:

> Orch owns the workflow.
> The model is a bounded worker.

This document defines how Orch should adopt a pi-inspired frontend while keeping Orch's structured planning, validation, review, and confidence model intact.

---

## 1. What We Want to Borrow from Pi

From pi, the most valuable ideas are:

1. **UI/runtime separation**
   - session/runtime logic is separate from TUI rendering
   - interactive mode is only one surface on top of the runtime

2. **Input pipeline before model call**
   - user input can be intercepted, transformed, routed, or handled locally
   - prompt shaping should not require an extra LLM call by default

3. **Component-based terminal UI**
   - messages, tool blocks, composer, footer, dialogs, and widgets are separate components

4. **Comfortable terminal editor UX**
   - multi-line input
   - better visual composition
   - cleaner message cards
   - stronger footer / status model

5. **Theme/token system**
   - message colors, composer state, tool state, review state, confidence state

6. **Future extensibility**
   - input transforms
   - local slash commands
   - overlays / dialogs
   - custom views for Orch artifacts

---

## 2. What We Should Not Copy Blindly

Pi is a flexible harness.
Orch is a workflow control plane.

So Orch should **not** copy these parts naively:

- free-form agent ownership of workflow
- model-led completion decisions
- loose quality semantics
- "patch exists = success" behavior

In Orch:
- the frontend can feel like pi
- but the backend decision model must remain Orch-owned

---

## 3. Product Principle

### Desired split

#### Frontend shell owns:
- input comfort
- command routing UX
- message rendering
- progress rendering
- explanation rendering
- local input transforms
- future overlays/widgets

#### Orch runtime owns:
- task normalization
- structured planning
- execution contract
- validation gates
- test gates
- review rubric
- confidence policy
- completion / revise / fail decisions

---

## 4. Target Layering

```text
User
  |
  v
Pi-inspired Orch Shell
  - editor
  - message timeline
  - footer/status
  - dialogs/widgets
  - local input transform layer
  |
  v
Interactive Session Runtime
  - command routing
  - message lifecycle
  - queueing / steering (future)
  - event emission
  |
  v
Orch Control Plane Backend
  - plan
  - contract
  - validate
  - test
  - review
  - confidence
  - persistence
  |
  v
Providers / Tools / Repo
```

---

## 5. Input Lifecycle

### v1 principle

User input should be processed in this order:

1. **local shell commands**
   - `/help`
   - `/clear`
   - `/run ...`
   - `/plan ...`
   - `/stats`
   - `/explain`

2. **local input transforms**
   - example: `?quick ...`
   - example: future personas or formatting shorthands

3. **workflow routing**
   - plain text -> chat
   - `/plan ...` -> structured planning
   - `/run ...` -> full Orch pipeline

4. **runtime execution**
   - chat request
   - run request
   - CLI command request

5. **structured event emission**
   - input transformed
   - request started
   - provider call started
   - run completed
   - run failed

### Important rule

By default, **do not call a separate LLM just to rewrite the user's prompt**.

Prefer:
- local transforms
- templates
- structured routing
- explicit runtime-owned prompt construction

This matches the best part of pi's design while remaining cheaper, faster, and more deterministic.

---

## 6. Runtime Responsibilities

Orch should grow a session/runtime abstraction similar in spirit to pi's `AgentSession`, but adapted to Orch.

Suggested responsibilities:
- session state
- current mode/status
- input dispatch
- event stream for UI
- queued messages / steering (future)
- chat request execution
- run request execution
- artifact loading for explain/stats

Suggested future type:

```text
InteractiveSession
  Prompt(input)
  Dispatch(input)
  Subscribe(events)
  Abort()
  CurrentState()
```

The TUI should not directly own workflow logic.
It should observe and render runtime events.

---

## 7. TUI Layout Model

A pi-inspired Orch shell should render:

### Header
- product title
- session id / current session name
- provider/auth state
- current mode summary

### Message timeline
- user cards
- assistant cards
- run result cards
- explanation cards
- warning/error cards
- future tool/progress cards

### Composer
- multiline editor
- clear mode/status hints
- input transform hints
- future command autocomplete

### Footer
- cwd / execution root
- model summary
- provider summary
- session summary
- future token/context metrics if introduced

### Overlays / dialogs (future)
- model picker
- command picker
- explain detail view
- run selector
- confirmation dialog

---

## 8. Message/Card Model

The shell should standardize a small set of visual cards.

Initial card types:
- `user_message`
- `assistant_message`
- `input_transform_note`
- `run_result`
- `error`
- `warning`
- `system_note`

Future card types:
- `plan_summary`
- `validation_gate_result`
- `review_scorecard`
- `confidence_report`
- `tool_execution`
- `session_event`

This is important because Orch already has structured artifacts.
The frontend should render them as first-class objects, not only plain text blobs.

---

## 9. Theme / Token System

A pi-inspired Orch frontend should eventually expose tokens for:

### Core UI
- accent
- border
- muted
- dim
- success
- warning
- error
- text

### Message cards
- user card bg/text
- assistant card bg/text
- error card bg/text
- note card bg/text

### Composer states
- idle border
- chat border
- pipeline-running border
- input-transform-active border

### Orch-specific states
- validation pass/warn/fail
- review accept/revise
- confidence high/medium/low/very_low

This is where Orch should extend pi's visual model with **quality-state-aware UI tokens**.

---

## 10. Initial Feature Plan

### Phase 0 — Architecture + terminology
- define frontend/runtime split
- document pi-inspired model
- clarify that Orch will not use a second LLM pass by default for prompt rewriting

### Phase 1 — Shell UX refresh
- improve header/footer/composer layout
- improve message card rendering
- add local input transform support
- make interactive shell feel less like a raw command console

### Phase 2 — Runtime abstraction
- extract interactive dispatch into a reusable runtime/session layer
- add structured events instead of direct UI mutation everywhere

### Phase 3 — Orch-native cards
- render explain/review/confidence artifacts as dedicated cards
- richer run progress cards
- validation/test/review state blocks

### Phase 4 — Queueing and overlays
- steer/follow-up queue
- overlay dialogs
- command picker / run picker / session picker improvements

### Phase 5 — Extensibility
- input middleware hooks
- custom card renderers
- theme customization
- future extension API if needed

---

## 11. Current v0 Implementation Direction

The first Orch implementation step should be intentionally small:

1. introduce a **local input transform layer**
2. improve the **interactive shell layout**
3. render transformed-input notes explicitly
4. move toward a clearer **header / timeline / composer / footer** structure

This provides immediate UX value without destabilizing the backend.

---

## 12. Acceptance Criteria

This frontend direction is on the right track when:
- plain text / slash commands / transformed input are clearly separated
- prompt shaping does not require an extra LLM hop by default
- the shell feels closer to a structured agent workspace than a raw terminal log
- Orch artifacts become renderable UI objects over time
- backend workflow ownership remains inside Orch

---

## 13. Final Positioning

The target is **not**:
- "make Orch become pi"

The target is:
- **make Orch feel as good to use as pi**
- while **keeping Orch's workflow ownership and quality enforcement model**

In short:

> Pi-inspired shell.
> Orch-owned control plane.
