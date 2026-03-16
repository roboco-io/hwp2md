# AGENTS.md

Instructions for AI coding agents (ChatGPT Codex, etc.) working on this repository.

## Project Overview

hwp2md is a CLI tool for converting HWP/HWPX documents to Markdown (Go) with a reverse pipeline **md2hwp** for filling HWPX templates with content (Python).

## Repository Structure

```
hwp2md/
├── cmd/hwp2md/          # CLI entry point (Go)
├── internal/             # Core Go implementation
│   ├── parser/hwpx/     # HWPX XML parser
│   ├── parser/hwp5/     # HWP5 binary parser
│   ├── ir/              # Intermediate Representation
│   ├── llm/             # LLM provider abstraction
│   ├── formatter/       # Output formatting
│   └── cli/             # CLI commands
├── tools/md2hwp-ui/     # Web preview UI (Python, lower priority)
│   ├── server.py        # HTTP server + SSE
│   └── renderer.py      # HWPX -> HTML converter
├── tools/md2hwp/        # fill_hwpx.py (Python template injection engine)
├── tests/               # E2E tests (Go)
├── testdata/            # Test fixtures
└── docs/                # Technical documentation
    └── md2hwp/          # md2hwp design docs & specs
```

## Build & Test

```bash
# Go (hwp2md core)
make build              # Build binary to bin/hwp2md
make test               # Unit tests with race detection + coverage
make test-e2e           # E2E tests
make lint               # golangci-lint
make fmt                # gofmt

# Python (md2hwp / fill_hwpx.py)
pip install lxml        # Required dependency
python3 tools/md2hwp/fill_hwpx.py --help
python3 tools/md2hwp/fill_hwpx.py --inspect <template.hwpx>
python3 tools/md2hwp/fill_hwpx.py --inspect-tables <template.hwpx>
python3 tools/md2hwp/fill_hwpx.py --analyze <template.hwpx>
python3 tools/md2hwp/fill_hwpx.py <fill_plan.json>
```

## Key Conventions

- **Go code**: Follow golangci-lint rules, `make fmt` before commit
- **Python code**: Follow PEP 8, type hints where practical
- **Commits**: Conventional Commits 1.0.0 (feat/fix/docs/test/refactor/chore)
- **Language**: Korean for user-facing messages, English for code/docs/commits
- **Tests**: TDD workflow - write tests first, then implement

## md2hwp Architecture

See [docs/md2hwp/DESIGN.md](docs/md2hwp/DESIGN.md) for full design.

### fill_hwpx.py Overview

Template injection engine that modifies HWPX (ZIP + XML) files:

- **Input**: `fill_plan.json` with replacement instructions
- **Output**: Modified HWPX file with content injected
- **Preservation**: All formatting (fonts, cell sizes, merge patterns) preserved

### HWPX XML Structure

```
hs:sec (section root)
  hp:p (paragraph)
    hp:run (text run with style reference)
      hp:t (text content)
      hp:tbl (table)
        hp:tr (table row)
          hp:tc (table cell)
            hp:cellAddr (colAddr, rowAddr)
            hp:cellSpan (colSpan, rowSpan)
            hp:subList
              hp:p > hp:run > hp:t
```

### Namespace Map

```python
HWPX_NS = {
    "hp": "http://www.hancom.co.kr/hwpml/2011/paragraph",
    "hs": "http://www.hancom.co.kr/hwpml/2011/section",
    "hc": "http://www.hancom.co.kr/hwpml/2011/core",
    "hh": "http://www.hancom.co.kr/hwpml/2011/head",
}
```

## Working with Issues

Each issue assigned to you will contain:

1. **Context**: Why this change is needed
2. **Spec**: Exact interface/behavior expected
3. **Test fixtures**: Input/output examples in `testdata/` or inline
4. **Acceptance criteria**: What must pass for the PR to be accepted

### Branch Naming

```
codex/<issue-number>-<short-description>
# Example: codex/25-fix-empty-cell-fill
```

### PR Checklist

Before submitting a PR:
- [ ] All existing tests pass (`make test` for Go, pytest for Python)
- [ ] New tests added for new functionality
- [ ] Code formatted (`make fmt` for Go, PEP 8 for Python)
- [ ] Commit messages follow Conventional Commits
- [ ] No new linter warnings

## Important Files Reference

| File | Purpose |
|------|---------|
| `tools/md2hwp/fill_hwpx.py` | Template injection engine (Python) |
| `docs/md2hwp/DESIGN.md` | Architecture & fill_plan.json schema |
| `docs/md2hwp/FILL_PLAN_SCHEMA.md` | JSON schema reference |
| `testdata/hwpx_20260302_200059.hwpx` | Primary test template |
| `internal/parser/hwpx/parser.go` | HWPX parser (Go) |
| `docs/hwpx-schema.md` | HWPX XML format specification |

## Python-specific Notes

- **Python version**: 3.12+ (3.13 removed `cgi` module)
- **Dependencies**: `lxml` only (no Flask, no external frameworks)
- **File size limit**: 800 lines max per file
- **Function size**: 50 lines max
- **Error handling**: Always catch + descriptive error messages
