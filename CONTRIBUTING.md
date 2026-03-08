# Contributing to Orch

Thank you for your interest in contributing.

## Development Setup

Requirements:

- Go 1.25+

Setup:

```bash
go mod tidy
go test ./...
```

## Project Conventions

- Keep source and documentation content in English.
- Follow existing package boundaries and naming patterns.
- Prefer small, focused changes over broad rewrites.
- Add or update tests for behavior changes.
- Keep safety-related behavior explicit and deterministic.

## Language Guard

This repository enforces English-only source/docs checks.

Run locally:

```bash
bash scripts/check-language.sh
```

If an exception is required, add a single-line regex rule to `.language-guard-allowlist` with a short explanation.

## Pull Request Guidelines

Before opening a PR:

- Run `go test ./...` and ensure all tests pass.
- Ensure CLI behavior changes are covered by integration or command tests.
- Update relevant docs (`README.md`, specs under `docs/`) when behavior changes.
- Keep commit messages clear and scoped.

PR checklist:

- [ ] Tests added or updated
- [ ] Documentation updated
- [ ] No unrelated refactors
- [ ] Safety implications reviewed

## Reporting Issues

When reporting a bug, include:

- OS and Go version
- command used
- expected behavior
- actual behavior
- relevant logs from `.orch/runs/`
