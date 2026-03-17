# Workflow: Adopting specdown into an Existing Project

Add specdown to a project that already has code, tests, and documentation.

## 1. Install and configure

```sh
specdown init
```

If the default directory layout doesn't fit (e.g., docs live in `docs/specs/` instead of `specs/`), edit `specdown.json` manually:

```json
{
  "entry": "docs/specs/index.spec.md",
  "adapters": []
}
```

All paths are relative to the config file location. See [Configuration](../specs/config.spec.md).

## 2. Choose what to spec first

Don't try to spec everything at once. Start with one well-understood feature where:

- The behavior is stable and well-defined
- You can verify it with shell commands or an existing test harness
- Stakeholders would benefit from a readable specification

Good first candidates: API contracts, CLI behavior, configuration validation, data format rules.

## 3. Write a regression spec

Your first spec should document **existing behavior**, not new features. This gives you a safety net before making changes.

````markdown
# User API

## Create User

A POST to `/api/users` with a valid payload returns 201.

```run:shell
$ curl -s -o /dev/null -w '%{http_code}' -X POST \
    -H 'Content-Type: application/json' \
    -d '{"name":"alice"}' \
    http://localhost:3000/api/users
201
```
````

If the project needs setup before specs run (database, containers), use the [global setup/teardown](../specs/config.spec.md#global-setup-and-teardown) config:

```json
{
  "setup": "docker compose up -d && sleep 2",
  "teardown": "docker compose down"
}
```

## 4. Bridge existing test infrastructure

If you have a test harness, API client, or CLI wrapper, turn it into an [adapter](../specs/adapter-protocol.spec.md). The adapter receives JSON commands on stdin and returns results on stdout — any language works.

This lets you write specs with [check tables](../specs/syntax.spec.md#check-tables) instead of shell scripts:

```markdown
> check:user-api(method=POST, endpoint=/api/users)
| name  | expected_status |
| alice | 201             |
| ""    | 400             |
```

## 5. Grow incrementally

| When | Do |
|------|----|
| Adding a new feature | Write the spec first, then implement ([new feature workflow](workflow-evolve.md#adding-a-new-feature)) |
| Fixing a bug | Add a failing spec case that reproduces the bug, then fix |
| Refactoring | Ensure the affected behavior has specs before refactoring |
| Onboarding a team member | Point them to the HTML report — it's the spec and the test results in one |

## 6. Optional: add traceability

If the project has layered documentation (goals, features, stories), add [traceability](../specs/traceability.spec.md) to enforce coverage:

```json
{
  "trace": {
    "types": ["goal", "feature", "test"],
    "edges": {
      "covers":  { "from": "goal",    "to": "feature", "count": "1 → 1..*" },
      "tests":   { "from": "feature", "to": "test",    "count": "1 → 1..*" }
    }
  }
}
```

Edges follow UML dependency direction: `from` is the dependent, `to` is the dependency. A goal depends on features for its fulfillment; a feature depends on tests for its verification.

Run `specdown trace -strict` in CI to enforce coverage.
