#!/usr/bin/env python3
"""fill_hwpx.py - Template Injection for HWPX files.

Reads a fill_plan.json and applies text replacements to an HWPX template,
preserving all original formatting (cell sizes, merge patterns, styles).

Uses direct XML manipulation (zipfile + lxml) to handle ALL text elements
including those inside table cells, which python-hwpx's iter_runs() misses.

Usage:
    python3 fill_hwpx.py <fill_plan.json>
    python3 fill_hwpx.py <fill_plan.json> -o <output.hwpx>
    python3 fill_hwpx.py --inspect <template.hwpx>          # List all text runs
    python3 fill_hwpx.py --inspect <template.hwpx> -q <text> # Search for text
    python3 fill_hwpx.py --inspect-tables <template.hwpx>   # Show table structure
    python3 fill_hwpx.py --analyze <template.hwpx>          # Output fillable schema
"""

import json
import sys
import os
import argparse
import re
import shutil
import tempfile
import zipfile
from copy import deepcopy
from pathlib import Path

try:
    from lxml import etree
except ImportError:
    import xml.etree.ElementTree as etree
    print("WARNING: lxml not installed, using stdlib xml.etree (less robust)", file=sys.stderr)

# HWPX namespaces
HWPX_NS = {
    "hp": "http://www.hancom.co.kr/hwpml/2011/paragraph",
    "hs": "http://www.hancom.co.kr/hwpml/2011/section",
    "hc": "http://www.hancom.co.kr/hwpml/2011/core",
    "hh": "http://www.hancom.co.kr/hwpml/2011/head",
    "ha": "http://www.hancom.co.kr/hwpml/2011/app",
    "hp10": "http://www.hancom.co.kr/hwpml/2016/paragraph",
}

HP_T_TAG = f"{{{HWPX_NS['hp']}}}t"
HP_TC_TAG = f"{{{HWPX_NS['hp']}}}tc"
HP_TBL_TAG = f"{{{HWPX_NS['hp']}}}tbl"
HP_P_TAG = f"{{{HWPX_NS['hp']}}}p"
HP_RUN_TAG = f"{{{HWPX_NS['hp']}}}run"
HP_SUBLIST_TAG = f"{{{HWPX_NS['hp']}}}subList"
HP_CELLADDR_TAG = f"{{{HWPX_NS['hp']}}}cellAddr"
HP_CELLSPAN_TAG = f"{{{HWPX_NS['hp']}}}cellSpan"

# Event logging for real-time UI
EVENT_FILE = os.environ.get("MD2HWP_EVENT_FILE")
PLACEHOLDER_PATTERNS = [r"OO+", r"○{2,}"]
NUMERIC_PLACEHOLDER_PATTERN = r"(?<![0-9,])(0{3,})(?:원|년|월|일)?(?!\d)"


def _log_event(event: dict) -> None:
    """Append event to JSONL file for SSE streaming."""
    if not EVENT_FILE:
        return
    with open(EVENT_FILE, "a", encoding="utf-8") as f:
        f.write(json.dumps(event, ensure_ascii=False) + "\n")


def _build_parent_map(tree) -> dict:
    """Build element-to-parent mapping for ancestor traversal."""
    parent_map = {}
    for parent in tree.iter():
        for child in parent:
            parent_map[child] = parent
    return parent_map


def _local_name(tag: str) -> str:
    """Extract local tag name from a namespaced XML tag."""
    if "}" in tag:
        return tag.split("}", 1)[1]
    return tag


def _get_ancestor(elem, tag_local: str, parent_map: dict):
    """Walk up parent chain to find ancestor by local tag name."""
    current = parent_map.get(elem)
    while current is not None:
        tag = current.tag if isinstance(current.tag, str) else ""
        if _local_name(tag) == tag_local:
            return current
        current = parent_map.get(current)
    return None


