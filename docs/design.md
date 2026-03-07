# Executable Specification (`specdown`)

Design document for a project-independent executable specification system.
The core design is not tied to any specific product.


## Document Status

- Target audience: independent development teams
- Purpose: finalize requirements and boundaries to the point where a reusable core + adapter-based product can be implemented
- Deliverables: `specdown-core`, `specdown-cli`, `specdown-adapter-protocol`, `specdown-reporter-html`, `specdown-alloy`, reference adapters


## Problem

When design documents and test code are separated, the following problems recur.

- It is difficult to track whether properties stated in design documents are actually verified
- Tests verify behavior but do not sufficiently explain design intent and rationale
- Security properties and state-space properties are hard to cover with example-based tests alone
- As documents, tests, and formal models evolve separately, consistency depends on human memory


## Goals

`specdown` has four goals.

1. A single Markdown document serves as both a readable specification and an executable test.
2. Alloy models are woven into the same document in literate style, connecting to formal verification.
3. Table-based specifications (FIT style) are provided as a first-class feature.
4. Execution results are rendered as HTML, turning the document itself into an execution report with green/red status.


## Non-Goals

The following are excluded from the v1 scope.

- Replacing all tests with a Markdown DSL
- Automatically proving that the implementation is fully equivalent to the formal model
- Embedding Playwright, Vitest, Jest, or Bun test into the core
- Including project-specific DSLs such as DOM selectors, shell transcripts, or editor actions in the core
- Fully automating multi-repository/multi-package model imports


## Key Decisions

This document finalizes the following.

1. The document format is Markdown-first. Prose is preserved; only executable blocks are structurally interpreted.
2. The core knows nothing about test frameworks or product logic. All execution semantics are provided by adapters.
3. Alloy is supported via document-embedded blocks.
4. When the same `alloy:model(name)` appears multiple times, fragments are combined in document order into a single logical model.
5. The HTML report is a first-class deliverable, not an add-on.
6. Table-based specifications are core grammar; the execution semantics of each table are defined by a fixture adapter.
7. The adapter extension boundary defaults to a language-neutral process protocol.


## Product Layout

The recommended package layout is as follows.

```text
packages/
├── specdown-core/              # parser, AST, planning, event model
├── specdown-cli/               # run entry point
├── specdown-adapter-protocol/  # adapter process contract + JSON schema
├── specdown-reporter-html/     # static HTML report renderer
├── specdown-alloy/             # embedded model extraction + Alloy runner
├── specdown-adapter-shell/     # optional high-reuse builtin adapter
└── specdown-adapter-vitest/    # optional convenience adapter
```

`specdown-core` must not depend on `vitest`, `playwright`, `svelte`, or DOM selectors.


## Architecture

Two pipelines diverge from a single document.

```text
Spec Document (.spec.md)
    │
    ├── Spec Core
    │     ├── heading / prose / block / table parsing
    │     ├── variable scope computation
    │     ├── executable unit ID assignment
    │     └── embedded Alloy model extraction
    │
    ├── Runtime Adapter
    │     └── test execution + Spec Event emission
    │
    ├── Reporter Adapter
    │     └── HTML / JSON / CI artifact generation
    │
    └── Alloy Runner
          └── model check + Spec Event emission
```

The core principle is simple.

- Core structures the document and creates the execution plan
- Runtime adapter turns each block/table into an actual test or command
- Reporter adapter turns execution events into human-readable output
- Alloy runner feeds formal verification results into the same event model
- Adapters connect as out-of-process commands by default


## Core and Adapter Boundary

### Core Responsibilities

- Markdown parsing
- Converting heading hierarchy into suite hierarchy
- Extracting code blocks, directives, and tables
- Variable binding and scope computation
- `SpecId` generation
- Combining embedded Alloy fragments
- Generating a runtime-independent execution plan
- Defining the common event schema

### Adapter Responsibilities

