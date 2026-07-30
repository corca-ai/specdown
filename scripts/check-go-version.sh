#!/bin/sh
set -eu

ROOT=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"

fail() {
  echo "Go toolchain version drift: $*" >&2
  exit 1
}

is_exact_version() {
  VERSION_MAJOR=${1%%.*}
  VERSION_REST=${1#*.}
  [ "$VERSION_REST" != "$1" ] || return 1

  VERSION_MINOR=${VERSION_REST%%.*}
  VERSION_PATCH=${VERSION_REST#*.}
  [ "$VERSION_PATCH" != "$VERSION_REST" ] || return 1

  case "$VERSION_PATCH" in
    *.*) return 1 ;;
  esac
  for VERSION_PART in "$VERSION_MAJOR" "$VERSION_MINOR" "$VERSION_PATCH"; do
    case "$VERSION_PART" in
      ""|*[!0-9]*) return 1 ;;
    esac
  done
}

GO_VERSION=$(awk '$1 == "go" { print $2 }' go.mod)
if [ -z "$GO_VERSION" ]; then
  fail "go.mod does not declare a Go version"
fi
if ! is_exact_version "$GO_VERSION"; then
  fail "go.mod must use an exact major.minor.patch version, got $GO_VERSION"
fi

if [ ! -f mise.toml ]; then
  fail "mise.toml is missing"
fi
if grep -Eq '^/?mise[.]toml$' .gitignore; then
  fail "mise.toml is ignored"
fi

if ! MISE_VERSION=$(awk -F'"' '
  /^[[:space:]]*\[/ {
    in_tools = ($0 ~ /^[[:space:]]*\[tools\][[:space:]]*$/)
    next
  }
  in_tools && /^[[:space:]]*go[[:space:]]*=/ {
    count++
    version = $2
  }
  END {
    if (count != 1) {
      exit 1
    }
    print version
  }
' mise.toml); then
  fail "mise.toml must contain exactly one go entry in [tools]"
fi
if [ "$MISE_VERSION" != "$GO_VERSION" ]; then
  fail "mise.toml uses ${MISE_VERSION:-<missing>}, go.mod uses $GO_VERSION"
fi

for WORKFLOW in \
  .github/workflows/ci.yml \
  .github/workflows/selfspec.yml \
  .github/workflows/release.yml
do
  COUNT=$(awk -F"'" '/^[[:space:]]*go-version:/ { count++ } END { print count + 0 }' "$WORKFLOW")
  if [ "$COUNT" -ne 1 ]; then
    fail "$WORKFLOW must contain exactly one single-quoted go-version"
  fi
  WORKFLOW_VERSION=$(awk -F"'" '/^[[:space:]]*go-version:/ { print $2 }' "$WORKFLOW")
  if [ "$WORKFLOW_VERSION" != "$GO_VERSION" ]; then
    fail "$WORKFLOW uses ${WORKFLOW_VERSION:-<missing>}, go.mod uses $GO_VERSION"
  fi
done

echo "Go toolchain versions aligned at $GO_VERSION"
