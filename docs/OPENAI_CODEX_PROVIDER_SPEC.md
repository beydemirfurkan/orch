# Orch Spec: OpenAI/Codex Provider Integration (Interactive-First)

## Status

- Owner: Core Runtime
- Phase: Post-Phase 3 extension
- Priority: High
- Scope: Provider foundation + OpenAI/Codex first implementation + TUI integration contract

## Objective

Make Orch usable as a real AI coding assistant by replacing stub agent behavior with a production provider layer, starting with OpenAI/Codex.

Target user outcome:

- User runs `orch`
- User types a natural prompt
- Orch executes planner/coder/reviewer via OpenAI/Codex models
- User sees streamed progress and deterministic run artifacts

## Why This Spec Exists

Current gaps:

- Interactive mode exists, but provider/backend selection is not exposed
- Planner/Coder/Reviewer are not connected to a real LLM provider runtime
- No standard auth validation (`doctor`) for provider readiness
- No model routing policy between planner/coder/reviewer roles

This spec defines a provider architecture that keeps Orch deterministic while enabling real LLM execution.

## Non-Goals (This Iteration)

- Multi-provider parity in v1 (Anthropic/Gemini come later)
- Full cloud orchestration service
- Autonomous merge/deploy workflows
- Complex MCP tool ecosystem integration

## Functional Requirements

### F1. Provider Foundation

Implement a provider abstraction under `internal/providers`.

Required interfaces:

- `Provider`:
  - `Name() string`
  - `Validate(ctx) error`
  - `Chat(ctx, request) (response, error)`
  - `Stream(ctx, request) (<-chan event, <-chan error)`
- `Registry`:
  - `Register(provider)`
  - `Get(name)`
- `Router`:
  - role-based model selection (`planner/coder/reviewer`)

### F2. OpenAI/Codex v1 Provider

Add first concrete provider:

- provider name: `openai`
- primary model: `gpt-5.3-codex`
- optional override models:
  - `plannerModel`
  - `coderModel`
  - `reviewerModel`

Supported behavior:

- Non-stream chat completion (mandatory)
- Streaming token/event mode (mandatory for interactive mode)
- Retries for transient failures (429/5xx/timeouts)

### F3. Config and Environment Contract

Extend `.orch/config.json` with provider settings:

```json
{
  "provider": {
    "default": "openai",
    "openai": {
      "apiKeyEnv": "OPENAI_API_KEY",
      "baseURL": "https://api.openai.com/v1",
      "models": {
        "planner": "gpt-5.3-codex",
        "coder": "gpt-5.3-codex",
        "reviewer": "gpt-5.3-codex"
      },
      "reasoningEffort": "medium",
      "timeoutSeconds": 90,
      "maxRetries": 3
    }
  }
}
```

Rules:

- API key is read only from env var, never persisted
- Missing env var fails with actionable message
- Unknown model/provider fails fast

### F4. Agent Runtime Wiring

Replace stub-only execution path in planner/coder/reviewer with provider-backed path.

Expected flow:

- Orchestrator creates agent input
- Agent builds provider request payload
- Router selects model by role
- Provider returns parsed output struct
- Existing pipeline safety and retries remain in effect

### F5. Interactive Mode UX Contract

Interactive mode must clearly show provider context.

Required UI indicators:

- active provider
- active role models
- auth readiness state

Required commands:

- `/provider` -> show current provider config
- `/provider set openai`
- `/model` -> show role model map
- `/model set <role> <model>`

### F6. Doctor Command

Add `orch doctor` to validate runtime readiness.

Minimum checks:

- config parse
- provider exists
- env key exists
- OpenAI auth handshake success
- selected models accessible

Output:

- concise pass/fail summary
- actionable remediation lines

## Technical Design

### Package Layout

- `internal/providers/provider.go` (interfaces and shared types)
- `internal/providers/registry.go`
- `internal/providers/router.go`
- `internal/providers/openai/client.go`
- `internal/providers/openai/mapper.go`
- `internal/providers/openai/stream.go`

### Shared Types

- `ChatRequest`
  - `Role` (`planner|coder|reviewer`)
  - `SystemPrompt`
  - `UserPrompt`
  - `Tools` (future-safe, optional)
  - `Temperature` (optional)
  - `MaxTokens` (optional)
- `ChatResponse`
  - `Text`
  - `FinishReason`
  - `Usage`
  - `ProviderMetadata`
- `StreamEvent`
  - `Type` (`token|tool_call|status|done|error`)
  - `Text`
  - `Metadata`

### Error Taxonomy

Normalize provider failures into stable categories:

- `provider_auth_error`
- `provider_rate_limited`
- `provider_timeout`
- `provider_model_unavailable`
- `provider_transient_error`
- `provider_invalid_response`

Mapped errors must preserve original provider payload in logs.

### Retry Policy

OpenAI provider retry defaults:

- max attempts: 3
- backoff: exponential with jitter
- retryable: timeout, 429, 5xx
- non-retryable: 401, malformed request, model not found

## Security Requirements

- Never write API keys to disk
- Never print full secret values in terminal/logs
- Redact bearer tokens in debug output
- Keep network timeouts bounded

## Observability

Log additions per run:

- provider name
- model per role
- attempt count and retry reason
- request latency
- token usage summary

All provider logs must be attached to existing run logs and persisted.

## TUI Theming (Professional Baseline)

This spec defines a first-pass professional theme baseline (Dracula-inspired, not copy-dependent):

- background: dark slate
- accent: cyan/purple pair
- status colors: green/yellow/red
- high contrast prompt/input area

Theme tokens must be centralized in one file and referenced by TUI components.

## Migration Plan

### Step 1

Create provider abstractions and OpenAI provider with unit tests.

### Step 2

Wire planner/coder/reviewer through router/provider.

### Step 3

Add `orch doctor` and config validation.

### Step 4

Expose provider/model info and commands in interactive mode.

### Step 5

Enable streaming output in interactive mode.

## Testing Plan

### Unit Tests

- provider registry and routing
- model role selection logic
- error mapping and retry behavior
- config parsing and fallback defaults

### Integration Tests

- doctor command with missing/valid env key
- planner/coder/reviewer execution path with provider mock
- streaming event rendering in interactive mode (headless)

### Manual Validation

- `orch doctor` passes with valid OpenAI key
- `orch` interactive run produces non-stub responses
- retries occur on injected transient failure

## Acceptance Criteria

- `orch doctor` validates OpenAI/Codex readiness
- planner/coder/reviewer use provider responses instead of stubs
- interactive mode shows provider/model context
- transient provider failures retry deterministically
- logs include provider/model/retry metadata
- no secret leakage in output or persisted files

## Rollout Strategy

- Feature flag: `provider.openai.enabled`
- Default enabled in local dev after validation
- Keep fallback behavior only for explicit test mode

## Future Extensions

- Add Anthropic and Gemini providers behind same interface
- Add per-session provider/model overrides
- Add tool-calling schema bridge for external integrations
- Add prompt caching and cost metrics
