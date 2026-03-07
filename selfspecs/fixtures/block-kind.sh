#!/bin/sh
# Fixture: block-kind
# Verifies that a block info string has the expected kind and target.
#
# Columns: info, kind, target

set -e

info="$COL_INFO"
expected_kind="$COL_KIND"
expected_target="$COL_TARGET"

# Strip capture clause (-> $var) if present
base=$(echo "$info" | sed 's/ *->.*$//')

# Extract kind (before colon)
actual_kind=$(echo "$base" | cut -d: -f1)

# Extract target (after colon)
actual_target=$(echo "$base" | cut -d: -f2-)

if [ "$actual_kind" != "$expected_kind" ]; then
  echo "kind mismatch: expected '$expected_kind', got '$actual_kind'" >&2
  exit 1
fi

if [ "$actual_target" != "$expected_target" ]; then
  echo "target mismatch: expected '$expected_target', got '$actual_target'" >&2
  exit 1
fi

# Also verify specdown dry-run can parse a spec with this block.
mkdir -p .tmp-test
tmpspec=".tmp-test/fixture-test-$$.spec.md"
tmpcfg=".tmp-test/fixture-test-$$.json"
trap 'rm -f "$tmpspec" "$tmpcfg"' EXIT

cat > "$tmpspec" <<SPEC
# Test

\`\`\`${info}
echo hello
\`\`\`
SPEC

cat > "$tmpcfg" <<CFG
{"include":["$(basename "$tmpspec")"],"adapters":[{"name":"t","command":["true"],"blocks":["${expected_kind}:${expected_target}"],"fixtures":[]}]}
CFG

specdown run -config "$tmpcfg" -dry-run >/dev/null 2>&1
