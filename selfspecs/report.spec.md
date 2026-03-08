# HTML Report

After a spec run, the HTML report is generated as a self-contained file.
It preserves the document structure, annotating execution results inline.

- Prose is displayed as-is; only execution results are annotated with status
- Status indicators appear at section, code block, table row, and alloy reference levels
- Pass is shown with green, fail with red
- Failed items display message, expected/actual diff inline
- A summary shows pass/fail counts

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

## Output files

| File | Description |
|------|-------------|
| `.artifacts/specdown/report.html` | Executed specification HTML report |
| `.artifacts/specdown/report.json` | Machine-readable results |
| `.artifacts/specdown/models/*.als` | Combined Alloy models |

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
{"entry":"index.spec.md","adapters":[{"name":"d","command":["sh","./diag-adapter.sh"],"blocks":[],"fixtures":["diag"]}],"reporters":[{"builtin":"html","outFile":"diag-report.html"}]}
CFG
specdown run -config .tmp-test/diag.json 2>&1 || true
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
