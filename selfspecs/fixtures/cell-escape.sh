#!/bin/sh
# Fixture: cell-escape
# Verifies that the adapter receives unescaped cell values.
#
# Columns: input, expected
# The core unescapes cells before sending to the adapter,
# so COL_INPUT and COL_EXPECTED should already be unescaped.

set -e

input="$COL_INPUT"
expected="$COL_EXPECTED"

if [ "$input" = "$expected" ]; then
  exit 0
else
  echo "cell escape mismatch" >&2
  echo "  input:    $(printf '%s' "$input" | cat -v)" >&2
  echo "  expected: $(printf '%s' "$expected" | cat -v)" >&2
  exit 1
fi
