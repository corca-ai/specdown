#!/bin/sh
# Check: block-kind
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
tmpspec=".tmp-test/check-test-$$.spec.md"
tmpentry=".tmp-test/check-test-$$-index.spec.md"
tmpcfg=".tmp-test/check-test-$$.json"
trap 'rm -f "$tmpspec" "$tmpentry" "$tmpcfg"' EXIT

cat > "$tmpspec" <<SPEC
# Test

\`\`\`${info}
echo hello
\`\`\`
SPEC

printf '# T\n\n- [Test](%s)\n' "$(basename "$tmpspec")" > "$tmpentry"

cat > "$tmpcfg" <<CFG
{"entry":"$(basename "$tmpentry")","adapters":[{"name":"t","command":["true"],"blocks":["${expected_kind}:${expected_target}"],"checks":[]}]}
CFG

specdown run -config "$tmpcfg" -dry-run >/dev/null 2>&1
