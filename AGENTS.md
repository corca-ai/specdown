# Agent Guide

`specdown`은 Markdown-first executable specification 시스템이다.
목표는 reusable core와 product-specific adapter를 분리하는 것이다.
`coop`는 첫 번째 reference adapter일 뿐, 코어 자체가 아니다.

## Read First

작업 전 아래 문서를 먼저 읽는다.

- [System design](docs/design.md) — 범위, 용어, 패키지 경계, 문법, 인터페이스, 단계별 산출물의 기준 문서
- [Documentation guide](docs/metadoc.md) — 문서를 짧고 정확하게 유지하는 규칙

## Working Rules

- `docs/design.md`를 현재 아키텍처의 source of truth로 취급한다.
- `core`와 `adapter`의 책임 경계를 유지한다.
- `coop` 전용 DSL, helper, runtime 의미는 코어가 아니라 adapter에 둔다.
- 문서와 예시는 현재 설계 용어를 그대로 따른다.
  - executable block
  - fixture table
  - `alloy:model(name)`
  - HTML report
  - `SpecId`, `SpecEvent`
- 오래된 용어와 이전 아키텍처 흔적은 발견 즉시 수정하거나 삭제한다.

## Target Package Shape

현재 설계의 권장 구성은 다음과 같다.

- `specdown-core`
- `specdown-cli`
- `specdown-adapter-protocol`
- `specdown-reporter-html`
- `specdown-alloy`
- `specdown-adapter-shell`
- `specdown-adapter-vitest`
- `specdown-adapter-coop`

## Documentation Notes

- `AGENTS.md`는 entry point만 담고, 자세한 설명은 개별 문서로 분리한다.
- 새 문서를 추가할 때는 작은 주제 단위로 만들고 관련 상위 문서에서 링크한다.
- 자동 생성 문서는 버전 관리하지 않는다.
- 예시는 실제 설계와 런타임 동작에 맞아야 한다.

`CLAUDE.md`는 `AGENTS.md`의 symlink로 유지한다.
