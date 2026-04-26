# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
make build          # Build binary to bin/hwp2md
make test           # Run all tests with race detection and coverage
make lint           # Run golangci-lint
make fmt            # Format code
make tidy           # Run go mod tidy

# Run a single test
go test -v -run TestName ./internal/parser/hwpx/

# E2E tests
make test-e2e
```

## Architecture

hwp2md uses a **2-stage pipeline** to convert HWP/HWPX documents to Markdown:

```
HWP/HWPX → Stage 1 (Parser) → IR → Stage 2 (LLM, optional) → Markdown
```

### Stage 1: Parser (`internal/parser/`)
- **HWPX Parser** (`hwpx/parser.go`): Native XML-based parser for HWPX format
- **HWP5 Parser** (`hwp5/parser.go`): Native binary parser for HWP 5.x format (OLE2/CFB container)
- **Upstage Parser** (`upstage/upstage.go`): Optional external API parser, outputs `RawMarkdown` directly (bypasses IR conversion)

### IR - Intermediate Representation (`internal/ir/`)
- `Document`: Root container with `Metadata`, `Content` (blocks), and optional `RawMarkdown`
- Block types: `Paragraph`, `Table`, `Image`, `List`
- Upstage parser stores markdown directly in `doc.RawMarkdown`, skipping block-level IR

### Stage 2: LLM Providers (`internal/llm/`)
- Common interface: `Provider` with `Format(ctx, doc, opts)` method
- Providers: `anthropic/`, `openai/`, `gemini/`, `upstage/`, `ollama/`
- Model name auto-detection: `claude-*` → Anthropic, `gpt-*` → OpenAI, etc.

### CLI (`internal/cli/`)
- Entry point: `cmd/hwp2md/main.go`
- Commands: `convert` (default), `extract`, `config`, `providers`
- `convert.go`: Main conversion logic, parser selection, LLM formatting

## Key Files

| File | Purpose |
|------|---------|
| `internal/cli/convert.go` | Main conversion pipeline, parser/LLM orchestration |
| `internal/parser/hwpx/parser.go` | HWPX XML parsing, table/cell span handling |
| `internal/parser/hwp5/parser.go` | HWP5 OLE2 parsing, main entry point |
| `internal/parser/hwp5/section.go` | HWP5 section parsing, table/paragraph extraction |
| `internal/parser/hwp5/text.go` | HWP5 UTF-16LE text extraction with control chars |
| `internal/ir/ir.go` | IR document structure definitions |
| `internal/llm/provider.go` | LLM provider interface |
| `internal/llm/prompt.go` | System prompts for LLM formatting |

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `HWP2MD_PARSER` | Parser selection: `native` (default), `upstage` |
| `HWP2MD_LLM` | Enable Stage 2: `true` |
| `HWP2MD_MODEL` | Model name (auto-detects provider) |
| `HWP2MD_BASE_URL` | Private API endpoint (Bedrock, Azure, local) |
| `HWP2MD_TIMEOUT` | LLM request timeout (Go duration: `5m`, `300s`, `10m30s`). Empty → provider default |
| `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GOOGLE_API_KEY`, `UPSTAGE_API_KEY` | Provider API keys |

## Conventions

- Korean is the primary language for CLI messages, comments, and documentation
- Cell merge handling: rowspan → `〃`, colspan → empty cell
- Special whitespace elements (`<hp:fwSpace/>`, `<hp:hwSpace/>`) → regular space

## HWP5 Binary Format Notes

- OLE2/CFB container parsed with `github.com/richardlehane/mscfb`
- Records use 4-byte header: TagID (10-bit), Level (10-bit), Size (12-bit)
- Text is UTF-16LE with inline control characters (Extended/Inline use +14 bytes)
- Tables: CTRL_HEADER(" lbt") → TABLE → LIST_HEADER (per cell) → PARA_HEADER/PARA_TEXT
- Compression: zlib or raw deflate (try zlib first, fallback to deflate)