- Interpreting block semantics such as `run:*`, `verify:*`, `test:*`
- Interpreting column semantics of fixture tables
- Connecting to external execution environments like shell, browser, API, editor, or sandbox
- Test framework integration
- Rendering results as HTML/JSON/JUnit
- Communicating with core via process protocol

### What Core Must Not Know

- Vitest's `describe/test` API
- Playwright page objects
- Product-specific filesystem layouts
- Product-specific command vocabularies
- Adapter implementation language and runtime


## Common Protocol

The adapter boundary must be a process protocol, not an in-process language API.
This allows each project to build adapters with minimal effort in any language: Go, Python, Rust, Node, Ruby, etc.

The default transport is fixed as follows.

- An adapter is an executable command
- `specdown-cli` launches the adapter process
- Messages are exchanged via stdin/stdout as NDJSON
- Only protocol messages are written to stdout
- A single adapter process can handle multiple `runCase` requests sequentially during one spec run
- `specdown-cli` sends `setup` before the first `runCase` and `teardown` after the last `runCase`
- Adapters may ignore `setup`/`teardown` (no response required)
- A non-zero exit indicates an adapter crash or infrastructure failure, not a case failure
- Capabilities (blocks/fixtures) are declared in config, not negotiated at runtime

Transmitted payloads must be JSON-serializable shapes.

```ts
export type Binding = {
  name: string;
  value: string;
};

export type AdapterRequest =
  | { type: "setup" }
  | { type: "teardown" }
  | {
      type: "runCase";
      id: number;
      case: {
        kind: "code" | "tableRow";
        block?: string;
        source?: string;
        fixture?: string;
        fixtureParams?: Record<string, string>;
        columns?: string[];
        cells?: string[];
        captureNames?: string[];
        bindings?: Binding[];
      };
    };

export type AdapterResponse =
  | { id: number; type: "passed"; bindings?: Binding[] }
  | { id: number; type: "failed"; message: string; expected?: string; actual?: string; label?: string };
```

Key rules:

- Core fixes only `CodeBlockNode`, `TableNode`, `SpecId`, and event schema
- An adapter declares the block info and fixture names it supports in `specdown.json`
- `specdown-cli` decides which adapter handles which case based on that declaration
- During execution, `runCase` requests are sent in document order with integer correlation IDs
- An adapter echoes the `id` back in every response
- An adapter can maintain process-local state and return values to core via `passed.bindings`
- On adapter failure, provide a `message` describing what went wrong
- Built-in adapters must follow the same protocol contract
- Language-specific helper SDKs are optional conveniences, not architectural essentials


## Configuration

Implementation teams adopt a data-only configuration file as the default.
The canonical config must not depend on any specific language runtime.

Example:

```json
{
  "entry": "specs/index.spec.md",
  "adapters": [
    {
      "name": "project",
      "command": ["python3", "./tools/specdown_adapter.py"],
      "blocks": ["run:myapp", "verify:myapp"],
      "fixtures": ["user-exists"]
    }
  ],
  "reporters": [
    {
      "builtin": "html",
      "outFile": ".artifacts/specdown/report.html"
    },
    {
      "builtin": "json",
      "outFile": ".artifacts/specdown/report.json"
    }
  ],
  "models": {
    "builtin": "alloy"
  }
}
```

Language-specific helpers may generate this file, but the canonical format must be data-only.
For v1, a single `specdown.json` is sufficient.


## Document Grammar

### Frontmatter

An optional YAML frontmatter can be placed at the top of a spec file.

```markdown
---
timeout: 5000
---
```

| Key | Meaning |
|-----|---------|
| `timeout` | Per-case execution time limit in milliseconds. 0 means unlimited |

If frontmatter is absent, defaults (unlimited) apply.

### Structure Mapping

Heading hierarchy is converted into a test suite hierarchy.