def _find_cell_by_addr(tbl, col: int, row: int):
    """Find <hp:tc> by its <hp:cellAddr> coordinates."""
    for tr in tbl:
        for tc in tr:
            if tc.tag != HP_TC_TAG:
                continue
            cell_addr = tc.find(f"./{HP_CELLADDR_TAG}")
            if cell_addr is None:
                continue
            try:
                col_addr = int(cell_addr.get("colAddr", "-1"))
                row_addr = int(cell_addr.get("rowAddr", "-1"))
            except ValueError:
                continue
            if col_addr == col and row_addr == row:
                return tc
    return None


def _set_cell_text(tc, text: str) -> None:
    """Set cell text, creating <hp:t> in first <hp:run> when absent."""
    run = tc.find(f".//{HP_RUN_TAG}")
    if run is None:
        sub_list = tc.find(f"./{HP_SUBLIST_TAG}")
        if sub_list is None:
            sub_list = etree.Element(HP_SUBLIST_TAG)
            tc.insert(0, sub_list)
        paragraph = sub_list.find(f"./{HP_P_TAG}")
        if paragraph is None:
            paragraph = etree.Element(HP_P_TAG)
            sub_list.append(paragraph)
        run = etree.Element(HP_RUN_TAG)
        paragraph.append(run)

    text_elem = run.find(f"./{HP_T_TAG}")
    if text_elem is None:
        text_elem = etree.Element(HP_T_TAG)
        run.append(text_elem)

    text_elem.text = text
    for child in list(text_elem):
        text_elem.remove(child)


def _clear_cell_except(tc, keep_elem, parent_map: dict) -> None:
    """Clear a cell except the run/paragraph containing keep_elem."""
    keep_run = _get_ancestor(keep_elem, "run", parent_map)
    keep_paragraph = _get_ancestor(keep_elem, "p", parent_map)
    sub_list = tc.find(f"./{HP_SUBLIST_TAG}")

    if sub_list is not None:
        for paragraph in list(sub_list.findall(f"./{HP_P_TAG}")):
            paragraph_parent = parent_map.get(paragraph)
            if paragraph is not keep_paragraph:
                if paragraph_parent is not None:
                    paragraph_parent.remove(paragraph)
                continue

            for run in list(paragraph.findall(f"./{HP_RUN_TAG}")):
                if run is keep_run:
                    continue
                paragraph.remove(run)

    if keep_run is not None:
        for text_elem in list(keep_run.findall(f"./{HP_T_TAG}")):
            if text_elem is keep_elem:
                continue
            keep_run.remove(text_elem)


def _get_table_index(tree, tbl) -> int:
    """Return ordinal table index in document tree."""
    for idx, candidate in enumerate(tree.findall(f".//{HP_TBL_TAG}")):
        if candidate is tbl:
            return idx
    return -1


def load_plan(plan_path: str) -> dict:
    """Load and validate fill_plan.json."""
    with open(plan_path, encoding="utf-8") as f:
        plan = json.load(f)

    required_keys = ["template_file", "output_file"]
    for key in required_keys:
        if key not in plan:
            raise ValueError(f"Missing required key in fill_plan.json: {key}")

    if not os.path.exists(plan["template_file"]):
        raise FileNotFoundError(f"Template file not found: {plan['template_file']}")

    for r in plan.get("simple_replacements", []):
        if not r.get("find"):
            raise ValueError("simple_replacements: 'find' must be non-empty")

    for r in plan.get("section_replacements", []):
        if not r.get("guide_text_prefix"):
            raise ValueError("section_replacements: 'guide_text_prefix' must be non-empty")

    for r in plan.get("table_cell_fills", []):
        if not r.get("find_label"):
            raise ValueError("table_cell_fills: 'find_label' must be non-empty")

    for r in plan.get("multi_paragraph_fills", []):
        if not r.get("guide_text_prefix"):
            raise ValueError("multi_paragraph_fills: 'guide_text_prefix' must be non-empty")
        if not r.get("paragraphs"):
            raise ValueError("multi_paragraph_fills: 'paragraphs' must be non-empty")

    return plan


def find_section_xmls(hwpx_path: str) -> list[str]:
    """Find all section XML files in HWPX archive."""
    sections = []
    with zipfile.ZipFile(hwpx_path, "r") as zf:
        for name in zf.namelist():
            if name.startswith("Contents/section") and name.endswith(".xml"):
                sections.append(name)
    sections.sort()
    return sections


