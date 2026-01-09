# hwp2markdown

HWP(한글 워드프로세서) 문서를 Markdown으로 변환하는 CLI 도구

## 개요

이 프로젝트는 HWP/HWPX 문서를 Markdown으로 변환합니다. 2단계 파이프라인 아키텍처를 사용하여 정확하고 읽기 쉬운 Markdown을 생성합니다.

### 아키텍처

```
HWP/HWPX → [Stage 1: Parser] → IR (JSON) → [Stage 2: LLM] → Markdown
```

- **Stage 1 (Parser)**: 문서를 파싱하여 중간 표현(IR)으로 변환
- **Stage 2 (LLM)**: LLM을 사용하여 IR을 깔끔한 Markdown으로 포맷팅 (선택적)

## 설치

### 바이너리 다운로드

[GitHub Releases](https://github.com/roboco-io/hwp2markdown/releases)에서 플랫폼별 바이너리를 다운로드할 수 있습니다.

| 플랫폼 | 아키텍처 | 파일 |
|--------|----------|------|
| Windows | x64 | `hwp2markdown_*_windows_amd64.zip` |
| macOS | Intel | `hwp2markdown_*_darwin_amd64.tar.gz` |
| macOS | Apple Silicon | `hwp2markdown_*_darwin_arm64.tar.gz` |
| Linux | x64 | `hwp2markdown_*_linux_amd64.tar.gz` |

### Go 설치

```bash
go install github.com/roboco-io/hwp2markdown/cmd/hwp2markdown@latest
```

## 사용법

### 기본 변환 (Stage 1만)

```bash
# HWPX 파일을 Markdown으로 변환
hwp2markdown convert document.hwpx -o output.md

# 표준 출력으로 변환
hwp2markdown convert document.hwpx
```

### LLM 포맷팅 (Stage 2)

LLM을 사용하면 더 자연스럽고 읽기 쉬운 Markdown을 생성할 수 있습니다.

```bash
# Anthropic Claude 사용 (기본)
export ANTHROPIC_API_KEY="your-api-key"
hwp2markdown convert document.hwpx --llm

# OpenAI GPT 사용
export OPENAI_API_KEY="your-api-key"
hwp2markdown convert document.hwpx --llm --provider openai

# Google Gemini 사용
export GOOGLE_API_KEY="your-api-key"
hwp2markdown convert document.hwpx --llm --provider gemini

# Ollama 사용 (로컬)
hwp2markdown convert document.hwpx --llm --provider ollama --model llama3.2
```

### IR 추출

```bash
# JSON 형식으로 IR 추출
hwp2markdown extract document.hwpx -o output.json

# 텍스트 형식으로 추출
hwp2markdown extract document.hwpx --format text
```

### 환경 변수

| 변수 | 설명 |
|------|------|
| `HWP2MD_LLM` | `true`로 설정하면 LLM 포맷팅 활성화 |
| `HWP2MD_PROVIDER` | LLM 프로바이더 (anthropic, openai, gemini, ollama) |
| `HWP2MD_MODEL` | 사용할 모델 이름 |
| `ANTHROPIC_API_KEY` | Anthropic API 키 |
| `OPENAI_API_KEY` | OpenAI API 키 |
| `GOOGLE_API_KEY` | Google Gemini API 키 |
| `OLLAMA_HOST` | Ollama 서버 주소 (기본: http://localhost:11434) |

## 지원 포맷

| 포맷 | 상태 | 설명 |
|------|------|------|
| HWPX | ✅ 지원 | XML 기반 개방형 포맷 (한컴오피스 2014+) |
| HWP 5.x | 🚧 계획 | OLE/CFBF 바이너리 포맷 |
| HWP 3.x | 미정 | 레거시 바이너리 포맷 |

## LLM 프로바이더

| 프로바이더 | 기본 모델 | 설명 |
|------------|-----------|------|
| `anthropic` | claude-3-5-sonnet-20241022 | Anthropic Claude |
| `openai` | gpt-4o-mini | OpenAI GPT |
| `gemini` | gemini-1.5-flash | Google Gemini |
| `ollama` | llama3.2 | 로컬 Ollama 서버 |

## 개발

### 요구사항

- Go 1.24 이상

### 빌드

```bash
# 저장소 클론
git clone https://github.com/roboco-io/hwp2markdown.git
cd hwp2markdown

# 의존성 다운로드
go mod download

# 빌드
make build

# 테스트
make test

# 린트 (golangci-lint 필요)
make lint
```

### 프로젝트 구조

```
hwp2markdown/
├── cmd/hwp2markdown/      # CLI 엔트리포인트
├── internal/
│   ├── cli/               # CLI 명령 구현
│   ├── config/            # 설정 관리
│   ├── ir/                # 중간 표현 (Intermediate Representation)
│   ├── llm/               # LLM 프로바이더
│   │   ├── anthropic/     # Anthropic Claude
│   │   ├── openai/        # OpenAI GPT
│   │   ├── gemini/        # Google Gemini
│   │   └── ollama/        # Local Ollama
│   └── parser/            # 문서 파서
│       └── hwpx/          # HWPX 파서
├── docs/                  # 문서
└── tests/                 # 테스트 데이터
```

## 문서

- [PRD](docs/PRD.md) - 제품 요구사항
- [기술 스택](docs/tech-stack.md) - 기술 스택 결정
- [HWPX 스키마](docs/hwpx-schema.md) - HWPX 파일 포맷 문서

## 라이센스

MIT License - 자세한 내용은 [LICENSE](LICENSE) 파일을 참조하세요.
