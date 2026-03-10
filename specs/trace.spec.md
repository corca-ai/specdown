---
type: spec
---

# Trace Graph

Consider a project with four layers of documentation: Themes describe
business goals, Epics break themes into deliverables, User Stories
describe end-user behavior, and Acceptance Tests verify each story.
How do you know every story has at least one test? That every epic
traces back to a theme? That no test is orphaned?

The trace graph answers these questions. Documents are nodes; named,
typed links between them are edges. Configure edge types and
cardinality constraints in [depends::specdown.json](config.spec.md),
and specdown checks them automatically:

```json
{
  "trace": {
    "types": ["theme", "epic", "story", "at"],
    "edges": {
      "decomposes": { "from": "epic",  "to": "theme", "count": "1 → 1..*" },
      "covers":     { "from": "story", "to": "epic",  "count": "1 → 1..*" },
      "tests":      { "from": "at",    "to": "story", "count": "1 → 1..*" }
    }
  }
}
```

With this configuration, `specdown trace --strict` fails if any epic
lacks a theme link, any story is not covered by an acceptance test,
or any document violates its declared cardinality. Everything else
is derived.

## Authoring Surface

### Node Type — Frontmatter

A document's type is declared in YAML frontmatter:

```yaml
---
type: spec
---
```

A type is an identifier (`[a-z][a-z0-9_]*`). Valid types must be declared in the
`trace.types` config array. A document with a type not listed in config is an error.
Documents without `type` are untyped — they may be navigation link targets but
cannot be trace link sources or targets.

### Trace Edges — Markdown Links with Edge Name Prefix

Trace links use standard markdown link syntax with an `<edge_name>::` prefix in the
link text:

```markdown
This feature [covers::Login Story](../stories/login.md) and
[covers::Password Reset](../stories/password-reset.md).
```

- Standard markdown links — render and click in any viewer
- Link text starts with `<edge_name>::` where edge name is declared in config
- Text after `::` is the display text — it is required
- Plain links without the `<identifier>::` pattern are navigation links — no trace semantics
- Edge name is always required — no inference or shorthand

### Documents Are Atoms

One document = one node. No heading-level references. Fragment anchors in links
are stripped before resolution — documents are atoms.

## Configuration

The `trace` key in `specdown.json` configures the trace graph:

```json
{
  "trace": {
    "types": ["goal", "feature", "test"],
    "ignore": ["vendor/**", "third_party/**"],
    "edges": {
      "covers":   { "from": "feature", "to": "goal",    "count": "1..* → 1..*" },
      "tests":    { "from": "test",    "to": "feature", "count": "1 → 1..*" },
      "requires": { "from": "feature", "to": "feature", "acyclic": true, "transitive": true }
    }
  }
}
```

### Config Validation

Types must be valid identifiers.

```run:shell
# Reject invalid type name
mkdir -p .tmp-test/trace
cat <<'CFG' > .tmp-test/trace/bad-type.json
{"entry":"i.spec.md","adapters":[],"trace":{"types":["Good"],"edges":{"x":{"from":"Good","to":"Good"}}}}
CFG
! specdown trace -config .tmp-test/trace/bad-type.json 2>/dev/null
```

Edge names must be valid identifiers.

```run:shell
# Reject invalid edge name (config-level)
cat <<'CFG' > .tmp-test/trace/bad-edge.json
{"entry":"i.spec.md","adapters":[],"trace":{"types":["goal"],"edges":{"Bad-Name":{"from":"goal","to":"goal"}}}}
CFG
! specdown trace -config .tmp-test/trace/bad-edge.json 2>/dev/null
```

Edge `from`/`to` must reference declared types.

```run:shell
# Reject edge referencing undeclared type
cat <<'CFG' > .tmp-test/trace/undeclared-type.json
{"entry":"i.spec.md","adapters":[],"trace":{"types":["goal"],"edges":{"covers":{"from":"feature","to":"goal"}}}}
CFG
! specdown trace -config .tmp-test/trace/undeclared-type.json 2>/dev/null
```

### Count Notation

Format: `<source> → <target>`, where each side is a UML multiplicity.
Both `→` (U+2192) and `->` (ASCII) are accepted.

| Multiplicity | Meaning |
|---|---|
| `1` | exactly one |
| `0..*` | zero or more |
| `1..*` | one or more |
| `0..1` | zero or one |

Source side (left of `→`): how many targets each source document must link to.
Target side (right of `→`): how many sources each target document must be linked from.

