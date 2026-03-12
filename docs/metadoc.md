# Documentation Guide

This document defines how to write and maintain project docs.

## Goal

Keep docs easy to scan and easy to trust for both humans and agents.

## Principles

- Treat docs like code.
- Keep intent explicit and concise.
- Prefer small focused docs over long mixed docs.
- Link related docs, but keep navigation short.
- Do not version control auto-generated docs.

## Structure

- Keep `README.md` minimal and point to `AGENTS.md`.
- Keep `AGENTS.md` minimal and link to detailed docs.
- Keep topic-specific docs (e.g. `html-css.md`, `testing.md`) focused and link from parent docs.

## Writing Rules

- Use stable terms consistent with current architecture.
- Remove outdated architecture terms immediately.
- Prefer examples that match current runtime behavior.
- In selfspecs, every testable prose claim must have a verification block immediately after it. "X produces Y" without a doctest or verify block is incomplete.
- In guide/skill docs, patterns and pitfalls must include concrete code examples. Abstract advice without illustration is not actionable.

## Link Prefixes

Cross-references between spec documents use semantic link prefixes to express the relationship type. The prefix appears between `[` and the link text, separated by `::`.

- **`depends::`** — The current document functionally depends on the linked document. The linked concept is a prerequisite that must hold for the current spec to make sense. Use in spec-type documents that define behavior.
  - Example: `[depends::adapter protocol](adapter-protocol.spec.md)` — the CLI spec depends on the adapter protocol definition.
- **`explains::`** — The current document introduces or contextualizes the linked document. The link is for onboarding or orientation, not a strict dependency. Use in guide-type documents (`type: guide`) that introduce concepts.
  - Example: `[explains::configuration file](config.spec.md)` — the overview explains what the config file is.

Do not mix prefixes within a single document. A `type: spec` document should use `depends::`. A `type: guide` document should use `explains::`.

Note: `CLAUDE.md` is a symlink to `AGENTS.md`.
