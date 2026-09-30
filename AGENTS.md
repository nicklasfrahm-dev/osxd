# AGENTS.md

## Commits

- Every commit message must be a [Conventional Commit](https://www.conventionalcommits.org/) with a **required scope**: `type(scope): subject`.
- Allowed types are `feat` and `fix` only. **Never** use `chore` (or any other type).
- Write the subject in the imperative mood: `feat(launcher): add fuzzy search`, not `added` or `adds`.

## Pull requests

- The PR title must follow the same rules as commit messages: `feat` or `fix`, a required scope, imperative mood.

## Code layout

- Never use `internal/`. Put all reusable packages under `pkg/`.
- Organize code by feature: put each feature's packages under `pkg/features/<feature>/`. Only code shared across features lives directly under `pkg/`.

## Workflows

- Every job and step in `.github/workflows/` must have a human-readable `name` of the form `verb [noun]`, in the imperative mood: `Run tests`, `Set up Go`, `Build`. Drop the noun only when the verb alone is clear.
- Pin every action to a full commit SHA, followed by a comment naming the exact tag: `uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1`.

## Features

Every feature must be documented:

- Add a bullet to the **Features** section of `README.md` that links to the feature's documentation.
- Document the feature in `docs/<feature>.md`: what it does, how to use and set it up, what it changes on the system, where its code lives, and its known limits.
- Keep the documentation up to date when the feature changes.
