# Configuration & Running

## 설정 파일

프로젝트 루트에 `specdown.json`을 만든다.

```json
{
  "include": ["specs/**/*.spec.md"],
  "adapters": [
    {
      "name": "myapp",
      "command": ["python3", "./tools/myapp_adapter.py"],
      "protocol": "specdown-adapter/v1"
    }
  ],
  "reporters": [
    {
      "builtin": "html",
      "outFile": ".artifacts/specdown/report.html"
    }
  ],
  "models": {
    "builtin": "alloy"
  }
}
```

| 필드 | 설명 |
|------|------|
| `include` | spec 파일 glob 패턴 |
| `adapters` | 실행 블록과 fixture를 처리할 adapter 목록 |
| `reporters` | 결과물 생성기. 현재 `html` builtin 제공 |
| `models` | Alloy 모델 검증. 사용하지 않으면 생략 가능 |

## 실행

```sh
specdown run
specdown run -out report.html     # 리포트 경로 직접 지정
specdown run -config other.json   # 다른 설정 파일 사용
```

모든 spec 파일을 파싱하고, adapter를 실행하고, 리포트를 생성한다.
실패 시 각 실패 항목의 상세 내용을 stderr에 출력한다.

## 산출물

| 파일 | 설명 |
|------|------|
| `.artifacts/specdown/report.html` | 실행된 명세서 HTML 리포트 |
| `.artifacts/specdown/report.json` | 기계 판독용 결과 |
| `.artifacts/specdown/models/*.als` | 결합된 Alloy 모델 |

## 프로젝트 구성 예시

```
my-project/
├── specdown.json
├── specs/
│   ├── auth.spec.md
│   └── billing.spec.md
├── tools/
│   └── myapp_adapter.py
└── .artifacts/
    └── specdown/
        └── report.html      ← 자동 생성, 버전 관리 제외
```

`.artifacts/`는 `.gitignore`에 추가한다.
