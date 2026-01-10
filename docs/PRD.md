# hwp2md PRD (Product Requirements Document)

## 1. 개요

### 1.1 프로젝트 명
hwp2md

### 1.2 목적
HWP(한글 워드프로세서) 문서를 Markdown으로 변환하는 오픈소스 도구 개발

### 1.3 배경
- HWP는 한국에서 널리 사용되는 문서 포맷이나, 버전 간 호환성 문제와 폐쇄적 생태계로 인해 활용에 제약이 있음
- LLM/AI 시대에 문서를 Markdown으로 변환하여 처리하려는 수요 증가
- 기존 솔루션들의 한계:
  - `unhwp` (Rust): 변환 품질 문제 (불필요한 HTML 태그, 스타일 정보 손실)
  - `hwpjs`: JSON까지만 파싱, Markdown 변환 미지원
  - 상용 서비스: 외부 의존성, 비용, 프라이버시 우려
- **복잡한 레이아웃(표, 다단, 양식)의 완벽한 변환은 기술적으로 어려움**
  - 텍스트 추출 후 LLM을 활용한 포맷팅이 더 효과적

### 1.4 목표
- HWPX 및 HWP 5.x 포맷의 텍스트 추출 및 기본 Markdown 변환 지원
- **2단계 파이프라인**: 텍스트 추출 → LLM 포맷팅
- **LLM 플러그인 아키텍처**: OpenAI, Anthropic, Gemini 등 다양한 LLM 지원
- CLI 및 라이브러리 형태로 제공
- MIT 라이선스 오픈소스

---

## 2. 핵심 아키텍처

### 2.1 2단계 파이프라인

```
┌─────────────────────────────────────────────────────────────────────┐
│                        hwp2md Pipeline                         │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│   Stage 1: Extraction                 Stage 2: Formatting           │
│   ┌─────────────────────┐            ┌─────────────────────┐       │
│   │                     │            │                     │       │
│   │   HWP/HWPX Parser   │ ────────▶  │   LLM Formatter     │       │
│   │                     │            │                     │       │
│   │   - Text extraction │   Raw      │   - Structure       │       │
│   │   - Style hints     │   Text +   │     recognition     │       │
│   │   - Table markers   │   Hints    │   - Table formatting│       │
│   │   - Image refs      │            │   - Markdown output │       │
│   │                     │            │                     │       │
│   └─────────────────────┘            └─────────────────────┘       │
│           │                                    │                    │
│           ▼                                    ▼                    │
│   ┌─────────────────────┐            ┌─────────────────────┐       │
│   │  Intermediate       │            │  Final Markdown     │       │
│   │  Representation     │            │  Output             │       │
│   └─────────────────────┘            └─────────────────────┘       │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### 2.2 각 단계의 역할

| 단계 | 역할 | 입력 | 출력 |
|------|------|------|------|
| **Stage 1** (Parser) | 텍스트 추출 + 구조 힌트 | HWP/HWPX 파일 | Intermediate Representation |
| **Stage 2** (LLM) | 문맥 이해 + 구조화 | Intermediate Representation | 최종 Markdown |

### 2.3 Intermediate Representation (IR)

Stage 1에서 출력하는 중간 표현:

```json
{
  "version": "1.0",
  "metadata": {
    "title": "문서 제목",
    "author": "작성자",
    "created": "2024-01-01"
  },
  "content": [
    {
      "type": "paragraph",
      "text": "일반 텍스트 내용",
      "style": {
        "heading_level": 0,
        "bold": false,
        "italic": false
      }
    },
    {
      "type": "table_region",
      "hint": "표 영역 (3x4 추정)",
      "raw_text": "열1\t열2\t열3\n값1\t값2\t값3\n..."
    },
    {
      "type": "image",
      "path": "images/image001.png",
      "alt": ""
    },
    {
      "type": "list",
      "items": ["항목1", "항목2"],
      "ordered": false
    }
  ]
}
```

---

## 3. LLM 플러그인 아키텍처

### 3.1 Provider Interface

```go
// LLMProvider는 다양한 LLM 서비스를 추상화하는 인터페이스
type LLMProvider interface {
    // Name returns the provider name (e.g., "openai", "anthropic", "gemini")
    Name() string

    // Format takes intermediate representation and returns formatted markdown
    Format(ctx context.Context, ir *IntermediateRepresentation, opts FormatOptions) (*FormatResult, error)

    // Validate checks if the provider is properly configured
    Validate() error
}

