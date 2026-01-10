# hwp2md 기술 스택

## 1. 개요

hwp2md은 크로스 플랫폼 CLI 도구로, Windows, macOS, Linux에서 모두 동작해야 한다. 이 문서는 기술 스택 선정 과정과 결정 사항을 정리한다.

---

## 2. 언어 선택

### 후보 비교

| 언어 | 크로스 플랫폼 | 바이너리 배포 | 바이너리 크기 | 개발 속도 | 동시성 |
|------|--------------|---------------|---------------|-----------|--------|
| **Go** | O | 네이티브 바이너리 | 5-15MB | 빠름 | 고루틴 내장 |
| **Rust** | O | 네이티브 바이너리 | 2-10MB | 느림 | 비동기 지원 |
| **Python** | O | PyInstaller (30-50MB) | 큼 | 빠름 | GIL 제약 |
| **TypeScript** | O | pkg/nexe (50MB+) | 매우 큼 | 빠름 | 이벤트 루프 |

### 결정: Go

**이유:**
1. **단일 바이너리 배포**: 의존성 없이 단일 실행 파일로 배포 가능
2. **크로스 컴파일**: `GOOS`/`GOARCH` 설정만으로 모든 플랫폼 빌드
3. **빠른 빌드 속도**: 대규모 프로젝트도 수 초 내 빌드
4. **간결한 문법**: 학습 곡선이 낮고 코드 가독성 높음
5. **표준 라이브러리**: `archive/zip`, `encoding/xml` 등 풍부한 내장 기능
6. **동시성**: 고루틴으로 배치 처리 시 성능 최적화 용이

---

## 3. Go 버전

### 결정: Go 1.21+

**이유:**
- `slices`, `maps` 패키지 표준 라이브러리 포함
- 제네릭 기능 안정화
- `slog` 구조화된 로깅
- 향상된 PGO (Profile Guided Optimization)

---

## 4. 핵심 의존성

### 4.1 파일 파싱

| 패키지 | 용도 | 라이선스 |
|--------|------|----------|
| `archive/zip` (표준) | HWPX ZIP 압축 해제 | BSD |
| `encoding/xml` (표준) | HWPX XML 파싱 | BSD |
| `github.com/richardlehane/mscfb` | HWP 5.x OLE/CFBF 파싱 | Apache-2.0 |

#### HWPX 파싱 (표준 라이브러리)

```go
import (
    "archive/zip"
    "encoding/xml"
)

func parseHWPX(path string) (*Document, error) {
    r, err := zip.OpenReader(path)
    if err != nil {
        return nil, err
    }
    defer r.Close()

    for _, f := range r.File {
        if f.Name == "Contents/content.hpf" {
            rc, _ := f.Open()
            defer rc.Close()
            // XML 파싱
        }
    }
    return &Document{}, nil
}
```

#### HWP 5.x OLE 파싱

```go
import "github.com/richardlehane/mscfb"

func parseHWP5(path string) (*Document, error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    doc, err := mscfb.New(file)
    if err != nil {
        return nil, err
    }

    for entry, err := doc.Next(); err == nil; entry, err = doc.Next() {
        // FileHeader, DocInfo, BodyText/Section0 등 처리
        fmt.Println(entry.Name)
    }
    return &Document{}, nil
}
```

### 4.2 CLI 프레임워크

| 패키지 | 용도 | 선택 이유 |
|--------|------|-----------|
| `github.com/spf13/cobra` | CLI 인터페이스 | 업계 표준, 서브커맨드 지원 |
| `github.com/spf13/pflag` | POSIX 플래그 | cobra 의존성 |

#### cobra 예시

```go
package cmd

import (
    "github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
    Use:   "hwp2md",
    Short: "HWP/HWPX 문서를 Markdown으로 변환",
    Long:  `HWP(한글 워드프로세서) 문서를 Markdown으로 변환하는 CLI 도구입니다.`,
}

var convertCmd = &cobra.Command{
    Use:   "convert [input]",
    Short: "HWP/HWPX 파일을 Markdown으로 변환",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        input := args[0]
        output, _ := cmd.Flags().GetString("output")
        return convert(input, output)
    },
}

func init() {
    convertCmd.Flags().StringP("output", "o", "", "출력 파일")
    convertCmd.Flags().String("extract-images", "", "이미지 추출 디렉토리")
    rootCmd.AddCommand(convertCmd)
}
```

### 4.3 LLM Provider

| 패키지 | 용도 | 라이선스 |
|--------|------|----------|
| `github.com/sashabaranov/go-openai` | OpenAI API 클라이언트 | Apache-2.0 |
| `github.com/anthropics/anthropic-sdk-go` | Anthropic API 클라이언트 | MIT |
| `github.com/google/generative-ai-go` | Google Gemini API 클라이언트 | Apache-2.0 |

