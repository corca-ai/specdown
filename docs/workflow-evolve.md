# Workflow: Evolving an Existing Spec Suite

Work within a project that already uses specdown — adding features, changing behavior, or strengthening specs.

## Adding a new feature

Write the spec before (or alongside) the implementation.

1. Create a new `.spec.md` file. One file per feature ([best practice](../specs/best-practices.spec.md#keep-documents-focused)).
2. Start with prose: what the feature does and why it exists.
3. Add executable blocks and check tables that define the expected behavior. These will fail until the implementation is ready.
4. Link the new file from the index: `- [Feature Name](feature-name.spec.md)`.
5. Run `specdown run -filter "Feature Name"` to verify just the new spec as you implement.
6. When all cases pass, run `specdown run` to confirm nothing else broke.

If the project uses [traceability](../specs/traceability.spec.md), add the appropriate trace links (e.g., `[covers::Feature](feature.spec.md)` in the parent goal document).

## Changing existing behavior

The spec is the source of truth for intended behavior. When behavior changes:

1. **Read the existing spec** to understand the current contract.
2. **Update the spec first** — modify prose, expected outputs, and check table values to reflect the new behavior.
3. **Run the spec** — it should fail against the old implementation.
4. **Implement the change** — make the spec pass.
5. **Run the full suite** — `specdown run` to catch unintended side effects.

If a spec case becomes irrelevant, remove it. If a new edge case appears, add it.

## Strengthening specs

Improve coverage without changing behavior.

### Use doctest blocks for CLI output

When a spec verifies CLI output, doctest style (`$ ` command lines with expected output) is often clearer than a plain `run:shell` with manual `test` commands:

````markdown
```run:shell
$ my-cli whoami
alice@example.com (admin)
$ my-cli list
workspace-1
workspace-2
```
````

Use `...` to skip lines that change between runs (timestamps, IDs):

````markdown
```run:shell
$ my-cli status
connected to ...
uptime: ...
```
````

See [Doctest Blocks](../specs/syntax.spec.md#doctest-blocks) for full syntax.

### Use inline assertions in prose

When a single value check belongs in the narrative rather than a separate block, use `` `expect:` ``:

```markdown
After provisioning, the workspace count is `expect: ${count} == 3`.
```

This renders as a green/red badge in the HTML report without interrupting the prose flow. See [Inline expect](../specs/syntax.spec.md#inline-expect).

### Add check tables for repeated patterns

If you see three or more `run:shell` blocks testing variations of the same thing, refactor into a [check table](../specs/syntax.spec.md#check-tables):

Before:
````markdown
```run:shell
$ my-cli validate good.json && echo ok
ok
```

```run:shell
$ my-cli validate bad.json 2>&1 && echo ok || echo fail
fail
```
````

After:
```markdown
> check:validate
| input    | expected |
| good.json | ok      |
| bad.json  | fail    |
```

This requires an [adapter check](../specs/adapter-protocol.spec.md), but the spec becomes pure data.

For JSON data, the built-in `check:jq` works without an adapter:

```markdown
> check:jq
| input           | expr   | expected |
| {"a":1, "b":2}  | .a + .b | 3       |
```

### Add Alloy models for structural properties

When the state space is too large for examples, add an [Alloy model](../specs/alloy.spec.md) in the same section as the implementation checks. Use models for:

- Invariants that must hold for all combinations (e.g., "every item has exactly one owner")
- State machine transition safety (e.g., "no direct jump from state A to state C")
- Case classification completeness (e.g., "the three access levels cover all subjects")

See [Alloy patterns](../specs/best-practices.spec.md#patterns) for worked examples.

### Add traceability

If the project has goals or feature documents, add [trace edges](../specs/traceability.spec.md) to ensure coverage. Run `specdown trace -strict` to find gaps.

## Fixing a broken spec

When `specdown run` fails:

1. **Read the failure output** — the CLI shows `expected` vs `actual` values for both doctest mismatches and check table failures.
2. **Determine the cause**:
   - **Implementation bug** — fix the code, not the spec.
   - **Spec is wrong** — the spec described the wrong behavior. Update the spec.
   - **Environment issue** — missing dependency, stale state, ordering problem. Fix setup/teardown or add [hooks](../specs/syntax.spec.md#setup-and-teardown-hooks).
3. **Re-run** with `-filter` to iterate quickly: `specdown run -filter "Section Name"`.

## Iterating quickly with filters

Use `-filter` to run a subset of cases during development:

```sh
specdown run -filter "Login"              # heading substring
specdown run -filter type:table           # only check tables
specdown run -filter block:shell          # only shell blocks
specdown run -filter check:jq             # only jq checks
```

Only one filter can be active at a time. See [Filter](../specs/cli.spec.md#filter).

## Refactoring specs

- **Split a large spec** into focused files. Update the index and trace links.
- **Extract repeated shell logic** into an adapter check. The spec becomes a table; the plumbing moves to the adapter.
- **Add summary lines** to long shell blocks so the report stays scannable. See [Summary Lines](../specs/syntax.spec.md#summary-lines).
