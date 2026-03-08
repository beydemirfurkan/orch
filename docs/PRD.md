# Product Requirements Document
## Project: AI Coding Orchestrator CLI
## Codename: Orch

---

# 1. Overview

Orch is a CLI-based AI orchestration engine that executes coding tasks inside a repository using multiple AI agents.

Instead of a chat-based coding assistant, Orch operates as a task execution engine:

Task → Plan → Code → Test → Review → Patch

The system analyzes a repository, selects relevant context, generates a plan using AI, produces code changes as a unified diff patch, validates the patch, runs tests, and presents the result to the user.

The user remains in control and must explicitly approve code changes.

Primary goal:
Create a deterministic, auditable, high-performance AI-powered coding workflow tool.

---

# 2. Goals

Primary goals:

- Provide a CLI-first developer experience
- Execute coding tasks using multi-agent orchestration
- Generate code changes as unified diff patches
- Ensure safety through dry-run and validation
- Maintain full execution trace for transparency
- Work locally on developer repositories
- Be extremely fast and reliable

Non-goals (for MVP):

- No GUI
- No plugin marketplace
- No collaboration features
- No cloud dependency required
- No autonomous auto-merge

---

# 3. Target Users

Primary users:

- Software engineers
- AI-assisted developers
- Open source maintainers
- DevOps engineers

User profile:

Developers comfortable using terminal tools who want AI to execute structured coding tasks inside their repositories.

---

# 4. Core Use Cases

### 4.1 Implement Feature

User runs:

orch run "add redis caching to user service"

System:

1. analyzes repository
2. produces implementation plan
3. edits code
4. generates diff patch
5. runs tests
6. reviews changes
7. presents patch

---

### 4.2 Fix Bug

orch run "fix race condition in auth service"

System:

- locates relevant files
- generates fix
- tests changes

---

### 4.3 Write Tests

orch run "write unit tests for payment service"

System:

- finds target code
- generates test cases
- validates compilation/tests

---

### 4.4 Refactor Code

orch run "refactor user service for readability"

---

# 5. CLI Experience

CLI must be simple, predictable and fast.

Commands:

orch init
orch plan "<task>"
orch run "<task>"
orch logs
orch diff
orch apply

---

## orch init

Analyzes repository and creates configuration.

Outputs:

.orch/config.json  
.orch/repo-map.json

---

## orch plan

Produces AI implementation plan without editing code.

Example:

orch plan "add redis caching"

Output:

- files to inspect
- files to modify
- implementation steps

---

## orch run

Executes full pipeline.

Pipeline:

Task
→ Repo analysis
→ Context selection
→ Planner agent
→ Coder agent
→ Patch validation
→ Test runner
→ Reviewer agent
→ Result

---

## orch diff

Shows generated unified diff patch.

---

## orch apply

Applies generated patch to working tree.

Default mode must be dry-run.

---

## orch logs

Displays execution trace of agent workflow.

Example:

[analyzer] scanning repository
[planner] generating plan
[coder] editing userService.ts
[test] running npm test
[reviewer] patch approved

---

# 6. System Architecture

Core architecture:

CLI
↓
Orchestrator
↓
Agent Layer
↓
Tool Layer
↓
Repository

---

## 6.1 Orchestrator

Central engine controlling execution.

Responsibilities:

- manage workflow steps
- maintain run state
- coordinate agents
- enforce safety rules
- manage retries
- record logs

---

## 6.2 Agents

Agents are AI-driven modules with strict responsibilities.

### Planner Agent

Input:

- task description
- repo summary
- file index

Output:

structured plan:

- steps
- files_to_modify
- files_to_inspect
- risks
- test strategy

---

### Coder Agent

Input:

- plan
- relevant files

Output:

unified diff patch

Rules:

- minimal changes
- follow existing code style
- avoid unrelated edits

---

### Reviewer Agent

Input:

- patch
- plan
- test results

Output:

decision:

accept | revise

---

# 7. Patch Pipeline

System must use unified diff format.

Example:

diff --git a/userService.ts b/userService.ts
+ import redis

Patch stages:

1. generate
2. parse
3. validate
4. preview
5. apply

Validation rules:

- no binary files
- no .env changes
- patch size limits

---

# 8. Repository Analyzer

Analyzer extracts repository structure.

Responsibilities:

- detect language
- detect package manager
- detect test framework
- build file inventory
- build import relationships
- generate repo map

Outputs:

repo-map.json

---

# 9. Context Builder

Selects relevant files for a task.

Inputs:

- task
- repo map
- planner hints

Outputs:

selected files
related tests
relevant configs

Goal:

minimize context size while maximizing relevance.

---

# 10. Tool Layer

Tools available to agents:

read_file
write_file
search_code
list_files
run_command
run_tests
git_diff
apply_patch

Tools must be deterministic and logged.

---

# 11. Run State Machine

Run states:

created
analyzing
planning
coding
validating
testing
reviewing
completed
failed

Each step emits events.

---

# 12. Execution Trace

All runs must produce logs.

Example:

timestamp
actor
step
message

Logs stored in:

.orch/runs/

---

# 13. Configuration

.orch/config.json

Example:

{
  "version": 1,
  "models": {
    "planner": "openai:gpt-4o-mini",
    "coder": "anthropic:claude-sonnet",
    "reviewer": "openai:gpt-4o-mini"
  },
  "commands": {
    "test": "npm test",
    "lint": "npm run lint"
  },
  "patch": {
    "maxFiles": 10,
    "maxLines": 800
  },
  "safety": {
    "dryRun": true
  }
}

---

# 14. Security & Safety

System must never automatically apply patches.

Safety rules:

- dry-run by default
- patch preview required
- block sensitive files
- enforce patch limits

---

# 15. Performance Requirements

CLI startup: < 100ms

Typical task execution:

under 30 seconds for small tasks.

Repository scan must support large repos.

---

# 16. MVP Scope

MVP includes:

CLI
Repo analyzer
Planner agent
Coder agent
Patch generation
Patch validation
Test execution
Reviewer agent
Logs

Excluded from MVP:

GUI
Cloud execution
Plugin system
Team collaboration

---

# 17. Technology Stack

Language: Go

CLI framework: Cobra

LLM providers:

OpenAI
Anthropic
OpenRouter

Patch processing: internal module

Storage: local filesystem

---

# 18. Success Metrics

Key metrics:

patch generation success rate
patch apply success rate
test pass rate
review acceptance rate

Initial target:

40% successful task completion.

---

# 19. Future Roadmap

Possible extensions:

VSCode extension
Web dashboard
Agent marketplace
Cloud execution
Team collaboration
Advanced repo indexing
Learning system for repo memory

---

# End of PRD