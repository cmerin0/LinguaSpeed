---
inclusion: always
---

# Git Workflow Rules for LinguaSpeed

## Branch Model (git-flow)

| Branch | Purpose | Branched from | Merges into |
|---|---|---|---|
| `main` | Production-ready only | -- | -- |
| `develop` | Integration branch | `main` | `main` (via release PR) |
| `feature/*` | One task group per branch | `develop` | `develop` |
| `release/*` | Pre-release stabilisation | `develop` | `main` + `develop` |
| `hotfix/*` | Emergency production fix | `main` | `main` + `develop` |

**Never commit or push directly to `main` or `develop`.**

## What Kiro does after every task group

1. Stage and commit all changes on the current `feature/*` branch using a Conventional Commit message.
2. Push the feature branch to `origin`.
3. **Stop and tell the user** what was built, what tests pass, and what the PR will contain.
4. **Wait for the user to explicitly say** "create the PR" or "go ahead" before opening any pull request.
5. Only then create the PR targeting `develop` (never `main` directly).
6. Do NOT start the next task group until the user has approved and merged the PR.

## Branch naming

`feature/<short-task-group-name>` in kebab-case, for example:
- `feature/project-bootstrap`
- `feature/database-schema`
- `feature/domain-types`
- `feature/scoring`
- `feature/repository-layer`
- `feature/service-layer`
- `feature/handler-layer`
- `feature/frontend`
- `feature/property-tests`
- `feature/integration-tests`
- `feature/ci-cd`

## Commit message format (Conventional Commits)

```
type(scope): short description
```

Types: `feat`, `fix`, `test`, `chore`, `docs`, `refactor`, `ci`

Examples:
- `feat(config): implement env variable loading with fail-fast validation`
- `test(scoring): add property tests for non-negativity and degradation`
- `chore(ci): add GitHub Actions CI workflow with lint and test jobs`

One commit per logical unit of work. Do not batch unrelated changes.

## CI gate awareness

Before creating a PR, remind the user that:
- The `ci.yml` workflow will run lint, unit tests, property tests, and integration tests automatically on push.
- The PR should not be merged until all CI checks are green.
- Branch protection rules on GitHub (Settings > Branches) should require CI to pass before merge is allowed.

## Secrets and sensitive files

- Never commit `.env` files.
- Never commit private keys, tokens, or passwords in any file.
- `.env.example` (with placeholder values only) IS safe to commit.