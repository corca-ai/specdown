# Build & Run

## Prerequisites

Go 툴체인이 필요하다. 이 프로젝트는 nix 환경에서 관리한다.

```sh
nix-shell -p go
```

## Build

```sh
go build -o ~/.local/bin/specdown ./cmd/specdown
```

## Run

프로젝트 루트에서 실행한다. `specdown.json`을 설정으로 읽는다.

```sh
specdown run
```

리포트는 `.artifacts/specdown/report.html`에 생성된다.

## Test

```sh
go test ./...
```

## Release

`v*` 태그를 push하면 GitHub Actions가 Windows, macOS, Linux 바이너리를 빌드해서 Release에 첨부한다.

```sh
git tag v0.4.0
git push origin v0.4.0
```
