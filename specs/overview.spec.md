---
type: guide
workdir: .tmp-test
---

# Overview

`specdown` is a Markdown-first executable specification system.
A single Markdown document serves as both a readable specification and an executable test.

## Install

### Binary (recommended)

```sh
curl -sSfL https://raw.githubusercontent.com/corca-ai/specdown/main/install.sh | sh
```

### go install

```sh
go install github.com/corca-ai/specdown/cmd/specdown@latest
```

### Homebrew

```sh
brew install corca-ai/tap/specdown
```

## Getting Started

### Create a project

```sh
specdown init
```

This creates a [explains::configuration file](config.spec.md) and example specs
in the current directory. Run this from the project root so that
`specdown.json` sits next to `.git/`.

```run:shell
# Scaffold a fresh project and verify files exist
rm -rf init-overview && mkdir -p init-overview && cd init-overview && specdown init >/dev/null 2>&1
```

```run:shell
$ test -f init-overview/specdown.json && echo yes
yes
$ test -f init-overview/specs/index.spec.md && echo yes
yes
$ test -f init-overview/specs/example.spec.md && echo yes
yes
```

### Run specs

```sh
specdown run
```

Specs are parsed, executed via [explains::adapters](adapter-protocol.spec.md), and results are rendered as an [explains::HTML report](report.spec.md).
See the [explains::CLI reference](cli.spec.md) for all flags. Use `-dry-run` to validate syntax without executing.

```run:shell
$ cd init-overview && specdown run -dry-run 2>&1 | grep 'spec(s)'
...
```

### Install Claude Code skill

```sh
specdown install skills
```

This installs the `/specdown` skill with syntax reference, adapter protocol, and best practices.

The next chapter, [explains::Spec Syntax](syntax.spec.md), covers the full authoring surface: shell blocks, doctest blocks, variables, check tables, and hooks.