type FormatOptions struct {
    // Document type hint for better formatting
    DocumentType string // "government", "academic", "general"

    // Language preference
    Language string // "ko", "en"

    // Custom system prompt (optional)
    SystemPrompt string

    // Max tokens for response
    MaxTokens int
}

type FormatResult struct {
    Markdown string
    Usage    TokenUsage
    Warnings []string
}

type TokenUsage struct {
    InputTokens  int
    OutputTokens int
}
```

### 3.2 지원 LLM Providers

| Provider | 패키지 | 지원 모델 | 우선순위 |
|----------|--------|-----------|----------|
| **OpenAI** | `internal/llm/openai` | GPT-4o, GPT-4o-mini | P0 |
| **Anthropic** | `internal/llm/anthropic` | Claude 3.5 Sonnet, Claude 3 Haiku | P0 |
| **Google** | `internal/llm/gemini` | Gemini 1.5 Pro, Gemini 1.5 Flash | P1 |
| **Ollama** | `internal/llm/ollama` | Llama 3, Mistral 등 로컬 모델 | P1 |
| **Custom** | `internal/llm/custom` | 사용자 정의 엔드포인트 | P2 |

### 3.3 Provider 등록 및 사용

```go
// Provider Registry
type ProviderRegistry struct {
    providers map[string]LLMProvider
}

func NewProviderRegistry() *ProviderRegistry {
    r := &ProviderRegistry{
        providers: make(map[string]LLMProvider),
    }
    // Register built-in providers
    r.Register(openai.New())
    r.Register(anthropic.New())
    r.Register(gemini.New())
    r.Register(ollama.New())
    return r
}

func (r *ProviderRegistry) Register(p LLMProvider) {
    r.providers[p.Name()] = p
}

func (r *ProviderRegistry) Get(name string) (LLMProvider, error) {
    p, ok := r.providers[name]
    if !ok {
        return nil, fmt.Errorf("provider not found: %s", name)
    }
    return p, nil
}
```

### 3.4 설정 파일

```yaml
# ~/.hwp2md/config.yaml
default_provider: anthropic

providers:
  openai:
    api_key: ${OPENAI_API_KEY}
    model: gpt-4o-mini

  anthropic:
    api_key: ${ANTHROPIC_API_KEY}
    model: claude-sonnet-4-20250514

  gemini:
    api_key: ${GOOGLE_API_KEY}
    model: gemini-1.5-flash

  ollama:
    base_url: http://localhost:11434
    model: llama3.2
```

---

## 4. 사용자 및 사용 사례

### 4.1 대상 사용자

| 사용자 유형 | 설명 |
|-------------|------|
| 개발자 | HWP 문서를 프로그래밍 방식으로 처리해야 하는 개발자 |
| 데이터 엔지니어 | HWP 문서에서 텍스트를 추출하여 데이터 파이프라인에 활용 |
| AI/ML 엔지니어 | LLM 학습/추론을 위해 HWP 문서를 Markdown으로 변환 |
| 일반 사용자 | CLI를 통해 HWP 문서를 Markdown으로 변환하려는 사용자 |

### 4.2 주요 사용 사례

#### UC-1: CLI를 통한 단일 파일 변환 (LLM 사용)
```bash
# 기본 변환 (LLM 포맷팅 포함)
hwp2md convert document.hwpx -o output.md

# 특정 LLM 프로바이더 지정
hwp2md convert document.hwpx -o output.md --provider anthropic

# OpenAI 사용
hwp2md convert document.hwpx -o output.md --provider openai --model gpt-4o
```

#### UC-2: 텍스트만 추출 (LLM 없이)
```bash
# Stage 1만 실행 - 원시 텍스트 추출
hwp2md extract document.hwpx -o output.txt

