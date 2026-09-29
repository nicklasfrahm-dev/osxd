# AGENTS.md

## Commits

- Every commit message must be a [Conventional Commit](https://www.conventionalcommits.org/) with a **required scope**: `type(scope): subject`.
- Allowed types are `feat` and `fix` only. **Never** use `chore` (or any other type).
- Write the subject in the imperative mood: `feat(launcher): add fuzzy search`, not `added` or `adds`.

## Pull requests

- The PR title must follow the same rules as commit messages: `feat` or `fix`, a required scope, imperative mood.

## Code layout

- Never use `internal/`. Put all reusable packages under `pkg/`.