Omitted `count` defaults to `0..* → 0..*` (no constraints).

## Discovery

When trace is configured, specdown scans the entire directory tree rooted at
the config file location. All `.md` files are included unless they match an
`ignore` pattern.

```run:shell
# Discovery finds all .md files with types
mkdir -p .tmp-test/trace/disc
cat <<'CFG' > .tmp-test/trace/disc/specdown.json
{"entry":"specs/index.spec.md","adapters":[],"trace":{"types":["goal","feature"],"edges":{"covers":{"from":"feature","to":"goal"}}}}
CFG
mkdir -p .tmp-test/trace/disc/specs .tmp-test/trace/disc/goals .tmp-test/trace/disc/features
printf '# Index\n' > .tmp-test/trace/disc/specs/index.spec.md
printf -- '---\ntype: goal\n---\n# G1\n' > .tmp-test/trace/disc/goals/g1.md
printf -- '---\ntype: feature\n---\n# F1\n\n[covers::G1](../goals/g1.md)\n' > .tmp-test/trace/disc/features/f1.md
specdown trace -config .tmp-test/trace/disc/specdown.json 2>&1 | grep -c '"type"'
```

```run:shell
$ specdown trace -config .tmp-test/trace/disc/specdown.json 2>&1 | grep -c '"type"'
2
```

## Link Errors

### Unknown Edge Name

A trace link using an undeclared edge name is an error.

```run:shell
# Unknown edge name is reported
mkdir -p .tmp-test/trace/unknown-edge
cat <<'CFG' > .tmp-test/trace/unknown-edge/specdown.json
{"entry":"specs/index.spec.md","adapters":[],"trace":{"types":["feature"],"edges":{"covers":{"from":"feature","to":"feature"}}}}
CFG
mkdir -p .tmp-test/trace/unknown-edge/specs
printf '# Index\n' > .tmp-test/trace/unknown-edge/specs/index.spec.md
printf -- '---\ntype: feature\n---\n# F1\n\n[bogus::something](f2.md)\n' > .tmp-test/trace/unknown-edge/f1.md
printf -- '---\ntype: feature\n---\n# F2\n' > .tmp-test/trace/unknown-edge/f2.md
specdown trace -config .tmp-test/trace/unknown-edge/specdown.json 2>&1 | grep 'unknown edge'
```

```run:shell
$ specdown trace -config .tmp-test/trace/unknown-edge/specdown.json 2>&1 | grep -c 'unknown edge'
1
```

### Untyped Source

A trace link from an untyped document is an error.

```run:shell
# Untyped source using trace link
mkdir -p .tmp-test/trace/untyped-src
cat <<'CFG' > .tmp-test/trace/untyped-src/specdown.json
{"entry":"specs/index.spec.md","adapters":[],"trace":{"types":["goal"],"edges":{"covers":{"from":"goal","to":"goal"}}}}
CFG
mkdir -p .tmp-test/trace/untyped-src/specs
printf '# Index\n' > .tmp-test/trace/untyped-src/specs/index.spec.md
printf '# NoType\n\n[covers::G](g.md)\n' > .tmp-test/trace/untyped-src/src.md
printf -- '---\ntype: goal\n---\n# G\n' > .tmp-test/trace/untyped-src/g.md
specdown trace -config .tmp-test/trace/untyped-src/specdown.json 2>&1 | grep -c 'no type'
```

```run:shell
$ specdown trace -config .tmp-test/trace/untyped-src/specdown.json 2>&1 | grep -c 'no type'
1
```

### Type Mismatch

Source document type must match the edge's `from`.

```run:shell
# Source type mismatch
mkdir -p .tmp-test/trace/type-mm
cat <<'CFG' > .tmp-test/trace/type-mm/specdown.json
{"entry":"specs/index.spec.md","adapters":[],"trace":{"types":["goal","test"],"edges":{"tests":{"from":"test","to":"goal"}}}}
CFG
mkdir -p .tmp-test/trace/type-mm/specs
printf '# Index\n' > .tmp-test/trace/type-mm/specs/index.spec.md
printf -- '---\ntype: goal\n---\n# WrongType\n\n[tests::G](g.md)\n' > .tmp-test/trace/type-mm/src.md
printf -- '---\ntype: goal\n---\n# G\n' > .tmp-test/trace/type-mm/g.md
specdown trace -config .tmp-test/trace/type-mm/specdown.json 2>&1 | grep -c 'type mismatch'
```

