# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.1] - 2025-01-10

### Added
- Upstage Document Parse as alternative Stage 1 parser (`--parser upstage`, `HWP2MD_PARSER=upstage`)
  - Direct HWP/HWPX file support via Upstage API
  - OCR-based layout recognition for complex documents
  - Markdown output directly from API for better table/list rendering
- `RawMarkdown` field in IR for external parser passthrough
- Stage 1 parser comparison examples in README

## [0.2.0] - 2025-01-10

### Added
- Upstage Solar LLM provider (`solar-pro`, `solar-mini`)
- Private tenancy support via `--base-url` flag and `HWP2MD_BASE_URL` env
  - AWS Bedrock, Azure OpenAI, local server endpoints
- Auto-detect LLM provider from model name (claude-* → anthropic, gpt-* → openai, solar-* → upstage, etc.)
- HWPX-Markdown differences documentation (`docs/hwpx-markdown-differences.md`)
- Claude skill for release automation
- Mermaid architecture diagram in README

### Changed
- Simplified configuration by removing `HWP2MD_PROVIDER` environment variable
- Users only need to set `HWP2MD_MODEL` for LLM provider selection
- `convert` command can be omitted (default command)
- Go install prioritized over binary download in documentation
- CI skips documentation-only changes

## [0.1.0] - 2024-01-10

### Added
- Initial release of hwp2markdown CLI tool
- 2-stage pipeline architecture:
  - Stage 1: HWPX parser with intermediate representation (IR)
  - Stage 2: LLM-based Markdown formatting (optional)
- HWPX format support (XML-based HWP format from Hancom Office 2014+)
- Multiple LLM providers:
  - Anthropic Claude (default)
  - OpenAI GPT
  - Google Gemini
  - Ollama (local)
- Table parsing with cell span support
- Nested table handling (converted to inline text)
- Legal content tables conversion to list format
- Full-width/half-width space handling (`<hp:fwSpace/>`, `<hp:hwSpace/>`)
- Info-box table detection and formatting
- CLI commands:
  - `convert` - Convert HWP/HWPX to Markdown
  - `extract` - Extract IR in JSON/text format
  - `providers` - List available LLM providers
  - `config` - Configuration management
- Environment variable configuration
- YAML configuration file support
- Git hooks for pre-commit linting
- GitHub Actions CI/CD pipeline
- goreleaser configuration for cross-platform builds

### Technical Details
- Written in Go 1.24+
- Uses Cobra for CLI framework
- Modular LLM provider architecture
- Comprehensive test coverage

[Unreleased]: https://github.com/roboco-io/hwp2markdown/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/roboco-io/hwp2markdown/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/roboco-io/hwp2markdown/releases/tag/v0.1.0