### 4.4 기타 유틸리티

| 패키지 | 용도 | 라이선스 |
|--------|------|----------|
| `golang.org/x/text/encoding/korean` | EUC-KR 인코딩 처리 | BSD |
| `github.com/fatih/color` | 컬러 터미널 출력 (선택적) | MIT |
| `gopkg.in/yaml.v3` | 설정 파일 파싱 | Apache-2.0 |

### 4.5 전체 의존성 (go.mod)

```go
module github.com/roboco-io/hwp2md

go 1.21

require (
    github.com/anthropics/anthropic-sdk-go v0.2.0
    github.com/google/generative-ai-go v0.18.0
    github.com/richardlehane/mscfb v1.0.4
    github.com/sashabaranov/go-openai v1.32.0
    github.com/spf13/cobra v1.8.0
    golang.org/x/text v0.14.0
    gopkg.in/yaml.v3 v3.0.1
)
```

---

## 5. 바이너리 배포

### 5.1 크로스 컴파일

Go는 단일 명령으로 모든 플랫폼용 바이너리 생성 가능:

```bash
# Windows
GOOS=windows GOARCH=amd64 go build -o hwp2md-windows-x64.exe

# macOS Intel
GOOS=darwin GOARCH=amd64 go build -o hwp2md-macos-x64

# macOS Apple Silicon
GOOS=darwin GOARCH=arm64 go build -o hwp2md-macos-arm64

# Linux
GOOS=linux GOARCH=amd64 go build -o hwp2md-linux-x64
```

### 5.2 빌드 최적화

```bash
# 릴리스 빌드 (심볼 제거, 크기 최적화)
go build -ldflags="-s -w" -o hwp2md

# 버전 정보 삽입
go build -ldflags="-s -w -X main.version=1.0.0" -o hwp2md
```

### 5.3 배포 대상

| 플랫폼 | 아키텍처 | 파일명 | 예상 크기 |
|--------|----------|--------|-----------|
| Windows | x64 | `hwp2md-windows-x64.exe` | ~8MB |
| macOS | x64 | `hwp2md-macos-x64` | ~8MB |
| macOS | arm64 | `hwp2md-macos-arm64` | ~8MB |
| Linux | x64 | `hwp2md-linux-x64` | ~8MB |

---

## 6. 프로젝트 구조

```
hwp2md/
├── cmd/
│   └── hwp2md/
│       └── main.go              # 진입점
├── internal/
│   ├── cli/
│   │   ├── root.go              # cobra 루트 커맨드
│   │   ├── convert.go           # convert 서브커맨드
│   │   ├── extract.go           # extract 서브커맨드
│   │   ├── providers.go         # providers 서브커맨드
│   │   └── config.go            # config 서브커맨드
│   ├── parser/
│   │   ├── parser.go            # 파서 인터페이스
│   │   ├── hwpx.go              # HWPX 파서
│   │   ├── hwp5.go              # HWP 5.x 파서
│   │   └── detector.go          # 포맷 감지
│   ├── ir/
│   │   ├── ir.go                # Intermediate Representation 정의
│   │   ├── paragraph.go         # 문단 IR
│   │   ├── table.go             # 테이블 IR
│   │   └── image.go             # 이미지 IR
│   ├── llm/
│   │   ├── provider.go          # LLM Provider 인터페이스
│   │   ├── registry.go          # Provider Registry
│   │   ├── openai/
│   │   │   └── openai.go        # OpenAI Provider
│   │   ├── anthropic/
│   │   │   └── anthropic.go     # Anthropic Provider
│   │   ├── gemini/
│   │   │   └── gemini.go        # Google Gemini Provider
│   │   └── ollama/
│   │       └── ollama.go        # Ollama Provider (로컬)
│   ├── config/
│   │   ├── config.go            # 설정 관리
│   │   └── loader.go            # 설정 파일 로더
│   └── renderer/
│       ├── renderer.go          # 렌더러 인터페이스
│       ├── markdown.go          # Markdown 렌더러
│       └── text.go              # Plain Text 렌더러
├── pkg/
│   └── hwp2md/
│       └── convert.go           # 공개 API
├── testdata/
│   ├── sample.hwpx
│   ├── sample.hwp
│   └── expected.md              # 테스트 기대 결과
├── docs/
│   ├── hwp-format-research.md
│   ├── existing-solutions-research.md
│   ├── PRD.md
│   └── tech-stack.md
├── go.mod
├── go.sum
├── Makefile
├── README.md
├── LICENSE
└── .github/
    └── workflows/
        ├── test.yml
        └── release.yml
```

---

## 7. 빌드 및 개발

### 7.1 Makefile