# IR (Intermediate Representation) JSON 출력
hwp2md extract document.hwpx -o output.json --format ir
```

#### UC-3: 배치 변환
```bash
hwp2md convert ./documents/*.hwpx -o ./output/ --provider gemini
```

#### UC-4: 라이브러리로 프로그래밍 방식 사용
```go
import "github.com/roboco-io/hwp2md/pkg/hwp2md"

// Stage 1만 사용
ir, err := hwp2md.Extract("document.hwpx")

// Stage 1 + Stage 2 (LLM 포맷팅)
result, err := hwp2md.Convert("document.hwpx", hwp2md.Options{
    Provider: "anthropic",
    Model:    "claude-sonnet-4-20250514",
})
fmt.Println(result.Markdown)
```

#### UC-5: 이미지 추출과 함께 변환
```bash
hwp2md convert input.hwpx -o output.md --extract-images ./images/
```

---

## 5. 기능 요구사항

### 5.1 지원 포맷

| 우선순위 | 포맷 | 설명 |
|----------|------|------|
| P0 | HWPX | XML 기반 개방형 포맷 (ZIP + XML) |
| P1 | HWP 5.x | OLE/Compound 바이너리 포맷 |
| P2 | HWP 3.x | 레거시 바이너리 포맷 (향후 검토) |

### 5.2 Stage 1 (Parser) 출력 요소

| 요소 | 설명 | 출력 형태 |
|------|------|-----------|
| 일반 텍스트 | 문단 내용 | 그대로 출력 |
| 문단 구분 | 문단 경계 | 명시적 구분자 |
| 스타일 힌트 | 굵게, 기울임, 제목 등 | 메타데이터 |
| 표 영역 | 표 시작/끝 마커 | 탭/줄바꿈 구분 텍스트 + 힌트 |
| 이미지 참조 | 이미지 위치 | 파일 경로 + 위치 마커 |
| 목록 | 순서/비순서 목록 | 항목 + 유형 힌트 |

### 5.3 Stage 2 (LLM) 처리 요소

| 요소 | LLM 처리 | Markdown 출력 |
|------|----------|---------------|
| 구조 인식 | 문맥 기반 헤딩 레벨 판단 | `#`, `##`, `###` |
| 표 재구성 | 원시 텍스트 → 구조화 | GFM 테이블 |
| 목록 정리 | 중첩 구조 인식 | `- `, `1. ` |
| 인용문 | 문맥 기반 판단 | `> ` |
| 강조 | 스타일 힌트 활용 | `**`, `*`, `~~` |

### 5.4 CLI 인터페이스

```
hwp2md [command] [OPTIONS] <INPUT>...

Commands:
  convert     HWP/HWPX 파일을 Markdown으로 변환
  extract     HWP/HWPX 파일에서 텍스트만 추출 (Stage 1만)
  providers   사용 가능한 LLM 프로바이더 목록
  config      설정 관리

Convert Options:
  -o, --output <PATH>       출력 파일 또는 디렉토리
  --llm                     LLM 포맷팅 활성화 (Stage 2)
                            환경변수: HWP2MD_LLM=true
  --provider <NAME>         LLM 프로바이더 (자동 감지됨)
                            가능한 값: openai, anthropic, gemini, ollama
  --model <NAME>            LLM 모델 (프로바이더 자동 감지)
                            환경변수: HWP2MD_MODEL
                            claude-* → anthropic, gpt-*/o1-*/o3-* → openai
                            gemini-* → gemini, 그 외 → ollama
  --extract-images <DIR>    이미지 추출 디렉토리

Extract Options:
  -o, --output <PATH>       출력 파일 또는 디렉토리
  -f, --format <FORMAT>     출력 포맷 [기본값: text]
                            가능한 값: text, ir (JSON)
  --extract-images <DIR>    이미지 추출 디렉토리

Global Options:
  -v, --verbose             상세 출력
  -q, --quiet               조용한 모드
  -h, --help                도움말 출력
  -V, --version             버전 출력
