"""renderer.py - HWPX to HTML converter for browser preview.

Parses HWPX (ZIP+XML) and generates HTML with data-idx attributes
on each text element for real-time highlight support.
"""

import zipfile
from html import escape
from lxml import etree

HP = "http://www.hancom.co.kr/hwpml/2011/paragraph"
HS = "http://www.hancom.co.kr/hwpml/2011/section"
NS = {"hp": HP, "hs": HS}

_T = f"{{{HP}}}t"
_P = f"{{{HP}}}p"
_RUN = f"{{{HP}}}run"
_TBL = f"{{{HP}}}tbl"
_TR = f"{{{HP}}}tr"
_TC = f"{{{HP}}}tc"
_CELL_SPAN = f"{{{HP}}}cellSpan"
_CELL_SZ = f"{{{HP}}}cellSz"
_SUB_LIST = f"{{{HP}}}subList"
_LINE_BREAK = f"{{{HP}}}lineBreak"
_SEC = f"{{{HS}}}sec"


def render_hwpx_to_html(hwpx_path: str) -> tuple[str, int]:
    """Convert HWPX file to HTML string.

    Returns (html_string, total_text_count).
    Each <hp:t> gets a <span data-idx="N"> for SSE targeting.
    """
    xml_bytes = _extract_section_xml(hwpx_path)
    root = etree.fromstring(xml_bytes)
    ctx = {"idx": 0}
    html = _render_element(root, ctx)
    return html, ctx["idx"]


def _extract_section_xml(hwpx_path: str) -> bytes:
    """Extract section0.xml from HWPX ZIP."""
    with zipfile.ZipFile(hwpx_path, "r") as zf:
        for name in sorted(zf.namelist()):
            if name.startswith("Contents/section") and name.endswith(".xml"):
                return zf.read(name)
    raise FileNotFoundError("No section XML found in HWPX")


def _render_element(elem, ctx: dict) -> str:
    """Recursively render an XML element to HTML."""
    tag = _local_tag(elem)

    if tag == "sec":
        return _render_children(elem, ctx)
    if tag == "tbl":
        return _render_table(elem, ctx)
    if tag == "p":
        return _render_paragraph(elem, ctx)
    if tag == "run":
        return _render_run(elem, ctx)
    if tag == "t":
        return _render_text(elem, ctx)
    if tag == "lineBreak":
        return "<br>"
    if tag in ("subList",):
        return _render_children(elem, ctx)

    return _render_children(elem, ctx)


def _render_children(elem, ctx: dict) -> str:
    """Render all children of an element."""
    parts = []
    for child in elem:
        parts.append(_render_element(child, ctx))
    return "".join(parts)


def _render_table(tbl, ctx: dict) -> str:
    """Render <hp:tbl> as HTML <table>."""
    rows = tbl.findall(_TR)
    if not rows:
        return ""

    # Calculate column width ratios from first row
    first_row_cells = rows[0].findall(_TC)
    total_width = 0
    col_widths = []
    for cell in first_row_cells:
        sz = cell.find(_CELL_SZ)
        w = int(sz.get("width", "0")) if sz is not None else 0
        span = cell.find(_CELL_SPAN)
        cs = int(span.get("colSpan", "1")) if span is not None else 1
        for _ in range(cs):
            col_widths.append(w // cs if cs > 0 else w)
        total_width += w

    html = '<table class="hwpx-table">'
    if total_width > 0 and col_widths:
        html += "<colgroup>"
        for w in col_widths:
            pct = round(w / total_width * 100, 1) if total_width else 0
            html += f'<col style="width:{pct}%">'
        html += "</colgroup>"

    for row in rows:
        html += _render_row(row, ctx)
    html += "</table>"
    return html


def _render_row(tr, ctx: dict) -> str:
    """Render <hp:tr> as HTML <tr>."""
    cells = tr.findall(_TC)
    html = "<tr>"
    for cell in cells:
        html += _render_cell(cell, ctx)
    html += "</tr>"
    return html


def _render_cell(tc, ctx: dict) -> str:
    """Render <hp:tc> as HTML <td>."""
    span = tc.find(_CELL_SPAN)
    cs = int(span.get("colSpan", "1")) if span is not None else 1
    rs = int(span.get("rowSpan", "1")) if span is not None else 1

    is_header = tc.get("header") == "1"
    tag = "th" if is_header else "td"

    attrs = ""
    if cs > 1:
        attrs += f' colspan="{cs}"'
    if rs > 1:
        attrs += f' rowspan="{rs}"'

    # Render cell content (paragraphs inside subList)
    content = ""
    sub_list = tc.find(_SUB_LIST)
    if sub_list is not None:
        content = _render_children(sub_list, ctx)
    else:
        content = _render_children(tc, ctx)

    return f"<{tag}{attrs}>{content}</{tag}>"


def _render_paragraph(p, ctx: dict) -> str:
    """Render <hp:p> as HTML <p>."""
    content = _render_children(p, ctx)
    if not content.strip():
        return ""
    return f'<p class="hwpx-p">{content}</p>'


def _render_run(run, ctx: dict) -> str:
    """Render <hp:run> as inline content."""
    return _render_children(run, ctx)


def _render_text(t, ctx: dict) -> str:
    """Render <hp:t> as <span data-idx="N">.

    Always increments idx (even for empty text) to stay in sync with
    fill_hwpx.py's enumerate(get_all_text_elements(tree)).
    """
    text = t.text or ""
    idx = ctx["idx"]
    ctx["idx"] += 1
    if not text:
        return ""
    return f'<span class="hwpx-t" data-idx="{idx}">{escape(text)}</span>'


def _local_tag(elem) -> str:
    """Get local tag name without namespace."""
    tag = elem.tag
    if "}" in tag:
        return tag.split("}", 1)[1]
    return tag
