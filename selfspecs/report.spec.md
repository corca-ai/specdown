# HTML Report

The HTML report is a core deliverable.
The goal is to show an "executed specification," not a "test log."

After a spec run, the HTML report is generated as a self-contained file.
It preserves the document structure, annotating execution results inline.

- Prose is displayed as-is; only execution results are annotated with status
- Status indicators appear at section, code block, table row, and alloy reference levels
- Pass is shown with green, fail with red
- Failed items display message, expected/actual diff inline
- A summary shows pass/fail counts

```run:shell
# Generate a report with a passing doctest
mkdir -p .tmp-test
BT=$(printf '\140\140\140')
printf '%s\n' '# Summary Test' '' "\${BT}run:shell" '$ echo ok' 'ok' "\${BT}" > .tmp-test/summary-test.spec.md
printf '# T\n\n- [Summary](summary-test.spec.md)\n' > .tmp-test/index.spec.md
printf '{"entry":"index.spec.md","adapters":[],"reporters":[{"builtin":"html","outFile":"summary-report.html"}]}' > .tmp-test/summary-cfg.json
specdown run -config .tmp-test/summary-cfg.json 2>&1 || true
```

The HTML report contains a pass/fail summary.

```run:shell
$ grep -q 'class="pill pass"' .tmp-test/summary-report.html && echo found
found
$ grep -q 'passed' .tmp-test/summary-report.html && echo found
found
```

## UX Principles

- The body and key failure information must be readable without JavaScript
- Anchor links allow jumping directly to original headings
- Failed rows and failed blocks support fold/unfold
- Prose and results from the same document are not separated

```run:shell
# Generate a minimal report for UX verification
mkdir -p .tmp-test
cat <<'SPEC' > .tmp-test/report-test.spec.md
# Report Test

A minimal spec for testing report generation.
SPEC
printf '# Report Test\n\n- [Report](report-test.spec.md)\n' > .tmp-test/index.spec.md
cat <<'CFG' > .tmp-test/report-test.json
{"entry":"index.spec.md","adapters":[]}
CFG
specdown run -config .tmp-test/report-test.json -out report.html 2>&1 || true
```

The report file must exist and contain standard HTML structure.

```run:shell
$ test -f .tmp-test/report.html && echo yes
yes
$ grep -q '<html' .tmp-test/report.html && echo found
found
$ grep -q 'Report Test' .tmp-test/report.html && echo found
found
```

The report must be readable without JavaScript (no script-gated content).

```run:shell
# Verify headings appear outside script-gated blocks
grep -q '<h1' .tmp-test/report.html
```

The report must include anchor links for sections.

```run:shell
# Verify section anchor IDs exist
grep -q 'id="section-' .tmp-test/report.html
```

### Intent Captions

Blocks whose first line is a comment render collapsed in the report.
The caption text appears in an `exec-caption-text` span inside a
`<summary>` element, with an expand marker on the right.
The containing section gets the `has-caption` class.

## Output Files

| File | Description |
|------|-------------|
| `.artifacts/specdown/report.html` | Executed specification HTML report |
| `.artifacts/specdown/report.json` | Machine-readable results |
| `.artifacts/specdown/models/*.als` | Combined Alloy models |
| `.artifacts/specdown/counterexamples/*` | Counterexample artifacts (on Alloy check failure) |

## Failure Diagnostics

When an adapter returns `expected`/`actual` values on failure,
those values appear in both the CLI output and the JSON report.

The CLI output format for check table failures includes the row number
and optional label:

```
  FAIL  Heading > Path  [check-name] row 5 "description"
        error message
        expected: expected-value
        actual:   actual-value
```

```run:shell
# Create a failing adapter that returns expected/actual/label
mkdir -p .tmp-test
cat <<'ADAPTER' > .tmp-test/diag-adapter.sh
#!/bin/sh
while IFS= read -r line; do
  type=$(printf '%s' "$line" | grep -o '"type":"[^"]*"' | head -1 | cut -d'"' -f4)
  id=$(printf '%s' "$line" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
  case "$type" in
    assert)
      printf '{"id":%s,"type":"failed","message":"content mismatch","expected":"alpha\\nbeta","actual":"alpha\\ngamma","label":"diag row"}\n' "$id"
      ;;
  esac
done
ADAPTER
chmod +x .tmp-test/diag-adapter.sh
```

```run:shell
# Run a spec against the failing adapter
cat <<'SPEC' > .tmp-test/diag.spec.md
# Diag Test

> check:diag
| input | output |
| --- | --- |
| a | b |
SPEC
printf '# T\n\n- [Diag](diag.spec.md)\n' > .tmp-test/index.spec.md
cat <<'CFG' > .tmp-test/diag.json
{"entry":"index.spec.md","adapters":[{"name":"d","command":["sh","./diag-adapter.sh"],"blocks":[],"checks":["diag"]}],"reporters":[{"builtin":"html","outFile":"diag-report.html"},{"builtin":"json","outFile":"report.json"}]}
CFG
specdown run -config .tmp-test/diag.json 2>&1 || true
```

The HTML report displays expected/actual values inline for diffing.

```run:shell
$ grep -q 'failure-diff' .tmp-test/diag-report.html && echo found
found
```

The JSON report must contain the expected and actual fields, and the adapter label.

```run:shell
$ grep -q '"expected"' .tmp-test/report.json && echo found
found
$ grep -q '"actual"' .tmp-test/report.json && echo found
found
$ grep -q '"diag row"' .tmp-test/report.json && echo found
found
```
