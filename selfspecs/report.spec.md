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
mkdir -p .tmp-test
cat <<'SPEC' > .tmp-test/summary-test.spec.md
# Summary Test

```doctest:shell
$ echo ok
ok
```
SPEC
printf '# T\n\n- [Summary](summary-test.spec.md)\n' > .tmp-test/index.spec.md
cat <<'CFG' > .tmp-test/summary-cfg.json
{"entry":"index.spec.md","adapters":[{"name":"s","command":["./specdown-adapter-shell"],"blocks":["doctest:shell"]}],"reporters":[{"builtin":"html","outFile":"summary-report.html"}]}
CFG
specdown run -config .tmp-test/summary-cfg.json 2>&1 || true
```

The HTML report contains a pass/fail summary.

```doctest:shell
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

```doctest:shell
$ test -f .tmp-test/report.html && echo yes
yes
$ grep -q '<html' .tmp-test/report.html && echo found
found
$ grep -q 'Report Test' .tmp-test/report.html && echo found
found
```

The report must be readable without JavaScript (no script-gated content).

```verify:shell
# The report should not require JS for basic content visibility
# Check that headings and prose appear outside of <script> or noscript-hidden blocks
grep -q '<h1' .tmp-test/report.html
```

The report must include anchor links for sections.

```verify:shell
grep -q 'id="section-' .tmp-test/report.html
```

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

The CLI output format for fixture table failures includes the row number
and optional label:

```
  FAIL  Heading > Path  [fixture-name] row 5 "description"
        error message
        expected: expected-value
        actual:   actual-value
```

```run:shell
mkdir -p .tmp-test
cat <<'ADAPTER' > .tmp-test/diag-adapter.sh
#!/bin/sh
# A minimal adapter that fails with expected/actual/label
while IFS= read -r line; do
  type=$(printf '%s' "$line" | grep -o '"type":"[^"]*"' | head -1 | cut -d'"' -f4)
  case "$type" in
    setup|teardown) ;;
    runCase)
      printf '%s\n' '{"id":1,"type":"failed","message":"content mismatch","expected":"alpha\\nbeta","actual":"alpha\\ngamma","label":"diag row"}'
      ;;
  esac
done
ADAPTER
chmod +x .tmp-test/diag-adapter.sh
cat <<'SPEC' > .tmp-test/diag.spec.md
# Diag Test

> fixture:diag
| input | output |
| --- | --- |
| a | b |
SPEC
printf '# T\n\n- [Diag](diag.spec.md)\n' > .tmp-test/index.spec.md
cat <<'CFG' > .tmp-test/diag.json
{"entry":"index.spec.md","adapters":[{"name":"d","command":["sh","./diag-adapter.sh"],"blocks":[],"fixtures":["diag"]}],"reporters":[{"builtin":"html","outFile":"diag-report.html"},{"builtin":"json","outFile":"report.json"}]}
CFG
specdown run -config .tmp-test/diag.json 2>&1 || true
```

The HTML report displays expected/actual values inline for diffing.

```doctest:shell
$ grep -q 'failure-diff' .tmp-test/diag-report.html && echo found
found
```

The JSON report must contain the expected and actual fields, and the adapter label.

```doctest:shell
$ grep -q '"expected"' .tmp-test/report.json && echo found
found
$ grep -q '"actual"' .tmp-test/report.json && echo found
found
$ grep -q '"diag row"' .tmp-test/report.json && echo found
found
```