def get_all_text_elements(tree) -> list:
    """Get all <hp:t> text elements from an XML tree."""
    return tree.findall(f".//{HP_T_TAG}")


def apply_simple_replacements_xml(tree, replacements: list) -> int:
    """Apply exact text match replacements on XML tree.

    Each replacement: {"find": str, "replace": str, "occurrence"?: int}
    Replacements are sorted by find text length (longest first) to prevent
    shorter matches from breaking longer ones.
    """
    total = 0
    text_elements = get_all_text_elements(tree)

    # Sort by find text length descending to avoid partial match conflicts
    sorted_replacements = sorted(replacements, key=lambda r: len(r["find"]), reverse=True)

    for r in sorted_replacements:
        find_text = r["find"]
        replace_text = r["replace"]
        limit = r.get("occurrence")
        count = 0

        for elem_idx, elem in enumerate(text_elements):
            if elem.text and find_text in elem.text:
                elem.text = elem.text.replace(find_text, replace_text, 1)
                count += 1
                _log_event({"type": "replace", "idx": elem_idx, "find": find_text, "replace": replace_text})
                if limit and count >= limit:
                    break

        total += count
        find_display = find_text[:50] + ("..." if len(find_text) > 50 else "")
        replace_display = replace_text[:50] + ("..." if len(replace_text) > 50 else "")
        if count == 0:
            print(f"  WARNING: '{find_display}' not found", file=sys.stderr)
        else:
            print(f"  Replaced '{find_display}' -> '{replace_display}' ({count}x)")

    return total


def apply_section_replacements_xml(tree, replacements: list) -> int:
    """Replace guide text with actual content on XML tree.

    Each replacement: {"section_id": str, "guide_text_prefix": str, "content": str}
    """
    parent_map = _build_parent_map(tree)
    text_elements = get_all_text_elements(tree)
    total = 0

    for r in replacements:
        prefix = r["guide_text_prefix"]
        content = r["content"]
        section_id = r.get("section_id", "?")
        clear_cell = r.get("clear_cell", True)
        replaced = False

        for elem_idx, elem in enumerate(text_elements):
            if elem.text and prefix in elem.text:
                elem.text = content
                for child in list(elem):
                    elem.remove(child)

                if clear_cell:
                    cell = _get_ancestor(elem, "tc", parent_map)
                    if cell is not None:
                        _clear_cell_except(cell, elem, parent_map)

                total += 1
                replaced = True
                _log_event({"type": "replace", "idx": elem_idx, "find": prefix, "replace": content})
                if clear_cell:
                    print(f"  Section {section_id}: replaced guide text (cell cleared)")
                else:
                    print(f"  Section {section_id}: replaced guide text")
                break

        if not replaced:
            prefix_display = prefix[:50] + ("..." if len(prefix) > 50 else "")
            print(
                f"  WARNING: Section {section_id} guide text not found: '{prefix_display}'",
                file=sys.stderr,
            )

    return total


def _find_label_matches(text_elements: list, label: str) -> list[tuple[int, object]]:
    """Find label matches, preferring exact text matches over contains matches."""
    exact_matches = [(i, elem) for i, elem in enumerate(text_elements) if elem.text and elem.text.strip() == label]
    if exact_matches:
        return exact_matches
    return [(i, elem) for i, elem in enumerate(text_elements) if elem.text and label in elem.text]


def _try_celladdr_fill(
    tree,
    label_idx: int,
    label: str,
    value: str,
    label_cell,
    label_addr,
    offset_col: int,
    offset_row: int,
    parent_map: dict,
) -> bool:
    """Try table fill using cellAddr coordinates and target offset."""
    label_tbl = _get_ancestor(label_cell, "tbl", parent_map)
    if label_tbl is None or label_addr is None:
        return False

    try:
        label_col = int(label_addr.get("colAddr", "-1"))
        label_row = int(label_addr.get("rowAddr", "-1"))
    except ValueError:
        label_col = -1
        label_row = -1

    target_col = label_col + offset_col
    target_row = label_row + offset_row
    target_cell = _find_cell_by_addr(label_tbl, target_col, target_row)
    if target_cell is None:
        return False

    _set_cell_text(target_cell, value)
    table_idx = _get_table_index(tree, label_tbl)
    _log_event({"type": "replace", "idx": label_idx, "find": label, "replace": value})
    value_display = value[:40] + ("..." if len(value) > 40 else "")
    print(f"  Table cell '{label}' -> '{value_display}' (T{table_idx} R{target_row} C{target_col})")
    return True


