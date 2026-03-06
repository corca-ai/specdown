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