| Markdown | Meaning |
|----------|---------|
| `#`, `##`, `###` | suite hierarchy |
| plain prose | document body, not an execution target |
| fenced code block | executable block or model block |
| HTML comment directive | meta directives such as setup, teardown, fixture, alloy reference |
| Markdown table | execution data when combined with a fixture directive |

### Supported Blocks

The core knows only the following rules.

| Notation | Core meaning | Execution semantics provider |
|----------|-------------|------------------------------|
| `run:<target>` | side-effecting executable block | block adapter |
| `verify:<target>` | assertion-bearing executable block | block adapter |
| `expect` | assertion block | block adapter or core helper |
| `test:<name>` | named high-level test DSL | block adapter |
| `alloy:model(name)` | embedded Alloy fragment | core + Alloy runner |

The actual meaning of `<target>` and `<name>` is determined by the adapter, not the core.

### Variables

Variable bindings are supported to connect dynamic values within a document.

Example:

````markdown
```run:shell -> $channelId
create-channel random
```

```expect
${channelId} matches /^ch-/
```
````

Rules:

- Variables from parent sections are readable in child sections
- Sibling sections at the same depth can share variables (in document order, only previously captured values)
- An unresolved variable is a compile-time error
- Escaping with `\${...}` passes a literal `${...}`

### Setup / Teardown

```markdown
<!-- setup -->
<!-- teardown -->
<!-- setup:each -->
<!-- teardown:each -->
```

These directives must be followed by an executable code block that serves as the hook body.
They apply to the current heading subtree.

- `setup` / `teardown`: run once before the first case and after the last case in the scope.
- `setup:each` / `teardown:each`: run before the first case and after the last case of each immediate child section.

Example:

````markdown
## Scenario Group

<!-- setup:each -->
```run:api
login u0
```

<!-- teardown:each -->
```run:api
reset u0
```

### Scenario A

```run:api
do-action-a
```

### Scenario B

```run:api
do-action-b
```
````

In this example, `login u0` runs before Scenario A and before Scenario B.
`reset u0` runs after Scenario A and after Scenario B.


## Table-Based Specifications

The essence of the FIT style is preserved, but the core generalizes via a fixture adapter structure.

Example:

````markdown
<!-- fixture:write-permission(user=alan) -->
| path                       | write | reason                |
|----------------------------|-------|-----------------------|
| /private/test.txt          | yes   | personal workspace    |
| /MEMORY.md                 | yes   | persists across runs  |
| /channels/general/chat.log | no    | channels are post-only|
````

Rules:

- A table is executable only when combined with a `fixture` directive immediately above it
- The fixture directive supports optional parameters: `<!-- fixture:name(key=value, key2=value2) -->`
- Parameters are passed to the adapter as `fixtureParams` in the `runCase` message
- The first row must be a header
- Each fixture adapter must explicitly validate the required columns
- An unknown fixture is a compile-time error
- Each row becomes an independent test case and an independent report row

### Parameterized Fixture Call (Tableless Fixture)

A fixture directive with parameters can be used without a following table.
This creates a single assertion case using only `fixtureParams`, with empty `columns` and `cells`.

```markdown
<!-- fixture:check-user(field=plan, expected=STANDARD) -->
```

This is useful for inline assertions that don't warrant a full table:

```markdown
The plan changes to STANDARD after subscription.
<!-- fixture:check-user(field=plan, expected=STANDARD) -->

The next billing date is set.
<!-- fixture:check-user(field=nextBillingDate, expected=2025-03-03) -->
```

A fixture directive without parameters and without a table is a compile-time error.

### Cell Escaping

Table cells support the following escape sequences.

| Sequence | Meaning |
|----------|---------|
| `\n` | newline |
| `\|` | literal pipe |
| `\\` | literal backslash |

Escape processing is performed by the core before cells are sent to adapters. Adapters always receive unescaped values and do not need to implement their own escape handling. The HTML reporter also unescapes cells so that `\n` renders as visible line breaks in the report.

The fixture adapter contract must satisfy the following requirements.