def _try_fallback_fill(
    text_elements: list,
    start_idx: int,
    label: str,
    value: str,
    label_cell,
    parent_map: dict,
) -> bool:
    """Try table fill by scanning nearby text elements within the same table."""
    label_tbl = _get_ancestor(label_cell, "tbl", parent_map)
    for j in range(start_idx + 1, min(start_idx + 50, len(text_elements))):
        next_elem = text_elements[j]
        next_cell = _get_ancestor(next_elem, "tc", parent_map)
        next_tbl = _get_ancestor(next_cell, "tbl", parent_map) if next_cell is not None else None
        if next_cell is None or next_cell is label_cell or next_tbl is not label_tbl:
            continue

        next_elem.text = value
        for child in list(next_elem):
            next_elem.remove(child)
        _log_event({"type": "replace", "idx": j, "find": label, "replace": value})
        value_display = value[:40] + ("..." if len(value) > 40 else "")
        print(f"  Table cell '{label}' -> '{value_display}' (fallback)")
        return True
    return False


def apply_table_cell_fills_xml(tree, fills: list) -> int:
    """Fill table cells by finding label text and replacing the adjacent value cell.

    Each fill: {"find_label": str, "value": str}

    Primary strategy: cellAddr-based table lookup by offset.
    Fallback strategy: flat scan for next text element in a different cell.
    """
    total = 0
    parent_map = _build_parent_map(tree)
    text_elements = get_all_text_elements(tree)

    for fill in fills:
        label = fill["find_label"]
        value = fill["value"]
        offset = fill.get("target_offset", {"col": 1, "row": 0})
        offset_col = int(offset.get("col", 1))
        offset_row = int(offset.get("row", 0))
        found = False
        matches = _find_label_matches(text_elements, label)

        for i, elem in matches:
            label_cell = _get_ancestor(elem, "tc", parent_map)
            if label_cell is None:
                continue

            label_addr = label_cell.find(f"./{HP_CELLADDR_TAG}")
            if _try_celladdr_fill(tree, i, label, value, label_cell, label_addr, offset_col, offset_row, parent_map):
                total += 1
                found = True
                break
            if _try_fallback_fill(text_elements, i, label, value, label_cell, parent_map):
                total += 1
                found = True
                break

        if not found:
            print(f"  WARNING: Table label '{label}' not found or no adjacent cell", file=sys.stderr)

    return total


def _create_paragraph(ref_p, text: str):
    """Create paragraph by cloning reference paragraph style/layout."""
    new_p = deepcopy(ref_p)

    runs = new_p.findall(f"./{HP_RUN_TAG}")
    if not runs:
        run = etree.Element(HP_RUN_TAG)
        new_p.append(run)
        runs = [run]

    for run in runs[1:]:
        new_p.remove(run)

    t_elem = runs[0].find(f"./{HP_T_TAG}")
    if t_elem is None:
        t_elem = etree.SubElement(runs[0], HP_T_TAG)
    t_elem.text = text
    for child in list(t_elem):
        t_elem.remove(child)

    return new_p