```

### 5.5 환경변수

| 환경변수 | 설명 | 기본값 |
|----------|------|--------|
| `HWP2MD_LLM` | LLM 포맷팅 활성화 (`true`/`false`) | `false` |
| `HWP2MD_MODEL` | LLM 모델 (프로바이더 자동 감지) | 프로바이더 기본값 |
| `OPENAI_API_KEY` | OpenAI API 키 | - |
| `ANTHROPIC_API_KEY` | Anthropic API 키 | - |
| `GOOGLE_API_KEY` | Google Gemini API 키 | - |

### 5.6 사용 예시

```bash
# Stage 1만 (기본값) - LLM 없이 기본 Markdown 변환
hwp2md convert document.hwpx -o output.md

# Stage 1 + Stage 2 - LLM 포맷팅 활성화 (플래그)
hwp2md convert document.hwpx -o output.md --llm

# Stage 1 + Stage 2 - LLM 포맷팅 활성화 (환경변수)
HWP2MD_LLM=true hwp2md convert document.hwpx -o output.md

# 특정 모델 지정 (프로바이더 자동 감지)
hwp2md convert document.hwpx -o output.md --llm --model gpt-4o

# 환경변수로 모델 설정
export HWP2MD_LLM=true
export HWP2MD_MODEL=gpt-4o-mini
hwp2md convert document.hwpx -o output.md

# IR JSON 추출 (Stage 1만)
hwp2md extract document.hwpx -o output.json --format ir
```

### 5.7 라이브러리 API

#### Go API

```go
import "github.com/roboco-io/hwp2md/pkg/hwp2md"

// Stage 1만: 텍스트 추출 + 기본 Markdown 변환
result, err := hwp2md.Convert("document.hwpx", hwp2md.Options{
    ExtractImages: true,
    ImageDir:      "./images",
})

// Stage 1 + Stage 2: LLM 포맷팅 활성화
result, err := hwp2md.Convert("document.hwpx", hwp2md.Options{
    UseLLM:        true,  // LLM 포맷팅 활성화
    Provider:      "anthropic",
    Model:         "claude-sonnet-4-20250514",
    ExtractImages: true,
    ImageDir:      "./images",
})

// IR만 추출 (Intermediate Representation)
ir, err := hwp2md.Extract("document.hwpx")

// 결과 사용
fmt.Println(result.Markdown)
if result.TokenUsage != nil {
    fmt.Printf("Tokens used: %d\n", result.TokenUsage.Total)
}
```

#### Options 및 결과 구조체

```go
type Options struct {
    UseLLM        bool   // LLM 포맷팅 활성화 (기본값: false)
    Provider      string // LLM 프로바이더 (openai, anthropic, gemini, ollama)
    Model         string // LLM 모델
    ExtractImages bool   // 이미지 추출 여부
    ImageDir      string // 이미지 저장 디렉토리
}

type ExtractResult struct {
    IR       *IntermediateRepresentation
    Images   []ImageInfo
    Metadata DocumentMetadata
    Warnings []string
}

type ConvertResult struct {
    Markdown   string
    Images     []ImageInfo
    Metadata   DocumentMetadata
    TokenUsage *TokenUsage // LLM 사용 시에만 채워짐
    Warnings   []string
}

