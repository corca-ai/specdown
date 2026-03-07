# Writing Specs

Spec files are `*.spec.md` Markdown documents.
Prose is preserved as-is; only specific blocks and tables are executed.

## Frontmatter

An optional YAML frontmatter can be added at the top of a spec file.

```markdown
---
timeout: 5000
---

# Product Name
```

| Key | Description |
|-----|-------------|
| `timeout` | Per-case execution time limit in milliseconds. 0 means unlimited |

Frontmatter is optional. If absent, defaults (unlimited) apply.

## Heading Structure

Heading hierarchy becomes the test suite hierarchy.

```markdown
# Product Name          ← top-level suite
## Feature A            ← child suite
### Scenario 1          ← individual scenario
```

## Executable Block

Executable blocks are indicated by the info string of a fenced code block.

````markdown
```run:board -> $boardName
create-board
```
````

| Prefix | Meaning |
|--------|---------|
| `run:<target>` | side-effecting executable block |
| `verify:<target>` | assertion block |

`<target>` is defined by the adapter. Results can be captured with `-> $variableName`.

## Variables

Values captured from blocks are referenced in subsequent blocks and tables using `${variableName}`.

````markdown
```run:api -> $userId
POST /users {"name": "alice"}
```

```verify:api
GET /users/${userId}
```
````

Rules:
- Variables from parent sections are readable in child sections
- Sibling sections at the same depth can share variables (in document order, only previously captured values)
- Unresolved variables cause an error

### Escaping

To output a literal `${...}`, escape it with a backslash.

````markdown
```verify:api
header should contain \${literal}
```
````

`\${literal}` is passed as-is as `${literal}` without variable substitution.

## Fixture Table

Specify a fixture with an HTML comment, then connect the Markdown table immediately below.

```markdown
<!-- fixture:board-exists -->
| board        | exists |
|--------------|--------|
| ${boardName} | yes    |
```

- The first row is the header
- Each row is an independent test case
- Fixture names are defined by the adapter

### Fixture Parameters

Fixtures can accept parameters via `(key=value)` syntax.

```markdown
<!-- fixture:editor-op(type=lexical) -->
| initial    | op          | expected     |
|------------|-------------|--------------|
| hello‸     | press:Enter | hello\n‸     |
```

Parameters are passed to the adapter as `fixtureParams` in the `runCase` message. This avoids creating separate fixtures for each parameter combination.

Multiple parameters are comma-separated: `<!-- fixture:check(user=alan, role=admin) -->`.

### Cell Escaping

Table cells support escape sequences.

| Sequence | Meaning |
|----------|---------|
| `\n` | newline |
| `\|` | literal pipe |
| `\\` | literal backslash |

Escape processing is performed by specdown before cells are sent to the adapter. Adapters always receive unescaped values. The HTML report also unescapes cells, rendering `\n` as visible line breaks.

```markdown
<!-- fixture:check -->
| input    | expected       |
|----------|----------------|
| a\|b     | a\|b           |
| line1\nline2 | line1\nline2 |
```

## Alloy Model

Alloy fragments can be included directly in the document.

````markdown
```alloy:model(board)
module board

sig Card { column: one Column }
```
````

Fragments with the same name are combined in document order.
The `module` declaration is used only in the first fragment.

To link an assertion check result to the document:

```markdown
<!-- alloy:ref(board#cardHasExactlyOneColumn, scope=5) -->
```