def apply_multi_paragraph_fills(tree, fills: list) -> int:
    """Inject multi-paragraph content into a target cell."""
    parent_map = _build_parent_map(tree)
    text_elements = get_all_text_elements(tree)
    total = 0

    for fill in fills:
        section_id = fill.get("section_id", "?")
        prefix = fill["guide_text_prefix"]
        paragraphs = fill.get("paragraphs", [])
        replaced = False

        for elem_idx, elem in enumerate(text_elements):
            if not (elem.text and prefix in elem.text):
                continue

            cell = _get_ancestor(elem, "tc", parent_map)
            if cell is None:
                continue
            sub_list = cell.find(f"./{HP_SUBLIST_TAG}")
            if sub_list is None:
                continue
            ref_p = sub_list.find(f"./{HP_P_TAG}")
            if ref_p is None:
                continue

            for paragraph in list(sub_list.findall(f"./{HP_P_TAG}")):
                sub_list.remove(paragraph)
            for paragraph_text in paragraphs:
                sub_list.append(_create_paragraph(ref_p, paragraph_text))

            total += 1
            replaced = True
            _log_event({"type": "replace", "idx": elem_idx, "find": prefix, "replace": f"{len(paragraphs)} paragraphs"})
            print(f"  Section {section_id}: inserted {len(paragraphs)} paragraph(s)")
            break

        if not replaced:
            print(f"  WARNING: Section {section_id} multi-paragraph target not found", file=sys.stderr)

    return total


def fill_hwpx(plan: dict, output_path: str) -> int:
    """Main fill operation: copy template, modify XML, save."""
    template_path = plan["template_file"]
    shutil.copy2(template_path, output_path)
    section_files = find_section_xmls(template_path)
    if not section_files:
        raise ValueError("No section XML files found in HWPX")
    print(f"Found {len(section_files)} section(s): {', '.join(section_files)}")

    total_replacements = 0
    with zipfile.ZipFile(template_path, "r") as zf_in:
        for section_file in section_files:
            tree = etree.fromstring(zf_in.read(section_file))
            section_total = 0
            if plan.get("simple_replacements"):
                print(f"\n--- Simple Replacements ({section_file}) ---")
                section_total += apply_simple_replacements_xml(tree, plan["simple_replacements"])
            if plan.get("section_replacements"):
                print(f"\n--- Section Replacements ({section_file}) ---")
                section_total += apply_section_replacements_xml(tree, plan["section_replacements"])
            if plan.get("table_cell_fills"):
                print(f"\n--- Table Cell Fills ({section_file}) ---")
                section_total += apply_table_cell_fills_xml(tree, plan["table_cell_fills"])
            if plan.get("multi_paragraph_fills"):
                print(f"\n--- Multi Paragraph Fills ({section_file}) ---")
                section_total += apply_multi_paragraph_fills(tree, plan["multi_paragraph_fills"])

            total_replacements += section_total
            if section_total > 0:
                modified_xml = etree.tostring(tree, xml_declaration=True, encoding="UTF-8")
                _update_zip_file(output_path, section_file, modified_xml)
                print(f"  Updated {section_file} ({section_total} replacements)")

    _log_event({"type": "done", "total": total_replacements, "output": output_path})
    return total_replacements


def _update_zip_file(zip_path: str, target_file: str, new_content: bytes) -> None:
    """Replace a single file inside a ZIP archive."""
    tmp_fd, tmp_path = tempfile.mkstemp(suffix=".hwpx")
    os.close(tmp_fd)

    with zipfile.ZipFile(zip_path, "r") as zf_in, \
         zipfile.ZipFile(tmp_path, "w") as zf_out:
        for item in zf_in.infolist():
            if item.filename == target_file:
                zf_out.writestr(item, new_content)
            else:
                zf_out.writestr(item, zf_in.read(item.filename))

    shutil.move(tmp_path, zip_path)


def inspect_template(template_path: str, query: str | None = None) -> None:
    """List all <hp:t> text elements in a template for debugging.

    Uses direct XML parsing to find ALL text including table cells.
    """
    section_files = find_section_xmls(template_path)

    total_elements = 0
    with zipfile.ZipFile(template_path, "r") as zf:
        for section_file in section_files:
            xml_bytes = zf.read(section_file)
            tree = etree.fromstring(xml_bytes)
            text_elements = get_all_text_elements(tree)
            parent_map = _build_parent_map(tree)
            total_elements += len(text_elements)

            print(f"Section: {section_file} ({len(text_elements)} text elements)\n")

            for i, elem in enumerate(text_elements):
                text = elem.text or ""
                if not text.strip():
                    continue
                if query and query.lower() not in text.lower():
                    continue
                display = text[:100] + ("..." if len(text) > 100 else "")
                context = ""
                cell = _get_ancestor(elem, "tc", parent_map)
                if cell is not None:
                    table = _get_ancestor(cell, "tbl", parent_map)
                    cell_addr = cell.find(f"./{HP_CELLADDR_TAG}")
                    if table is not None and cell_addr is not None:
                        table_idx = _get_table_index(tree, table)
                        try:
                            col = int(cell_addr.get("colAddr", "-1"))
                            row = int(cell_addr.get("rowAddr", "-1"))
                        except ValueError:
                            col = -1
                            row = -1
                        context = f"[T{table_idx} R{row} C{col}]  "

                print(f"  [{i:4d}] {context}{display}")

    print(f"\nTotal <hp:t> elements: {total_elements}")


