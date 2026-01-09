# 변경 이력

이 프로젝트의 모든 주요 변경사항을 기록합니다.

[Keep a Changelog](https://keepachangelog.com/en/1.0.0/) 형식을 따르며,
[Semantic Versioning](https://semver.org/spec/v2.0.0.html)을 준수합니다.

## [Unreleased]

## [0.2.2] - 2025-01-10

### 변경
- 마크다운 테이블의 병합 셀 표시 방식 개선:
  - 세로 병합 셀(rowspan): `〃` 표시
  - 가로 병합 셀(colspan): 빈칸 유지
- 테이블 구조를 유지하면서 가독성 향상

## [0.2.1] - 2025-01-10

### 추가
- Upstage Document Parse를 Stage 1 파서로 지원 (`--parser upstage`, `HWP2MD_PARSER=upstage`)
  - Upstage API를 통한 HWP/HWPX 직접 지원
  - OCR 기반 복잡한 레이아웃 인식
  - API 마크다운 출력 활용으로 테이블/리스트 렌더링 개선
- IR에 `RawMarkdown` 필드 추가 (외부 파서 패스스루)
- README에 Stage 1 파서 비교 예시 추가

## [0.2.0] - 2025-01-10

### 추가
- Upstage Solar LLM 프로바이더 (`solar-pro`, `solar-mini`)
- `--base-url` 플래그 및 `HWP2MD_BASE_URL` 환경변수로 프라이빗 테넌시 지원
  - AWS Bedrock, Azure OpenAI, 로컬 서버 엔드포인트
- 모델 이름으로 LLM 프로바이더 자동 감지 (claude-* → anthropic, gpt-* → openai, solar-* → upstage 등)
- HWPX-Markdown 차이점 문서 (`docs/hwpx-markdown-differences.md`)
- 릴리즈 자동화를 위한 Claude skill
- README에 Mermaid 아키텍처 다이어그램

### 변경
- `HWP2MD_PROVIDER` 환경변수 제거로 설정 단순화
- LLM 프로바이더 선택은 `HWP2MD_MODEL`만 설정하면 됨
- `convert` 명령어 생략 가능 (기본 명령)
- 문서에서 Go install을 바이너리 다운로드보다 우선 안내
- CI가 문서만 변경된 경우 생략

## [0.1.0] - 2024-01-10

### 추가
- hwp2markdown CLI 도구 초기 릴리즈
- 2단계 파이프라인 아키텍처:
  - Stage 1: 중간 표현(IR)을 사용한 HWPX 파서
  - Stage 2: LLM 기반 Markdown 포맷팅 (선택적)
- HWPX 포맷 지원 (한컴오피스 2014+ XML 기반 HWP)
- 다중 LLM 프로바이더:
  - Anthropic Claude (기본)
  - OpenAI GPT
  - Google Gemini
  - Ollama (로컬)
- 셀 병합을 지원하는 테이블 파싱
- 중첩 테이블 처리 (인라인 텍스트로 변환)
- 법률 콘텐츠 테이블을 리스트 형식으로 변환
- 전각/반각 공백 처리 (`<hp:fwSpace/>`, `<hp:hwSpace/>`)
- 정보 박스 테이블 감지 및 포맷팅
- CLI 명령:
  - `convert` - HWP/HWPX를 Markdown으로 변환
  - `extract` - IR을 JSON/텍스트 형식으로 추출
  - `providers` - 사용 가능한 LLM 프로바이더 목록
  - `config` - 설정 관리
- 환경 변수 설정
- YAML 설정 파일 지원
- pre-commit 린팅을 위한 Git hooks
- GitHub Actions CI/CD 파이프라인
- 크로스 플랫폼 빌드를 위한 goreleaser 설정

### 기술 세부사항
- Go 1.24+로 작성
- CLI 프레임워크로 Cobra 사용
- 모듈형 LLM 프로바이더 아키텍처
- 종합적인 테스트 커버리지

[Unreleased]: https://github.com/roboco-io/hwp2markdown/compare/v0.2.2...HEAD
[0.2.2]: https://github.com/roboco-io/hwp2markdown/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/roboco-io/hwp2markdown/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/roboco-io/hwp2markdown/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/roboco-io/hwp2markdown/releases/tag/v0.1.0
