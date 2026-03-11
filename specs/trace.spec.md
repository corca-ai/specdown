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

### Structural Guarantee

When an edge kind connects documents of different types (e.g., `test → feature`),
the type system alone prevents reverse cycles within that edge kind.
This means hierarchical traceability (theme → epic → story → test) is
inherently acyclic without needing the `acyclic` flag.

```alloy:model(tracegraph)
module tracegraph

sig Type {}
sig Doc { dtype: one Type }

sig EdgeKind {
  fromType: one Type,
  toType: one Type
}

sig Link {
  kind: one EdgeKind,
  src: one Doc,
  tgt: one Doc
}

-- links are well-typed
fact wellTyped {
  all l: Link |
    l.src.dtype = l.kind.fromType and
    l.tgt.dtype = l.kind.toType
}

-- self-loops are forbidden
fact noSelfLoop {
  no l: Link | l.src = l.tgt
}

-- cross-type edges cannot form reverse pairs
assert crossTypeNoReverse {
  all ek: EdgeKind | ek.fromType != ek.toType implies
    no disj l1, l2: Link | l1.kind = ek and l2.kind = ek and
      l1.src = l2.tgt and l1.tgt = l2.src
}

check crossTypeNoReverse for 5
```

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

Type names, edge names, and `from`/`to` values must all be valid identifiers
declared in `trace.types`. Invalid config is rejected before scanning begins.

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

specdown validates every trace link against the configured edge definitions.

| Error | Cause |
|-------|-------|
| Unknown edge name | Link uses an edge name not declared in config |
| Untyped source | Source document has no `type` in frontmatter |
| Type mismatch | Source type doesn't match the edge's `from` type |
| Dangling reference | Target file doesn't exist |
| Self-loop | Document links to itself |

Duplicate links (same edge + same target) are silently deduplicated.

Here is an example — a feature document using an undeclared edge name:

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
```

```run:shell
$ specdown trace -config .tmp-test/trace/unknown-edge/specdown.json 2>&1 | grep -c 'unknown edge'
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

## Output Formats

| Flag | Format | Description |
|------|--------|-------------|
| `-format=json` | JSON | Nodes, direct edges, transitive edges |
| `-format=dot` | Graphviz DOT | For visualization with `dot` or similar tools |
| `-format=matrix` | Traceability matrix | Tabular summary of coverage |

```run:shell
$ specdown trace -config .tmp-test/trace/disc/specdown.json -format=json 2>&1 | head -1
{
```

```run:shell
$ specdown trace -config .tmp-test/trace/disc/specdown.json -format=dot 2>&1 | head -1
digraph trace {
```

### Strict Mode

With `--strict`, validation errors cause a non-zero exit code.

```run:shell
# Strict mode exits non-zero on errors
! specdown trace -config .tmp-test/trace/card/specdown.json -strict 2>/dev/null
```

## Opt-in

No `trace` config in `specdown.json` means everything works as before.
The trace feature activates only when the `trace` key is present in config.

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
```

```run:shell
$ specdown run -config .tmp-test/trace/run-int/specdown.json 2>&1 | grep -c 'trace:'
1
```