def _collect_table_cell_infos(table) -> list[tuple[int, int, int, int, str]]:
    """Collect row/col/span/text info from table cells."""
    cell_infos = []
    for cell in table.findall(f".//{HP_TC_TAG}"):
        col, row = _parse_cell_addr(cell)
        if col is None or row is None:
            continue
        cell_span = cell.find(f"./{HP_CELLSPAN_TAG}")
        if cell_span is not None:
            try:
                col_span = int(cell_span.get("colSpan", "1"))
                row_span = int(cell_span.get("rowSpan", "1"))
            except ValueError:
                col_span = 1
                row_span = 1
        else:
            col_span = 1
            row_span = 1
        text = _get_cell_text(cell) or "[EMPTY]"
        cell_infos.append((row, col, col_span, row_span, text))
    return sorted(cell_infos, key=lambda x: (x[0], x[1]))


def _inspect_table_structure(template_path: str) -> None:
    """Inspect table layout with cell coordinates and spans."""
    with zipfile.ZipFile(template_path, "r") as zf:
        for section_file in find_section_xmls(template_path):
            tree = etree.fromstring(zf.read(section_file))
            tables = tree.findall(f".//{HP_TBL_TAG}")
            print(f"Section: {section_file} ({len(tables)} tables)\n")
            for table_idx, table in enumerate(tables):
                row_cnt = table.get("rowCnt", "?")
                col_cnt = table.get("colCnt", "?")
                print(f"  Table {table_idx}: {row_cnt} rows x {col_cnt} cols")
                for row, col, col_span, row_span, text in _collect_table_cell_infos(table):
                    span = f" (span {col_span}x{row_span})" if col_span > 1 or row_span > 1 else ""
                    print(f"    R{row} C{col}{span}: {text}")
                print()


def _get_cell_text(tc) -> str:
    """Get normalized text content of a table cell."""
    return "".join((t.text or "") for t in tc.findall(f".//{HP_T_TAG}")).strip()


def _parse_cell_addr(tc) -> tuple[int | None, int | None]:
    """Parse cell address (colAddr, rowAddr) from <hp:cellAddr>."""
    cell_addr = tc.find(f"./{HP_CELLADDR_TAG}")
    if cell_addr is None:
        return None, None
    try:
        return int(cell_addr.get("colAddr", "-1")), int(cell_addr.get("rowAddr", "-1"))
    except ValueError:
        return None, None


def _detect_placeholder_pattern(text: str) -> str | None:
    """Detect placeholder-like patterns such as OO/○○/000."""
    for pattern in PLACEHOLDER_PATTERNS:
        match = re.search(pattern, text)
        if match:
            return match.group(0)
    numeric_match = re.search(NUMERIC_PLACEHOLDER_PATTERN, text)
    if numeric_match:
        return numeric_match.group(1)
    return None