```run:shell
$ specdown trace -config .tmp-test/trace/type-mm/specdown.json 2>&1 | grep -c 'type mismatch'
1
```

### Dangling Reference

Target must resolve to an existing document.

```run:shell
# Dangling reference
mkdir -p .tmp-test/trace/dangling
cat <<'CFG' > .tmp-test/trace/dangling/specdown.json
{"entry":"specs/index.spec.md","adapters":[],"trace":{"types":["feature","goal"],"edges":{"covers":{"from":"feature","to":"goal"}}}}
CFG
mkdir -p .tmp-test/trace/dangling/specs
printf '# Index\n' > .tmp-test/trace/dangling/specs/index.spec.md
printf -- '---\ntype: feature\n---\n# F\n\n[covers::Missing](missing.md)\n' > .tmp-test/trace/dangling/f.md
specdown trace -config .tmp-test/trace/dangling/specdown.json 2>&1 | grep -c 'dangling reference'
```

```run:shell
$ specdown trace -config .tmp-test/trace/dangling/specdown.json 2>&1 | grep -c 'dangling reference'
1
```

### Self-Loop

A document linking to itself is always forbidden.

```run:shell
# Self-loop detection
mkdir -p .tmp-test/trace/self-loop
cat <<'CFG' > .tmp-test/trace/self-loop/specdown.json
{"entry":"specs/index.spec.md","adapters":[],"trace":{"types":["feature"],"edges":{"requires":{"from":"feature","to":"feature"}}}}
CFG
mkdir -p .tmp-test/trace/self-loop/specs
printf '# Index\n' > .tmp-test/trace/self-loop/specs/index.spec.md
printf -- '---\ntype: feature\n---\n# F\n\n[requires::Self](f.md)\n' > .tmp-test/trace/self-loop/f.md
specdown trace -config .tmp-test/trace/self-loop/specdown.json 2>&1 | grep -c 'self-loop'
```

```run:shell
$ specdown trace -config .tmp-test/trace/self-loop/specdown.json 2>&1 | grep -c 'self-loop'
1
```

### Duplicate Edges

Multiple trace links with the same edge name and target are deduplicated to a
single edge. No warning, no error.

```run:shell
# Duplicate edges are deduplicated silently
mkdir -p .tmp-test/trace/dedup
cat <<'CFG' > .tmp-test/trace/dedup/specdown.json
{"entry":"specs/index.spec.md","adapters":[],"trace":{"types":["feature","goal"],"edges":{"covers":{"from":"feature","to":"goal"}}}}
CFG
mkdir -p .tmp-test/trace/dedup/specs
printf '# Index\n' > .tmp-test/trace/dedup/specs/index.spec.md
printf -- '---\ntype: goal\n---\n# G\n' > .tmp-test/trace/dedup/g.md
printf -- '---\ntype: feature\n---\n# F\n\n[covers::G](g.md)\n[covers::G again](g.md)\n' > .tmp-test/trace/dedup/f.md
```

```run:shell
$ specdown trace -config .tmp-test/trace/dedup/specdown.json 2>&1 | grep -c '"covers"'
1
```

## Graph Constraints

### Cardinality

For each edge type with a `count` constraint, every document of the relevant type
must satisfy both source-side and target-side multiplicity.

```run:shell
# Cardinality violation: feature with no incoming tests
mkdir -p .tmp-test/trace/card
cat <<'CFG' > .tmp-test/trace/card/specdown.json
{"entry":"specs/index.spec.md","adapters":[],"trace":{"types":["feature","test"],"edges":{"tests":{"from":"test","to":"feature","count":"1 → 1..*"}}}}
CFG
mkdir -p .tmp-test/trace/card/specs
printf '# Index\n' > .tmp-test/trace/card/specs/index.spec.md
printf -- '---\ntype: feature\n---\n# F\n' > .tmp-test/trace/card/f.md
```

```run:shell
$ specdown trace -config .tmp-test/trace/card/specdown.json 2>&1 | grep -c 'cardinality'
1
```

### Cycle Detection

Edges with `acyclic: true` reject cycles.

```run:shell
# Cycle detection with acyclic edge
mkdir -p .tmp-test/trace/cycle
cat <<'CFG' > .tmp-test/trace/cycle/specdown.json
{"entry":"specs/index.spec.md","adapters":[],"trace":{"types":["feature"],"edges":{"requires":{"from":"feature","to":"feature","acyclic":true}}}}
CFG
mkdir -p .tmp-test/trace/cycle/specs
printf '# Index\n' > .tmp-test/trace/cycle/specs/index.spec.md
printf -- '---\ntype: feature\n---\n# A\n\n[requires::B](b.md)\n' > .tmp-test/trace/cycle/a.md
printf -- '---\ntype: feature\n---\n# B\n\n[requires::A](a.md)\n' > .tmp-test/trace/cycle/b.md
```