```makefile
.PHONY: build test lint clean release

VERSION ?= $(shell git describe --tags --always --dirty)
LDFLAGS := -ldflags="-s -w -X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o bin/hwp2md ./cmd/hwp2md

test:
	go test -v -race -cover ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/ dist/

release:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/hwp2md-windows-x64.exe ./cmd/hwp2md
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/hwp2md-macos-x64 ./cmd/hwp2md
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/hwp2md-macos-arm64 ./cmd/hwp2md
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/hwp2md-linux-x64 ./cmd/hwp2md
```

### 7.2 개발 환경 설정

```bash
# 저장소 클론
git clone https://github.com/roboco-io/hwp2md.git
cd hwp2md

# 의존성 다운로드
go mod download

# 빌드
make build

# 테스트
make test

# 린트 (golangci-lint 필요)
make lint
```

---

## 8. CI/CD

### 8.1 테스트 워크플로

```yaml
# .github/workflows/test.yml
name: Test

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ${{ matrix.os }}
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
        go-version: ["1.21", "1.22"]

    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ matrix.go-version }}

      - name: Install dependencies
        run: go mod download

      - name: Lint
        uses: golangci/golangci-lint-action@v4
        with:
          version: latest

      - name: Test
        run: go test -v -race -coverprofile=coverage.out ./...

      - name: Upload coverage
        uses: codecov/codecov-action@v4
        with:
          file: coverage.out
```

### 8.2 릴리스 워크플로

```yaml
# .github/workflows/release.yml
name: Release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: "1.22"

      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v5
        with:
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### 8.3 GoReleaser 설정

```yaml
# .goreleaser.yml
project_name: hwp2md

builds:
  - id: hwp2md
    main: ./cmd/hwp2md
    binary: hwp2md
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w
      - -X main.version={{.Version}}

archives:
  - id: default
    format: tar.gz
    format_overrides:
      - goos: windows
        format: zip
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"

checksum:
  name_template: "checksums.txt"

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^chore:"

release:
  github:
    owner: roboco-io
    name: hwp2md
```

---

## 9. 코드 품질

### 9.1 golangci-lint 설정

```yaml
# .golangci.yml
run:
  timeout: 5m

linters:
  enable:
    - errcheck
    - gosimple
    - govet
    - ineffassign
    - staticcheck
    - unused
    - gofmt
    - goimports
    - misspell
    - revive

linters-settings:
  revive:
    rules:
      - name: exported
        disabled: false
```

### 9.2 pre-commit 설정

```yaml
# .pre-commit-config.yaml
repos:
  - repo: https://github.com/golangci/golangci-lint
    rev: v1.55.2
    hooks:
      - id: golangci-lint

  - repo: local
    hooks:
      - id: go-test
        name: go test
        entry: go test ./...
        language: system
        pass_filenames: false
```

---

## 10. LLM Provider 아키텍처

### 10.1 2단계 파이프라인

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         2-Stage Pipeline                                 │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  Stage 1: Text Extraction                                               │
│  ┌─────────┐    ┌────────┐    ┌─────────────────────────┐              │
│  │ HWP/X   │───▶│ Parser │───▶│ Intermediate            │              │
│  │ File    │    │        │    │ Representation (JSON)   │              │
│  └─────────┘    └────────┘    └─────────────────────────┘              │
│                                           │                             │
│                                           ▼                             │
│  Stage 2: LLM Formatting                                                │
│  ┌─────────────────────────┐    ┌─────────────────────┐                │
│  │ IR + Formatting Hints   │───▶│ LLM Provider        │                │
│  │                         │    │ (OpenAI/Anthropic/  │                │
│  │                         │    │  Gemini/Ollama)     │                │
│  └─────────────────────────┘    └─────────────────────┘                │
│                                           │                             │
│                                           ▼                             │
│                               ┌─────────────────────┐                   │
│                               │ Markdown Output     │                   │
│                               └─────────────────────┘                   │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 10.2 LLM Provider 인터페이스

```go
// internal/llm/provider.go

package llm

import "context"

// LLMProvider는 모든 LLM 서비스가 구현해야 하는 인터페이스
type LLMProvider interface {
    // Name은 provider 식별자를 반환
    Name() string

    // Format은 IR을 받아 Markdown으로 변환
    Format(ctx context.Context, ir *IntermediateRepresentation, opts FormatOptions) (*FormatResult, error)

    // Validate는 설정 유효성 검사
    Validate() error
}

// IntermediateRepresentation은 파서 출력물
type IntermediateRepresentation struct {
    Paragraphs   []Paragraph   `json:"paragraphs"`
    TableRegions []TableRegion `json:"table_regions"`
    Images       []ImageRef    `json:"images"`
    Lists        []ListBlock   `json:"lists"`
    Metadata     Metadata      `json:"metadata"`
}

// FormatOptions는 LLM 포맷팅 옵션
type FormatOptions struct {
    Language    string  // 출력 언어
    MaxTokens   int     // 최대 토큰
    Temperature float64 // 창의성 수준
    Prompt      string  // 커스텀 프롬프트 (선택)
}