def _extract_table_schema(tree) -> list[dict]:
    """Extract table layout and cell fillability metadata."""
    tables = []
    for table_idx, table in enumerate(tree.findall(f".//{HP_TBL_TAG}")):
        row_cnt = table.get("rowCnt", "0")
        col_cnt = table.get("colCnt", "0")
        table_info = {
            "index": table_idx,
            "rows": int(row_cnt) if row_cnt.isdigit() else 0,
            "cols": int(col_cnt) if col_cnt.isdigit() else 0,
            "cells": [],
        }
        for cell in table.findall(f".//{HP_TC_TAG}"):
            col, row = _parse_cell_addr(cell)
            if col is None or row is None:
                continue
            text = _get_cell_text(cell)
            cell_info = {"row": row, "col": col, "text": text}
            if not text:
                cell_info["is_empty"] = True
            elif col == 0:
                cell_info["is_label"] = True
            table_info["cells"].append(cell_info)

        table_info["cells"].sort(key=lambda c: (c["row"], c["col"]))
        tables.append(table_info)
    return tables


def _extract_text_markers(tree, index_offset: int) -> tuple[list[dict], list[dict]]:
    """Extract guide text markers and placeholders from text elements."""
    guide_texts = []
    placeholders = []
    parent_map = _build_parent_map(tree)
    text_elements = get_all_text_elements(tree)

    for local_idx, elem in enumerate(text_elements):
        text = (elem.text or "").strip()
        if not text:
            continue
        element_index = index_offset + local_idx
        cell = _get_ancestor(elem, "tc", parent_map)
        table = _get_ancestor(cell, "tbl", parent_map) if cell is not None else None
        table_index = _get_table_index(tree, table) if table is not None else -1
        col, row = _parse_cell_addr(cell) if cell is not None else (None, None)

        if text.startswith("※"):
            guide = {"element_index": element_index, "prefix": text[:100]}
            if table_index >= 0:
                guide["table_index"] = table_index
            if row is not None and col is not None:
                guide["cell"] = f"R{row}C{col}"
            guide_texts.append(guide)

        pattern = _detect_placeholder_pattern(text)
        if pattern:
            placeholders.append({"element_index": element_index, "text": text, "pattern": pattern})

    return guide_texts, placeholders


def analyze_template(template_path: str) -> dict:
    """Analyze template and return fillable schema metadata."""
    schema = {
        "template_file": template_path,
        "total_text_elements": 0,
        "tables": [],
        "guide_texts": [],
        "placeholders": [],
    }
    index_offset = 0

    with zipfile.ZipFile(template_path, "r") as zf:
        for section_file in find_section_xmls(template_path):
            tree = etree.fromstring(zf.read(section_file))
            text_elements = get_all_text_elements(tree)
            schema["total_text_elements"] += len(text_elements)
            schema["tables"].extend(_extract_table_schema(tree))
            guide_texts, placeholders = _extract_text_markers(tree, index_offset)
            schema["guide_texts"].extend(guide_texts)
            schema["placeholders"].extend(placeholders)
            index_offset += len(text_elements)

    return schema


def main():
    parser = argparse.ArgumentParser(description="Fill HWPX template with content")
    parser.add_argument("plan", nargs="?", help="Path to fill_plan.json")
    parser.add_argument("-o", "--output", help="Override output path")
    parser.add_argument("--inspect", metavar="HWPX", help="Inspect template text runs")
    parser.add_argument("--inspect-tables", metavar="HWPX", help="Show table structure of template")
    parser.add_argument("--analyze", metavar="HWPX", help="Analyze template and output JSON schema")
    parser.add_argument("-q", "--query", help="Filter runs by text (with --inspect)")
    args = parser.parse_args()

    if args.inspect:
        inspect_template(args.inspect, args.query)
        return
    if args.inspect_tables:
        _inspect_table_structure(args.inspect_tables)
        return
    if args.analyze:
        print(json.dumps(analyze_template(args.analyze), ensure_ascii=False, indent=2))
        return

    if not args.plan:
        parser.error("fill_plan.json is required (or use --inspect / --inspect-tables / --analyze)")

    # Load plan
    plan = load_plan(args.plan)
    template_path = plan["template_file"]
    output_path = args.output or plan["output_file"]

    print(f"Template: {template_path}")
    print(f"Output:   {output_path}")
    print()

    # Fill template
    total = fill_hwpx(plan, output_path)

    # Report
    size = os.path.getsize(output_path)
    print(f"\n--- Done ---")
    print(f"Saved: {output_path} ({size:,} bytes)")
    print(f"Total replacements: {total}")


if __name__ == "__main__":
    main()