type TokenUsage struct {
    Input  int
    Output int
    Total  int
}
```

---

## 6. 비기능 요구사항

### 6.1 성능

| 항목 | 목표 |
|------|------|
| Stage 1 (추출) | 10MB 문서 기준 5초 이내 |
| Stage 2 (LLM) | LLM API 응답 시간에 의존 |
| 메모리 사용 | 입력 파일 크기의 10배 이내 |
| 동시 처리 | 배치 변환 시 고루틴 병렬 처리 |

### 6.2 품질

| 항목 | 목표 |
|------|------|
| 텍스트 추출 정확도 | 99% 이상 (글자 손실 없음) |
| LLM 포맷팅 품질 | 수동 포맷팅의 90% 수준 |
| 테스트 커버리지 | 80% 이상 |

### 6.3 호환성

| 항목 | 요구사항 |
|------|----------|
| Go | 1.21 이상 |
| OS | Linux, macOS, Windows |
| 아키텍처 | amd64, arm64 |
| 인코딩 | UTF-8 출력 |

### 6.4 배포

| 항목 | 요구사항 |
|------|----------|
| 바이너리 | 주요 OS용 standalone 바이너리 (GitHub Releases) |
| Go 모듈 | `go get github.com/roboco-io/hwp2md` |
| Docker | Docker 이미지 제공 (향후) |

---

## 7. 기술 설계

### 7.1 전체 아키텍처

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           hwp2md                                   │
├─────────────────────────────────────────────────────────────────────────┤
│  CLI Layer                                                              │
│  ┌───────────────────────────────────────────────────────────────────┐ │
│  │  cobra 기반 CLI (convert, extract, providers, config)             │ │
│  └───────────────────────────────────────────────────────────────────┘ │
├─────────────────────────────────────────────────────────────────────────┤
│  Core API                                                               │
│  ┌───────────────────────────────────────────────────────────────────┐ │
│  │  Converter                                                         │ │
│  │  - Extract(path) -> IntermediateRepresentation                    │ │
│  │  - Convert(path, opts) -> Markdown                                │ │
│  └───────────────────────────────────────────────────────────────────┘ │
├─────────────────────────────────────────────────────────────────────────┤
│  Stage 1: Parser Layer                                                  │
│  ┌────────────────────────┐  ┌────────────────────────┐                │
│  │  HwpxParser            │  │  Hwp5Parser            │                │
│  │  (ZIP + XML)           │  │  (OLE/CFBF)            │                │
│  └────────────────────────┘  └────────────────────────┘                │
├─────────────────────────────────────────────────────────────────────────┤
│  Intermediate Representation (IR)                                       │
│  ┌───────────────────────────────────────────────────────────────────┐ │
│  │  Document, Paragraph, TableRegion, ImageRef, ListItems, ...       │ │
│  └───────────────────────────────────────────────────────────────────┘ │
├─────────────────────────────────────────────────────────────────────────┤
│  Stage 2: LLM Formatter Layer                                           │
│  ┌───────────────────────────────────────────────────────────────────┐ │
│  │  LLMProvider Interface                                            │ │
│  │  ┌─────────┐ ┌───────────┐ ┌─────────┐ ┌─────────┐ ┌──────────┐  │ │
│  │  │ OpenAI  │ │ Anthropic │ │ Gemini  │ │ Ollama  │ │ Custom   │  │ │
│  │  └─────────┘ └───────────┘ └─────────┘ └─────────┘ └──────────┘  │ │
│  └───────────────────────────────────────────────────────────────────┘ │
├─────────────────────────────────────────────────────────────────────────┤
│  Fallback: Basic Renderer (LLM 없이)                                    │
│  ┌───────────────────────────────────────────────────────────────────┐ │
│  │  BasicMarkdownRenderer - IR → 기본 Markdown (표 미지원)           │ │
│  └───────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────┘
```

### 7.2 핵심 모듈

| 모듈 | 역할 |
|------|------|
| `internal/parser/hwpx` | HWPX 파일 파싱 (ZIP + XML) |
| `internal/parser/hwp5` | HWP 5.x 파일 파싱 (OLE/CFBF) |
| `internal/ir` | Intermediate Representation 정의 |
| `internal/llm` | LLM Provider 인터페이스 및 레지스트리 |
| `internal/llm/openai` | OpenAI 프로바이더 |
| `internal/llm/anthropic` | Anthropic 프로바이더 |
| `internal/llm/gemini` | Google Gemini 프로바이더 |
| `internal/llm/ollama` | Ollama 프로바이더 |
| `internal/renderer` | 기본 Markdown 렌더러 (LLM 없이) |
| `internal/cli` | CLI 인터페이스 |
| `internal/config` | 설정 파일 관리 |

### 7.3 의존성

| 패키지 | 용도 |
|--------|------|
| `github.com/richardlehane/mscfb` | OLE/Compound 파일 파싱 (HWP 5.x) |
| `archive/zip` (표준) | HWPX ZIP 압축 해제 |
| `encoding/xml` (표준) | HWPX XML 파싱 |
| `github.com/spf13/cobra` | CLI 인터페이스 |
| `github.com/sashabaranov/go-openai` | OpenAI API 클라이언트 |
| `github.com/liushuangls/go-anthropic/v2` | Anthropic API 클라이언트 |
| `github.com/google/generative-ai-go` | Google Gemini API 클라이언트 |

