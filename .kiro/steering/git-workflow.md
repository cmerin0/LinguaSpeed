# Git Workflow Rules for LinguaSpeed

## Branch Model (git-flow)

This project follows **git-flow**:

| Branch | Purpose | Branched from | Merges into |
|---|---|---|---|
| main | Production-ready code only | --- | --- |
| develop | Integration branch, all feature work lands here | main | main (via release) |
| feature/* | One feature or task group per branch | develop | develop |
| release/* | Stabilise a release before tagging | develop | main + develop |
| hotfix/* | Emergency fix on production | main | main + develop |

**Never commit directly to main.** All work flows through develop.

## Rules Kiro must follow

1. All new code goes on a feature/* branch cut from develop.
2. Feature branches are named feature/<task-group> e.g. feature/project-bootstrap, feature/database-schema, feature/ci-cd.
3. After all tasks in a task group are complete, stop and ask the user to review the changes before creating a pull request.
4. The pull request targets develop (not main).
5. Never push directly to main or develop. Always use a PR.
6. Commit messages follow Conventional Commits: type(scope): description e.g. feat(game): implement session creation, chore(ci): add GitHub Actions workflow.
7. Each logical unit of work (one task or closely related tasks) gets its own commit.

## PR Review Gate

After every completed task group, Kiro must:
1. Stage and commit all changes with a descriptive Conventional Commit message.
2. Push the feature branch to origin.
3. Ask the user to review the changes before opening the PR.
4. Only open the PR after the user explicitly approves.

## Secrets

- Never commit .env files.
- Never commit files containing passwords, tokens, or private keys.
- The .env.example file (with placeholder values only) IS committed and safe to commit.