- It must be able to expand an input table into a per-row execution plan
- On failure, it must report which row failed and why, together with a `SpecId`
- When possible, it should provide a structured expected/actual diff


## Literate Alloy

It is important that Alloy is woven with natural language inside the document.
For v1, the following rules are fixed.

### Embedding Rules

`alloy:model(name)` is a fragment belonging to the logical model `name`.
Fragments with the same `name` are combined in document order.

Example:

````markdown
Explain the concept in prose.

```alloy:model(access)
module access

sig Node {}
sig Path {}
```

Explain the rationale for the private rule.

```alloy:model(access)
sig PrivatePath in Path {}

assert privateIsolation {
  all p: PrivatePath | ...
}

check privateIsolation for 5
```
````

### Combination Rules

- Fragments with the same model name are merged into a single virtual `.als` file
- Only the first fragment may contain a `module` declaration
- A `module` declaration in a subsequent fragment is a compile-time error
- Source mapping comments are inserted into the generated model
  - e.g., `-- specdown-source: docs/foo.spec.md#Access/Isolation`

### Model Reference

An explicit directive is provided so document readers can easily see which assertions have been verified.

```markdown
<!-- alloy:ref(access#privateIsolation, scope=5) -->
```

This directive serves the following purposes.

- Links the current paragraph/section to a specific model check result
- Displayed as a badge or status row in the HTML report
- Links to a corresponding counterexample artifact on failure
- On failure, the `Message` field includes a counterexample summary extracted from the Alloy solver output (e.g., atom bindings such as `Card$0 = {Card$0}`)

Natural-language blockquotes may be used freely, but the machine-readable contract is based on the above directive.


## Execution Result HTML View

The HTML report is a core deliverable of v1.
The goal is to show an "executed specification," not a "test log."

### Basic Requirements

- Document layout preserving the heading structure
- Prose displayed as-is; only execution results annotated with status
- Status indicators at section, code block, table row, and alloy reference levels
- Pass shown with green background or badge
- Fail shown with red background or badge
- Failed items display expected value/actual value/error message/stack trace/stdout/stderr inline or in an expandable panel
- Summary pane provided
  - Total execution count
  - Pass/fail counts
  - Failure list
  - Model check results

### Artifact Requirements

Minimum deliverables:

- `.artifacts/specdown/report.html`
- `.artifacts/specdown/report.json`
- `.artifacts/specdown/models/*.als`
- `.artifacts/specdown/counterexamples/*` (on failure)

### UX Principles

- The body and key failure information must be readable without JavaScript
- Anchor links must allow jumping directly to original headings
- Failed rows and failed blocks must support fold/unfold
- Prose and results from the same document must not be separated


## CLI

The CLI surface for an independent team to implement is roughly as follows.

```bash
specdown run                          # default execution
specdown run -filter "board" -jobs 4  # filtering + parallel execution
specdown run -dry-run                 # parse and validate only
specdown version                      # print version
specdown alloy dump                   # generate only Alloy model .als files
```

Meaning:

- `run`: performs Markdown parsing, adapter execution, embedded Alloy checks, model bundle generation, and report artifact generation in one pass
- `version`: prints the version string injected at build time
- `alloy dump`: generates only Alloy model files without running adapters

In v1, `specdown run` is the default path that performs compile + execute + report in one pass.

On failure, it prints each failed item's heading path, block/fixture name, and error message to stderr, followed by a summary.


## Implementation Phases

The implementation order for handoff to an independent team is fixed as follows.

### Phase 1: Fix Core Grammar and Serializable Event Schema

Goals:

- Fix parser, AST, `SpecId`, execution plan, and event schema
- Fix the JSON-serializable node shape to be passed to adapters

Deliverables:

- `specdown-core`
- Execution plan / event schema documentation
- Compile-time error rules documentation

### Phase 2: Fix Adapter Protocol and Host

Goals:

- Fix the adapter boundary early so project-specific extensions are possible without core changes
- Enable each project to build minimal-effort adapters in any language