// FormatResult는 LLM 포맷팅 결과
type FormatResult struct {
    Markdown    string `json:"markdown"`
    TokensUsed  int    `json:"tokens_used"`
    Model       string `json:"model"`
}
```

### 10.3 Provider Registry

```go
// internal/llm/registry.go

package llm

import (
    "fmt"
    "sync"
)

// ProviderRegistry는 LLM Provider를 관리하는 중앙 레지스트리
type ProviderRegistry struct {
    mu        sync.RWMutex
    providers map[string]LLMProvider
}

// NewRegistry는 새 레지스트리 생성
func NewRegistry() *ProviderRegistry {
    return &ProviderRegistry{
        providers: make(map[string]LLMProvider),
    }
}

// Register는 새 provider 등록
func (r *ProviderRegistry) Register(p LLMProvider) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.providers[p.Name()] = p
}

// Get은 이름으로 provider 조회
func (r *ProviderRegistry) Get(name string) (LLMProvider, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    p, ok := r.providers[name]
    if !ok {
        return nil, fmt.Errorf("provider not found: %s", name)
    }
    return p, nil
}

// List는 등록된 모든 provider 이름 반환
func (r *ProviderRegistry) List() []string {
    r.mu.RLock()
    defer r.mu.RUnlock()
    names := make([]string, 0, len(r.providers))
    for name := range r.providers {
        names = append(names, name)
    }
    return names
}
```

### 10.4 Provider 구현 예시 (OpenAI)

```go
// internal/llm/openai/openai.go

package openai

import (
    "context"
    "github.com/sashabaranov/go-openai"
    "github.com/roboco-io/hwp2md/internal/llm"
)

type OpenAIProvider struct {
    client *openai.Client
    model  string
}

func New(apiKey, model string) *OpenAIProvider {
    return &OpenAIProvider{
        client: openai.NewClient(apiKey),
        model:  model,
    }
}

func (p *OpenAIProvider) Name() string {
    return "openai"
}

func (p *OpenAIProvider) Validate() error {
    if p.client == nil {
        return fmt.Errorf("OpenAI client not initialized")
    }
    return nil
}

func (p *OpenAIProvider) Format(ctx context.Context, ir *llm.IntermediateRepresentation, opts llm.FormatOptions) (*llm.FormatResult, error) {
    prompt := buildPrompt(ir, opts)

    resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: p.model,
        Messages: []openai.ChatCompletionMessage{
            {Role: "system", Content: systemPrompt},
            {Role: "user", Content: prompt},
        },
        MaxTokens:   opts.MaxTokens,
        Temperature: float32(opts.Temperature),
    })
    if err != nil {
        return nil, err
    }

    return &llm.FormatResult{
        Markdown:   resp.Choices[0].Message.Content,
        TokensUsed: resp.Usage.TotalTokens,
        Model:      p.model,
    }, nil
}
```

### 10.5 설정 파일

```yaml
# ~/.hwp2md/config.yaml

default_provider: anthropic

providers:
  openai:
    api_key: ${OPENAI_API_KEY}
    model: gpt-4o-mini
    max_tokens: 4096

  anthropic:
    api_key: ${ANTHROPIC_API_KEY}
    model: claude-sonnet-4-20250514
    max_tokens: 4096

  gemini:
    api_key: ${GOOGLE_API_KEY}
    model: gemini-1.5-flash
    max_tokens: 4096

  ollama:
    endpoint: http://localhost:11434
    model: llama3.2
    max_tokens: 4096

format:
  temperature: 0.3
  language: ko
```

---

## 11. 요약

| 항목 | 선택 |
|------|------|
| 언어 | Go 1.21+ |
| CLI 프레임워크 | cobra |
| HWPX 파싱 | archive/zip + encoding/xml (표준) |
| HWP 5.x 파싱 | mscfb |
| 한글 인코딩 | golang.org/x/text/encoding/korean |
| LLM (OpenAI) | go-openai |
| LLM (Anthropic) | anthropic-sdk-go |
| LLM (Gemini) | generative-ai-go |
| 설정 파일 | yaml.v3 |
| 린터 | golangci-lint |
| 테스트 | go test (표준) |
| 릴리스 | GoReleaser |
| CI/CD | GitHub Actions |

### Python 대비 Go의 장점

| 항목 | Python | Go |
|------|--------|-----|
| 바이너리 크기 | 30-50MB (PyInstaller) | 5-15MB |
| 의존성 | 런타임 필요 또는 번들링 | 없음 (단일 바이너리) |
| 시작 시간 | 느림 | 즉시 |
| 크로스 컴파일 | 복잡 | `GOOS`/`GOARCH`로 간단 |
| 동시성 | GIL 제약 | 고루틴 |