```run:shell
$ specdown trace -config .tmp-test/trace/cycle/specdown.json 2>&1 | grep -c 'cycle detected'
1
```

### Transitive Closure

When `transitive: true`, specdown computes the transitive closure.
If A requires B and B requires C, A transitively requires C.

```run:shell
# Transitive closure adds indirect edges
mkdir -p .tmp-test/trace/trans
cat <<'CFG' > .tmp-test/trace/trans/specdown.json
{"entry":"specs/index.spec.md","adapters":[],"trace":{"types":["feature"],"edges":{"requires":{"from":"feature","to":"feature","transitive":true}}}}
CFG
mkdir -p .tmp-test/trace/trans/specs
printf '# Index\n' > .tmp-test/trace/trans/specs/index.spec.md
printf -- '---\ntype: feature\n---\n# A\n\n[requires::B](b.md)\n' > .tmp-test/trace/trans/a.md
printf -- '---\ntype: feature\n---\n# B\n\n[requires::C](c.md)\n' > .tmp-test/trace/trans/b.md
printf -- '---\ntype: feature\n---\n# C\n' > .tmp-test/trace/trans/c.md
```

```run:shell
$ specdown trace -config .tmp-test/trace/trans/specdown.json 2>&1 | grep -c 'transitiveEdges'
1
```

The transitive edge A→C should appear in the output.

```run:shell
$ specdown trace -config .tmp-test/trace/trans/specdown.json 2>&1 | grep -A1 'transitiveEdges' | grep -c '\['
1
```

## Output Formats

### JSON

`--format=json` outputs the graph as JSON with nodes, direct edges, and transitive edges.

```run:shell
$ specdown trace -config .tmp-test/trace/disc/specdown.json -format=json 2>&1 | head -1
{
```

### DOT

`--format=dot` outputs Graphviz DOT format.

```run:shell
$ specdown trace -config .tmp-test/trace/disc/specdown.json -format=dot 2>&1 | head -1
digraph trace {
```

### Matrix

`--format=matrix` outputs a traceability matrix.

```run:shell
$ specdown trace -config .tmp-test/trace/disc/specdown.json -format=matrix 2>&1 | head -1 | grep -c 'features'
1
```

### Strict Mode

With `--strict`, validation errors suppress output.

```run:shell
# Strict mode exits non-zero on errors
! specdown trace -config .tmp-test/trace/card/specdown.json -strict 2>/dev/null
```

## Opt-in

No `trace` config in `specdown.json` means everything works as before.
The trace feature activates only when the `trace` key is present in config.

```run:shell
# No trace config = trace command reports missing config
mkdir -p .tmp-test/trace/noop
printf '{"entry":"specs/index.spec.md","adapters":[]}' > .tmp-test/trace/noop/specdown.json
mkdir -p .tmp-test/trace/noop/specs
printf '# Index\n' > .tmp-test/trace/noop/specs/index.spec.md
! specdown trace -config .tmp-test/trace/noop/specdown.json 2>/dev/null
```

## Integration with `specdown run`

When trace is configured, `specdown run` performs trace validation before
executing specs. Trace errors are reported alongside spec results.
A trace error does not prevent spec execution.

```run:shell
# specdown run reports trace errors
mkdir -p .tmp-test/trace/run-int
cat <<'CFG' > .tmp-test/trace/run-int/specdown.json
{"entry":"specs/index.spec.md","adapters":[],"reporters":[],"trace":{"types":["feature","test"],"edges":{"tests":{"from":"test","to":"feature","count":"1 → 1..*"}}}}
CFG
mkdir -p .tmp-test/trace/run-int/specs
printf '# Index\n\n- [F](../f.md)\n' > .tmp-test/trace/run-int/specs/index.spec.md
printf -- '---\ntype: feature\n---\n# F\n' > .tmp-test/trace/run-int/f.md
specdown run -config .tmp-test/trace/run-int/specdown.json 2>&1 | grep -c 'trace:'
```

```run:shell
$ specdown run -config .tmp-test/trace/run-int/specdown.json 2>&1 | grep -c 'trace:'
1
```
