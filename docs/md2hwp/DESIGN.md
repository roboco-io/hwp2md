# md2hwp Design Document

> Reverse pipeline: Fill HWPX government templates with business plan content.

## Problem

Korean government funding applications require submission in HWP format with strict template compliance. Manual form-filling is tedious and error-prone. md2hwp automates this by injecting structured content into HWPX templates while preserving all formatting.

## Target Workflow

```
1. User uploads HWPX template → Claude analyzes structure
2. User discusses business plan → content finalized
3. Claude generates fill_plan.json → fill_hwpx.py injects content
4. User downloads completed HWPX → submits to government
```

## Architecture

```
fill_plan.json ──→ fill_hwpx.py ──→ output.hwpx
                        │
                   template.hwpx
                   (ZIP + XML)
```

### Core Engine: `tools/md2hwp/fill_hwpx.py`

Single Python script using `zipfile + lxml` for direct XML manipulation.

**Why not python-hwpx?** It misses text inside table cells. Direct XML parsing captures ALL `<hp:t>` elements.

### Replacement Strategies

| Strategy | Purpose | fill_plan.json key |
|----------|---------|-------------------|
| Simple | Exact text match → replace | `simple_replacements` |
| Section | Guide text → actual content (cell-scoped) | `section_replacements` |
| Table Cell | Label cell → adjacent value cell | `table_cell_fills` |
| Multi-paragraph | Inject multiple paragraphs into a cell | `multi_paragraph_fills` |

### Processing Order

1. Simple replacements (longest-first to prevent partial matches)
2. Section replacements (clears entire cell of guide text)
3. Table cell fills (cellAddr-based lookup with flat-scan fallback)
4. Multi-paragraph fills (creates new `<hp:p>` elements)

---

## fill_plan.json Schema

```json
{
  "template_file": "/absolute/path/to/template.hwpx",
  "output_file": "/absolute/path/to/output.hwpx",

  "simple_replacements": [
    {
      "find": "OO기업",
      "replace": "테스트기업 주식회사",
      "occurrence": 1
    }
  ],

  "section_replacements": [
    {
      "section_id": "1-1",
      "guide_text_prefix": "※ 과거 폐업 원인을",
      "content": "Actual content replacing the guide text.",
      "clear_cell": true
    }
  ],

  "table_cell_fills": [
    {
      "find_label": "과제명",
      "value": "AI 자세분석 플랫폼",
      "target_offset": {"col": 1, "row": 0}
    }
  ],

  "multi_paragraph_fills": [
    {
      "section_id": "1-1",
      "guide_text_prefix": "※ 과거 폐업 원인을",
      "paragraphs": [
        "First paragraph of content...",
        "Second paragraph of content...",
        "Third paragraph of content..."
      ]
    }
  ]
}
```

### Field Reference

#### simple_replacements

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `find` | string | yes | - | Exact text to find in `<hp:t>` elements |
| `replace` | string | yes | - | Replacement text |
| `occurrence` | int | no | all | Limit to N replacements |

#### section_replacements

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `section_id` | string | no | "?" | Section identifier for logging |
| `guide_text_prefix` | string | yes | - | Prefix of guide text to find |
| `content` | string | yes | - | Content to replace guide text |
| `clear_cell` | bool | no | true | Clear all other runs/paragraphs in the cell |

#### table_cell_fills

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `find_label` | string | yes | - | Label text in the label cell |
| `value` | string | yes | - | Value to write in the target cell |
| `target_offset` | object | no | `{"col":1,"row":0}` | Column/row offset from label cell |

#### multi_paragraph_fills

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `section_id` | string | no | "?" | Section identifier for logging |
| `guide_text_prefix` | string | yes | - | Prefix to locate the target cell |
| `paragraphs` | string[] | yes | - | Array of paragraph texts |

---

## CLI Interface

```bash
# Inspection
python3 tools/md2hwp/fill_hwpx.py --inspect <template.hwpx>           # List all text elements
python3 tools/md2hwp/fill_hwpx.py --inspect <template.hwpx> -q "text"  # Search text elements
python3 tools/md2hwp/fill_hwpx.py --inspect-tables <template.hwpx>     # Show table structure
python3 tools/md2hwp/fill_hwpx.py --analyze <template.hwpx>            # Extract fillable field schema

# Filling
python3 tools/md2hwp/fill_hwpx.py <fill_plan.json>
python3 tools/md2hwp/fill_hwpx.py <fill_plan.json> -o <output.hwpx>

# Environment
MD2HWP_EVENT_FILE=/tmp/events.jsonl  # Enable SSE event logging
```

---

## HWPX XML Reference

### Element Hierarchy

```xml
<hs:sec>                          <!-- Section root -->
  <hp:p paraPrIDRef="N">          <!-- Paragraph -->
    <hp:run charPrIDRef="N">      <!-- Text run (styling) -->
      <hp:t>Text content</hp:t>   <!-- Text element -->
    </hp:run>
    <hp:linesegarray>...</hp:linesegarray>  <!-- Line metrics (always present) -->
  </hp:p>
</hs:sec>
```

### Table Cell Hierarchy

