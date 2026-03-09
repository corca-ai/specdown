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

This creates `specdown.json`, `specs/index.spec.md`, and `specs/example.spec.md`.

```run:shell
# Scaffold a fresh project and verify files exist
rm -rf .tmp-test/init-overview && mkdir -p .tmp-test/init-overview && cd .tmp-test/init-overview && specdown init >/dev/null 2>&1
```

```run:shell
$ test -f .tmp-test/init-overview/specdown.json && echo yes
yes
$ test -f .tmp-test/init-overview/specs/index.spec.md && echo yes
yes
$ test -f .tmp-test/init-overview/specs/example.spec.md && echo yes
yes
```

### What a spec looks like

This section is a working example. In the HTML report, the block below appears green:

```run:shell
$ echo hello
hello
```

This block intentionally mismatches. It appears red and counts as a failure,
but `!fail` prevents it from causing a non-zero exit code:

```run:shell !fail
$ echo hello
goodbye
```

### Run specs

```sh
specdown run
```

Specs are parsed, executed via adapters, and results are rendered as an HTML report.
Use `-dry-run` to validate syntax without executing.

```run:shell
$ cd .tmp-test/init-overview && specdown run -dry-run 2>&1 | grep 'spec(s)'
...
```

### Install Claude Code skill

```sh
specdown install skills
```

This installs the `/specdown` skill with syntax reference, adapter protocol, and best practices.

## Why

When documents and test code are separated, properties stated in documents may not be verified, and tests verify behavior but do not explain design intent.

specdown solves this by making a single Markdown document serve as prose, executable tests, and optional formal models.