Deliverables:

- `specdown-adapter-protocol`
- `specdown-cli` adapter launcher
- `specdown.json` loader
- stdin/stdout NDJSON protocol documentation
- Two minimal reference adapters written in two different languages

### Phase 3: HTML Reporter

Goals:

- Generate a document-centric HTML report from the event stream alone

Deliverables:

- `specdown-reporter-html`
- `report.json` schema
- Anchorable static HTML artifact

### Phase 4: Optional Built-in Generic Adapters

Goals:

- Provide only highly reusable adapters as builtin packages
- Maintain architecture viability without any builtin adapter

Deliverables:

- One or two optional adapters such as `specdown-adapter-shell`
- Optional helper SDK or adapter template

### Phase 5: Table Fixtures

Goals:

- Generalize FIT-style table specifications via fixture adapters

Deliverables:

- Fixture adapter protocol extension
- Two or three sample fixtures
- Row-level reporting

### Phase 6: Alloy Support

Goals:

- Literate Alloy fragment extraction, bundle generation, and model check integration

Deliverables:

- `specdown-alloy`
- Embedded model source mapping
- Counterexample artifact wiring

### Phase 7: Reference Product Adapter

Goals:

- Provide an integration example by separating a specific product's DSL into an adapter

Deliverables:

- One reference adapter package
- Migration of one to three reference specs


## Acceptance Criteria

The completion criteria for independent team handoff are as follows.

1. `specdown-core` does not depend on any specific test framework or product code.
2. A single `.spec.md` document can contain prose, executable blocks, fixture tables, and Alloy fragments together.
3. `alloy:model(...)` fragments with the same model name are combined in document order.
4. The HTML report shows status for sections, blocks, rows, and alloy checks individually.
5. Failure details are displayed without losing document context.
6. A project can execute documents by registering just one adapter command in a data-only config.
7. An adapter can be implemented in any language as long as it follows the stdin/stdout protocol.
8. Product-specific DSLs and helpers exist only in adapters, not in core.


## Examples

### Document Example

````markdown
## Write permissions

Nodes follow the principle of least privilege.

<!-- alloy:ref(access#writeMinimality, scope=5) -->

<!-- fixture:write-permission(user=alan) -->
| path                       | write | reason                |
|----------------------------|-------|-----------------------|
| /private/test.txt          | yes   | personal workspace    |
| /MEMORY.md                 | yes   | persists across runs  |
| /channels/general/chat.log | no    | channels are post-only|
````

### Alloy Weaving Example

````markdown
## Private Isolation

Private paths must be readable only by their owner.

```alloy:model(access)
module access

sig Node {}
sig Path { owner: one Node }
sig PrivatePath in Path {}
```

Introduce a "can read" relation from the above concepts.

```alloy:model(access)
pred canRead[n: Node, p: Path] {
  p not in PrivatePath or p.owner = n
}

assert privateIsolation {
  all n1, n2: Node |
    n1 != n2 implies
      all p: PrivatePath | p.owner = n1 implies not canRead[n2, p]
}

check privateIsolation for 5
```

<!-- alloy:ref(access#privateIsolation, scope=5) -->
````


## Evaluation

| Criterion | Traditional approach | `specdown` |
|-----------|---------------------|------------|
| Document readability | Design and tests are separated | Single literate spec document |
| Formal verification | No separate model, or manual | Connected via Alloy |
| Test addition cost | Requires code | Add a table row or a block |
| Result visibility | Test-log centric | HTML-document centric |
| Product independence | Re-implement per product | Core + adapter architecture |
| Reusability | Low | Runtime/reporter/DSL are swappable |


## Conclusion

`specdown` is a system that provides "Markdown-based literate specification + embedded Alloy + FIT-style table specifications + HTML execution report" as a reusable core.

Independent teams should be able to start implementation immediately based on this document, maintaining clear boundaries between core and adapters.
