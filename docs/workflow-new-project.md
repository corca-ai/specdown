# Workflow: New Project

Start a new project with specdown from scratch.

## 1. Scaffold

```sh
mkdir my-project && cd my-project
git init
specdown init
```

This creates:

| File | Purpose |
|------|---------|
| `specdown.json` | [Configuration](../specs/config.spec.md) — entry file, adapters, reporters |
| `specs/index.spec.md` | Entry page linking all spec documents |
| `specs/example.spec.md` | Starter spec you'll replace |

## 2. Replace the example

Delete `specs/example.spec.md`. Create your first real spec:

````markdown
# Login

Users authenticate with email and password.

## Happy Path

A valid credential pair returns a session token.

```run:shell
$ echo '{"email":"a@b.com","pass":"secret"}' | my-app login
{"token":"..."}
```
````

Update `specs/index.spec.md` to link to it:

```markdown
# My Project

- [Login](login.spec.md)
```

Run `specdown run` to verify. The built-in shell adapter handles `run:shell` with no extra config.

## 3. Add adapters when needed

The shell adapter covers most projects. When you need domain-specific checks (database state, API responses, UI assertions), register an [adapter](../specs/adapter-protocol.spec.md) in `specdown.json`:

```json
{
  "adapters": [
    {
      "name": "myapp",
      "command": ["python3", "tools/adapter.py"],
      "blocks": ["run:myapp"],
      "checks": ["user-exists", "db-count"]
    }
  ]
}
```

Prefer [check tables](../specs/syntax.spec.md#check-tables) over shell blocks for repetitive assertions — they read as data, not scripts.

## 4. Set up CI

Add to your CI pipeline:

```sh
specdown run -quiet
```

The exit code is non-zero on failure. Use `-quiet` to suppress progress output and show only the summary.

## 5. Install the Claude Code skill

```sh
specdown install skills
```

This gives Claude Code the `/specdown` skill with full syntax reference and adapter protocol knowledge.

## 6. Grow the spec suite

As the project grows:

- One spec file per feature or bounded concern ([best practice](../specs/best-practices.spec.md#keep-documents-focused))
- Add [Alloy models](../specs/alloy.spec.md) when the state space is too large for example-based testing
- Add [traceability](../specs/traceability.spec.md) when you need to track which specs cover which goals