```xml
<hp:tbl rowCnt="N" colCnt="N">
  <hp:tr>
    <hp:tc>
      <hp:subList>
        <hp:p paraPrIDRef="N" styleIDRef="N">
          <hp:run charPrIDRef="N">
            <hp:t>Cell text</hp:t>
          </hp:run>
          <hp:linesegarray>...</hp:linesegarray>
        </hp:p>
      </hp:subList>
      <hp:cellAddr colAddr="0" rowAddr="0"/>
      <hp:cellSpan colSpan="1" rowSpan="1"/>
      <hp:cellSz width="N" height="N"/>
    </hp:tc>
  </hp:tr>
</hp:tbl>
```

### Namespaces

| Prefix | URI |
|--------|-----|
| `hp` | `http://www.hancom.co.kr/hwpml/2011/paragraph` |
| `hs` | `http://www.hancom.co.kr/hwpml/2011/section` |
| `hc` | `http://www.hancom.co.kr/hwpml/2011/core` |
| `hh` | `http://www.hancom.co.kr/hwpml/2011/head` |
| `hp10` | `http://www.hancom.co.kr/hwpml/2016/paragraph` |

---

## Target Template: 재도전성공패키지

Primary test template: `testdata/hwpx_20260302_200059.hwpx`

### Structure

- **28 tables**, 382 `<hp:t>` text elements
- 7 page limit (excl. TOC + appendix)

### Sections

| Section | Tables | Content Type |
|---------|--------|-------------|
| 과제 개요 | T5 (3x2) | 과제명, 기업명, 아이템 개요 |
| 폐업 이력 | T6 (14x4) | Repeatable rows (max 3) |
| 1. 문제인식 | T7-T9 | Guide text → multi-paragraph |
| 2. 실현가능성 | T10-T12 | Guide text → multi-paragraph |
| 3. 성장전략 | T13-T18 | Guide text + timeline table + budget table |
| 4. 기업 구성 | T19-T22 | Team table + staffing plan |
| 가점/면제 | T23-T28 | Checklist + evidence placeholders |

### Complex Tables

- **T6** (폐업이력 14x4): colspan patterns, 3 repeatable company rows
- **T16** (실현일정 5x4): 4 data rows with deliverables
- **T18** (사업비 9x6): 3-level rowspan header, budget items
- **T21** (팀구성 5x8): Personnel roster with colspan
- **T24** (가점체크 14x4): Grouped checkbox items with rowspan

### Template Constraints

| Constraint | Value |
|-----------|-------|
| Page limit | 7 pages (excl. TOC + appendix) |
| Budget max | 100,000,000 KRW |
| Gov support | ≤75% of total |
| Cash contribution | ≥5% of total |
| In-kind contribution | ≤20% of total |
| Closure history | Max 3 companies (most recent) |
| PII masking | Required (name, gender, DOB, university) |

---

## Known Gaps & Roadmap

### P0: Must-have (blocking basic operation)

| ID | Gap | Solution |
|----|-----|---------|
| P0-1 | section_replacements only replaces first `<hp:t>`, orphans rest | Cell-scoped clearing: replace first, remove other runs/paragraphs |
| P0-2 | table_cell_fills skips empty target cells (no `<hp:t>`) | cellAddr-based lookup + create `<hp:t>` in empty runs |
| P0-3 | --inspect lacks table/cell context | Add `[T2 R3 C1]` context + `--inspect-tables` mode |

### P1: Quality improvements

| ID | Gap | Solution |
|----|-----|---------|
| P1-4 | No multi-paragraph injection | New `multi_paragraph_fills` strategy, clones `<hp:p>` structure |
| P1-5 | No template schema extraction | `--analyze` mode outputs structured JSON of fillable fields |

### P2: Nice-to-have

| ID | Gap | Solution |
|----|-----|---------|
| P2-6 | No content validation | `--validate` mode checks char limits, budget math, required fields |

---

## Helper Functions (Implementation Reference)

These shared helpers are used across all strategies:

```python
_build_parent_map(tree) -> dict          # Element -> parent mapping
_get_ancestor(elem, tag, parent_map)     # Walk up to find ancestor (tc, tbl, etc.)
_clear_cell_except(tc, keep_elem, pm)    # Remove all runs/paragraphs except one
_find_cell_by_addr(tbl, col, row)        # Find cell by cellAddr coordinates
_set_cell_text(tc, text)                 # Set cell text, create <hp:t> if needed
_get_table_index(tree, tbl)              # Get table ordinal in document
```

---

## Testing Strategy

### Unit Tests (per function)

Each replacement strategy must have tests for:
- Normal case: text found and replaced
- Empty cell: target cell has no `<hp:t>`
- Multi-run guide text: guide text spans multiple `<hp:run>` elements
- Missing text: `find` text not in template (warning, no crash)
- Edge cases: colspan/rowspan cells, nested tables

### Integration Tests

- Full fill_plan.json → output.hwpx → hwp2md reverse → verify content
- Use `testdata/hwpx_20260302_200059.hwpx` as primary fixture

### Test Fixtures Location

```
testdata/
├── hwpx_20260302_200059.hwpx        # Primary template
├── md2hwp-outputs/                   # Test output directory
└── fill_plans/                       # Test fill_plan.json files (to create)
    ├── test_simple.json
    ├── test_section.json
    ├── test_table_cell.json
    └── test_multi_paragraph.json
```
