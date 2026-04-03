---
type: spec
workdir: .tmp-test
---

# Traceability

Consider a project with four layers of documentation: Themes describe
business goals, Epics break themes into deliverables, User Stories
describe end-user behavior, and Acceptance Tests verify each story.
How do you know every story has at least one test? That every epic
traces back to a theme? That no test is orphaned?

Traceability answers these questions. Documents are nodes; named,
typed links between them are edges. Configure edge types and
cardinality constraints in [depends::specdown.json](config.spec.md),
and specdown checks them automatically.

Edges follow UML dependency direction: `from` is the dependent,
`to` is the dependency. The `from` document contains the trace link.
In the example below, a theme depends on its epics, an epic depends
on its stories, and a story depends on its acceptance tests:

```json
{
  "trace": {
    "types": ["theme", "epic", "story", "at"],
    "edges": {
      "decomposes": { "from": "theme", "to": "epic",  "count": "1 → 1..*" },
      "covers":     { "from": "epic",  "to": "story", "count": "1 → 1..*" },
      "tests":      { "from": "story", "to": "at",    "count": "1 → 1..*" }
    }
  }
}
```

With this configuration, `specdown trace --strict` fails if any theme
lacks epic links, any story lacks acceptance test links, or any document
violates its declared cardinality. Everything else is derived.

## Authoring Surface

### Node Type — Frontmatter

A document's type is declared in YAML frontmatter:

```yaml
---
type: spec
---
```

A type is an identifier (`[a-z][a-z0-9_-]*`). Valid types must be declared in the
`trace.types` config array. A document with a type not listed in config is an error.
Documents without `type` are untyped — they may be navigation link targets but
cannot be trace link sources or targets.

### Trace Edges — Markdown Links with Edge Name Prefix

Trace links use standard markdown link syntax with an `<edge_name>::` prefix in the
link text:

```markdown
This epic [covers::Login Story](../stories/login.md) and
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

When an edge kind connects documents of different types (e.g., `feature → test`),
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

pred sanityCheck {}
run sanityCheck {} for 5
```

## Configuration

The `trace` key in `specdown.json` configures traceability:

```json
{
  "trace": {
    "types": ["goal", "feature", "test"],
    "ignore": ["vendor/**", "third_party/**"],
    "edges": {
      "covers":   { "from": "goal",    "to": "feature", "count": "1..* → 1..*" },
      "tests":    { "from": "feature", "to": "test",    "count": "1 → 1..*" },
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

Source side (left of `→`): how many sources each target document must be linked from.
Target side (right of `→`): how many targets each source document must link to.

Omitted `count` defaults to `0..* → 0..*` (no constraints).

## Discovery

When trace is configured, specdown scans the entire directory tree rooted at
the config file location. All `.md` files are included unless they match an
`ignore` pattern.

```run:shell
# Discovery finds all .md files with types
mkdir -p trace/disc
cat <<'CFG' > trace/disc/specdown.json
{"entry":"specs/index.spec.md","adapters":[],"trace":{"types":["goal","feature"],"edges":{"covers":{"from":"goal","to":"feature"}}}}
CFG
mkdir -p trace/disc/specs trace/disc/goals trace/disc/features
printf '# Index\n' > trace/disc/specs/index.spec.md
printf -- '---\ntype: goal\n---\n# G1\n\n[covers::F1](../features/f1.md)\n' > trace/disc/goals/g1.md
printf -- '---\ntype: feature\n---\n# F1\n' > trace/disc/features/f1.md
specdown trace -config trace/disc/specdown.json 2>&1 | grep -c '"type"'
```

```run:shell
$ specdown trace -config trace/disc/specdown.json 2>&1 | grep -c '"type"'
2
```

## Link Errors

specdown validates every trace link against the configured edge definitions.

| Error | Cause |
|-------|-------|
| Unknown edge name | Link uses an edge name not declared in config |
| Untyped source | Source document has no `type` in frontmatter |
| Untyped target | Target document has no `type` in frontmatter |
| Type mismatch | Source type doesn't match the edge's `from` type |
| Dangling reference | Target file doesn't exist |
| Self-loop | Document links to itself |

Duplicate links (same edge + same target from the same document) are
deduplicated, but each duplicate is reported as a validation error.
This catches copy-paste mistakes and accidental link repetition.

Here is an example — a feature document using an undeclared edge name:

```run:shell
# Unknown edge name is reported
mkdir -p trace/unknown-edge
cat <<'CFG' > trace/unknown-edge/specdown.json
{"entry":"specs/index.spec.md","adapters":[],"trace":{"types":["feature"],"edges":{"covers":{"from":"feature","to":"feature"}}}}
CFG
mkdir -p trace/unknown-edge/specs
printf '# Index\n' > trace/unknown-edge/specs/index.spec.md
printf -- '---\ntype: feature\n---\n# F1\n\n[bogus::something](f2.md)\n' > trace/unknown-edge/f1.md
printf -- '---\ntype: feature\n---\n# F2\n' > trace/unknown-edge/f2.md
```

```run:shell
$ specdown trace -config trace/unknown-edge/specdown.json 2>&1 | grep -c 'unknown edge'
1
```

## Graph Constraints

### Cardinality

For each edge type with a `count` constraint, every document of the relevant type
must satisfy both source-side and target-side multiplicity.

```run:shell
# Cardinality violation: feature with no outgoing test links
mkdir -p trace/card
cat <<'CFG' > trace/card/specdown.json
{"entry":"specs/index.spec.md","adapters":[],"trace":{"types":["feature","test"],"edges":{"tests":{"from":"feature","to":"test","count":"1 → 1..*"}}}}
CFG
mkdir -p trace/card/specs
printf '# Index\n' > trace/card/specs/index.spec.md
printf -- '---\ntype: feature\n---\n# F\n' > trace/card/f.md
```

```run:shell
$ specdown trace -config trace/card/specdown.json 2>&1 | grep -c 'cardinality'
1
```

Cardinality constraints apply to **direct edges only**. Transitive edges
(computed via `transitive: true`) do not count toward cardinality — an
indirect link A → B → C does not satisfy a "must have at least 1 outgoing
edge" constraint on A unless A also has a direct edge.

### Cycle Detection

Edges with `acyclic: true` reject cycles.

```run:shell
# Cycle detection with acyclic edge
mkdir -p trace/cycle
cat <<'CFG' > trace/cycle/specdown.json
{"entry":"specs/index.spec.md","adapters":[],"trace":{"types":["feature"],"edges":{"requires":{"from":"feature","to":"feature","acyclic":true}}}}
CFG
mkdir -p trace/cycle/specs
printf '# Index\n' > trace/cycle/specs/index.spec.md
printf -- '---\ntype: feature\n---\n# A\n\n[requires::B](b.md)\n' > trace/cycle/a.md
printf -- '---\ntype: feature\n---\n# B\n\n[requires::A](a.md)\n' > trace/cycle/b.md
```

```run:shell
$ specdown trace -config trace/cycle/specdown.json 2>&1 | grep -c 'cycle detected'
1
```

### Transitive Closure

When `transitive: true`, specdown computes the transitive closure.
If A requires B and B requires C, A transitively requires C.
Transitive edges appear in the report graph but do not affect cardinality
checks — only direct edges satisfy `count` constraints.

```run:shell
# Transitive closure adds indirect edges
mkdir -p trace/trans
cat <<'CFG' > trace/trans/specdown.json
{"entry":"specs/index.spec.md","adapters":[],"trace":{"types":["feature"],"edges":{"requires":{"from":"feature","to":"feature","transitive":true}}}}
CFG
mkdir -p trace/trans/specs
printf '# Index\n' > trace/trans/specs/index.spec.md
printf -- '---\ntype: feature\n---\n# A\n\n[requires::B](b.md)\n' > trace/trans/a.md
printf -- '---\ntype: feature\n---\n# B\n\n[requires::C](c.md)\n' > trace/trans/b.md
printf -- '---\ntype: feature\n---\n# C\n' > trace/trans/c.md
```

```run:shell
$ specdown trace -config trace/trans/specdown.json 2>&1 | grep -c 'transitiveEdges'
1
```

## Output Formats

| Flag | Format | Description |
|------|--------|-------------|
| `-format=json` | JSON | Nodes, direct edges, transitive edges |
| `-format=dot` | Graphviz DOT | For visualization with `dot` or similar tools |
| `-format=matrix` | Traceability matrix | Tabular summary of coverage |

```run:shell
$ specdown trace -config trace/disc/specdown.json -format=json 2>&1 | head -1
{
```

```run:shell
$ specdown trace -config trace/disc/specdown.json -format=dot 2>&1 | head -1
digraph trace {
```

### Strict Mode

With `--strict`, validation errors cause a non-zero exit code.

```run:shell
# Strict mode exits non-zero on errors
! specdown trace -config trace/card/specdown.json -strict 2>/dev/null
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
mkdir -p trace/run-int
cat <<'CFG' > trace/run-int/specdown.json
{"entry":"specs/index.spec.md","adapters":[],"reporters":[],"trace":{"types":["feature","test"],"edges":{"tests":{"from":"feature","to":"test","count":"1 → 1..*"}}}}
CFG
mkdir -p trace/run-int/specs
printf '# Index\n\n- [F](../f.md)\n' > trace/run-int/specs/index.spec.md
printf -- '---\ntype: feature\n---\n# F\n' > trace/run-int/f.md
```

```run:shell
$ specdown run -config trace/run-int/specdown.json 2>&1 | grep -c 'trace:'
1
```
