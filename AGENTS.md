# AGENTS.md

## Commits

- Every commit message must be a [Conventional Commit](https://www.conventionalcommits.org/) with a **required scope**: `type(scope): subject`.
- Allowed types are `feat` and `fix` only. **Never** use `chore` (or any other type).
- Write the subject in the imperative mood: `feat(launcher): add fuzzy search`, not `added` or `adds`.

## Pull requests

- The PR title must follow the same rules as commit messages: `feat` or `fix`, a required scope, imperative mood.

## Code layout

- Never use `internal/`. Put all reusable packages under `pkg/`.

## Workflows

- Every job and step in `.github/workflows/` must have a human-readable `name` of the form `verb [noun]`, in the imperative mood: `Run tests`, `Set up Go`, `Build`. Drop the noun only when the verb alone is clear.
- Pin every action to a full commit SHA, followed by a comment naming the exact tag: `uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1`.