---

## 8. 구현 단계

### Phase 1: 기본 인프라 구축

**목표**: 프로젝트 구조, IR 정의, LLM 플러그인 인터페이스

| 작업 | 설명 |
|------|------|
| 프로젝트 구조 설정 | Go 모듈 구조, 테스트 환경 |
| IR 모델 정의 | Intermediate Representation 구조체 |
| LLM Provider 인터페이스 | 추상 인터페이스 및 레지스트리 |
| 설정 파일 관리 | YAML 설정 로드/저장 |

### Phase 2: HWPX 파서 (Stage 1)

**목표**: HWPX 파일에서 IR 생성

| 작업 | 설명 |
|------|------|
| HWPX 파서 구현 | archive/zip + encoding/xml |
| 텍스트 추출 | 문단, 스타일 힌트 추출 |
| 표 영역 감지 | 표 영역 마킹 + 원시 텍스트 추출 |
| 이미지 추출 | BinData에서 이미지 추출 |
| CLI extract 명령 | 텍스트/IR 출력 |

### Phase 3: LLM 프로바이더 (Stage 2)

**목표**: LLM 기반 포맷팅 구현

| 작업 | 설명 |
|------|------|
| OpenAI 프로바이더 | GPT-4o, GPT-4o-mini 지원 |
| Anthropic 프로바이더 | Claude 3.5 Sonnet 지원 |
| 포맷팅 프롬프트 | 최적화된 시스템/유저 프롬프트 |
| CLI convert 명령 | 전체 파이프라인 |

### Phase 4: 추가 프로바이더 및 HWP 5.x

**목표**: 추가 LLM 및 HWP 5.x 지원

| 작업 | 설명 |
|------|------|
| Gemini 프로바이더 | Google Gemini 지원 |
| Ollama 프로바이더 | 로컬 LLM 지원 |
| HWP 5.x 파서 | mscfb 활용 |
| 기본 렌더러 | LLM 없이 기본 변환 |

### Phase 5: 배포 및 안정화

**목표**: 바이너리 릴리스, 문서화, 테스트 강화

| 작업 | 설명 |
|------|------|
| 바이너리 릴리스 | GoReleaser로 크로스 플랫폼 빌드 |
| 문서화 | README, API 문서, 예제 |
| 테스트 강화 | 다양한 HWP 샘플로 테스트 |
| CI/CD 설정 | GitHub Actions |

---

## 9. 성공 지표

| 지표 | 목표 |
|------|------|
| 지원 포맷 | HWPX, HWP 5.x |
| 지원 LLM | OpenAI, Anthropic, Gemini, Ollama |
| 텍스트 추출 정확도 | 99% |
| LLM 포맷팅 품질 | 사용자 만족도 90% |
| 바이너리 다운로드 | 출시 후 3개월 내 1,000회 |
| GitHub Stars | 출시 후 6개월 내 100개 |

---

## 10. 위험 및 대응

| 위험 | 영향 | 대응 |
|------|------|------|
| HWP 5.x 바이너리 구조 복잡성 | 개발 지연 | HWPX 우선 지원, 기존 파서 참고 |
| LLM API 비용 | 운영 비용 | Ollama 로컬 옵션 제공, 토큰 사용량 표시 |
| LLM API 가용성 | 서비스 중단 | 다중 프로바이더 지원, 폴백 옵션 |
| LLM 출력 일관성 | 품질 변동 | 프롬프트 최적화, 재시도 로직 |
| 다양한 HWP 버전 호환성 | 변환 실패 | 점진적 버전 지원, 사용자 피드백 수집 |

---

## 11. 참고 자료

- [HWP 포맷 조사 보고서](hwp-format-research.md)
- [기존 솔루션 조사](existing-solutions-research.md)
- [기술 스택](tech-stack.md)
- [mscfb (Go OLE parser)](https://github.com/richardlehane/mscfb)
- [cobra (Go CLI)](https://github.com/spf13/cobra)
- [go-openai](https://github.com/sashabaranov/go-openai)
- [go-anthropic](https://github.com/liushuangls/go-anthropic)
