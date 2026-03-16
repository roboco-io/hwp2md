import json
import subprocess
import sys
import zipfile
from pathlib import Path

import pytest
from lxml import etree

ROOT = Path(__file__).resolve().parents[1]
SCRIPT_PATH = ROOT / "tools" / "md2hwp" / "fill_hwpx.py"
sys.path.insert(0, str(SCRIPT_PATH.parent))

import fill_hwpx as fh  # noqa: E402

NS = fh.HWPX_NS["hp"]
TEMPLATE_PATH = ROOT / "testdata" / "hwpx_20260302_200059.hwpx"
HWP2MD_BIN = ROOT / "bin" / "hwp2md"
SAMPLE_PLAN_PATH = ROOT / "testdata" / "fill_plans" / "재도전성공패키지_sample.json"


def _make_cell(col, row, text="", colspan=1, rowspan=1, with_text=True):
    tc = etree.Element(f"{{{NS}}}tc")
    sub = etree.SubElement(tc, f"{{{NS}}}subList")
    p = etree.SubElement(sub, f"{{{NS}}}p")
    p.set("paraPrIDRef", "31")
    p.set("styleIDRef", "0")
    run = etree.SubElement(p, f"{{{NS}}}run")
    run.set("charPrIDRef", "31")
    if with_text:
        t = etree.SubElement(run, f"{{{NS}}}t")
        t.text = text
    etree.SubElement(p, f"{{{NS}}}linesegarray")
    addr = etree.SubElement(tc, f"{{{NS}}}cellAddr")
    addr.set("colAddr", str(col))
    addr.set("rowAddr", str(row))
    span = etree.SubElement(tc, f"{{{NS}}}cellSpan")
    span.set("colSpan", str(colspan))
    span.set("rowSpan", str(rowspan))
    return tc


def _make_table(cells):
    tbl = etree.Element(f"{{{NS}}}tbl")
    row_map = {}
    for tc in cells:
        row = int(tc.find(f"./{fh.HP_CELLADDR_TAG}").get("rowAddr", "0"))
        row_map.setdefault(row, []).append(tc)
    for row_idx in sorted(row_map):
        tr = etree.SubElement(tbl, f"{{{NS}}}tr")
        for tc in sorted(
            row_map[row_idx],
            key=lambda cell: int(cell.find(f"./{fh.HP_CELLADDR_TAG}").get("colAddr", "0")),
        ):
            tr.append(tc)
    return tbl


def _first_text(elem):
    t = elem.find(f".//{fh.HP_T_TAG}")
    return t.text if t is not None else None


def test_build_parent_map():
    root = etree.Element("root")
    p = etree.SubElement(root, fh.HP_P_TAG)
    run = etree.SubElement(p, fh.HP_RUN_TAG)
    t = etree.SubElement(run, fh.HP_T_TAG)
    t.text = "A"
    parent_map = fh._build_parent_map(root)
    assert parent_map[t] is run
    assert parent_map[run] is p


def test_get_ancestor():
    root = etree.Element("root")
    tbl = _make_table([_make_cell(0, 0, "LABEL"), _make_cell(1, 0, "VALUE")])
    root.append(tbl)
    t = root.find(f".//{fh.HP_T_TAG}")
    parent_map = fh._build_parent_map(root)
    assert fh._get_ancestor(t, "tc", parent_map).tag == fh.HP_TC_TAG
    assert fh._get_ancestor(t, "tbl", parent_map).tag == fh.HP_TBL_TAG


def test_find_cell_by_addr():
    tbl = _make_table([_make_cell(0, 0, "A"), _make_cell(1, 0, "B")])
    tc = fh._find_cell_by_addr(tbl, 1, 0)
    assert tc is not None
    assert _first_text(tc) == "B"


def test_find_cell_by_addr_ignores_nested_table():
    inner_cell = _make_cell(1, 0, "NESTED")
    inner_tbl = _make_table([inner_cell])

    outer_cell = _make_cell(0, 0, "OUTER")
    outer_sub = outer_cell.find(f"./{fh.HP_SUBLIST_TAG}")
    outer_sub.append(inner_tbl)
    target = _make_cell(1, 0, "TARGET")
    outer_tbl = _make_table([outer_cell, target])

    result = fh._find_cell_by_addr(outer_tbl, 1, 0)
    assert result is not None
    assert _first_text(result) == "TARGET"


def test_set_cell_text_creates_hp_t_for_empty_cell():
    tc = _make_cell(0, 0, with_text=False)
    assert tc.find(f".//{fh.HP_T_TAG}") is None
    fh._set_cell_text(tc, "NEW")
    assert _first_text(tc) == "NEW"


def test_clear_cell_except_removes_other_paragraphs_and_runs():
    tc = _make_cell(0, 0, "KEEP")
    sub = tc.find(f"./{fh.HP_SUBLIST_TAG}")
    p = sub.find(f"./{fh.HP_P_TAG}")
    extra_run = etree.SubElement(p, fh.HP_RUN_TAG)
    extra_t = etree.SubElement(extra_run, fh.HP_T_TAG)
    extra_t.text = "REMOVE-RUN"
    extra_p = etree.SubElement(sub, fh.HP_P_TAG)
    extra_run_2 = etree.SubElement(extra_p, fh.HP_RUN_TAG)
    extra_t_2 = etree.SubElement(extra_run_2, fh.HP_T_TAG)
    extra_t_2.text = "REMOVE-PARA"

    keep_elem = tc.find(f".//{fh.HP_T_TAG}")
    root = etree.Element("root")
    root.append(_make_table([tc]))
    parent_map = fh._build_parent_map(root)
    fh._clear_cell_except(tc, keep_elem, parent_map)

    texts = [(t.text or "") for t in tc.findall(f".//{fh.HP_T_TAG}")]
    assert texts == ["KEEP"]


def test_get_table_index():
    root = etree.Element("root")
    tbl0 = _make_table([_make_cell(0, 0, "A")])
    tbl1 = _make_table([_make_cell(0, 0, "B")])
    root.append(tbl0)
    root.append(tbl1)
    assert fh._get_table_index(root, tbl1) == 1


@pytest.mark.parametrize(
    ("text", "pattern"),
    [
        ("OO기업", "OO"),
        ("○○○", "○○○"),
        ("0000원", "0000"),
        ("1000", None),
        ("3,448,000", None),
        ("일반 텍스트", None),
    ],
)
def test_detect_placeholder_pattern(text, pattern):
    assert fh._detect_placeholder_pattern(text) == pattern


def test_load_plan_rejects_empty_find(tmp_path):
    plan_path = tmp_path / "plan_empty_find.json"
    plan = {
        "template_file": str(TEMPLATE_PATH),
        "output_file": str(tmp_path / "out.hwpx"),
        "simple_replacements": [{"find": "", "replace": "X"}],
    }
    plan_path.write_text(json.dumps(plan, ensure_ascii=False), encoding="utf-8")
    with pytest.raises(ValueError, match="simple_replacements: 'find' must be non-empty"):
        fh.load_plan(str(plan_path))


def test_load_plan_rejects_empty_guide_text_prefix(tmp_path):
    plan_path = tmp_path / "plan_empty_section_prefix.json"
    plan = {
        "template_file": str(TEMPLATE_PATH),
        "output_file": str(tmp_path / "out.hwpx"),
        "section_replacements": [{"guide_text_prefix": "", "content": "X"}],
    }
    plan_path.write_text(json.dumps(plan, ensure_ascii=False), encoding="utf-8")
    with pytest.raises(ValueError, match="section_replacements: 'guide_text_prefix' must be non-empty"):
        fh.load_plan(str(plan_path))


def test_load_plan_rejects_empty_find_label(tmp_path):
    plan_path = tmp_path / "plan_empty_find_label.json"
    plan = {
        "template_file": str(TEMPLATE_PATH),
        "output_file": str(tmp_path / "out.hwpx"),
        "table_cell_fills": [{"find_label": "", "value": "X"}],
    }
    plan_path.write_text(json.dumps(plan, ensure_ascii=False), encoding="utf-8")
    with pytest.raises(ValueError, match="table_cell_fills: 'find_label' must be non-empty"):
        fh.load_plan(str(plan_path))


def test_load_plan_rejects_empty_multi_paragraph_prefix(tmp_path):
    plan_path = tmp_path / "plan_empty_multi_prefix.json"
    plan = {
        "template_file": str(TEMPLATE_PATH),
        "output_file": str(tmp_path / "out.hwpx"),
        "multi_paragraph_fills": [{"guide_text_prefix": "", "paragraphs": ["A"]}],
    }
    plan_path.write_text(json.dumps(plan, ensure_ascii=False), encoding="utf-8")
    with pytest.raises(ValueError, match="multi_paragraph_fills: 'guide_text_prefix' must be non-empty"):
        fh.load_plan(str(plan_path))


def test_apply_simple_replacements_xml_basic_and_occurrence():
    root = etree.Element("root")
    p1 = etree.SubElement(root, fh.HP_P_TAG)
    run1 = etree.SubElement(p1, fh.HP_RUN_TAG)
    t1 = etree.SubElement(run1, fh.HP_T_TAG)
    t1.text = "AB AB"
    p2 = etree.SubElement(root, fh.HP_P_TAG)
    run2 = etree.SubElement(p2, fh.HP_RUN_TAG)
    t2 = etree.SubElement(run2, fh.HP_T_TAG)
    t2.text = "AB"
    total = fh.apply_simple_replacements_xml(
        root,
        [{"find": "AB", "replace": "X", "occurrence": 2}],
    )
    assert total == 2
    assert t1.text == "X AB"
    assert t2.text == "X"


def test_apply_simple_replacements_xml_not_found_warning(capsys):
    root = etree.Element("root")
    p = etree.SubElement(root, fh.HP_P_TAG)
    run = etree.SubElement(p, fh.HP_RUN_TAG)
    t = etree.SubElement(run, fh.HP_T_TAG)
    t.text = "hello"
    total = fh.apply_simple_replacements_xml(root, [{"find": "missing", "replace": "X"}])
    captured = capsys.readouterr()
    assert total == 0
    assert "WARNING" in captured.err


def test_apply_section_replacements_xml_clear_cell_true():
    root = etree.Element("root")
    tc = _make_cell(0, 0, "※ guide text")
    sub = tc.find(f"./{fh.HP_SUBLIST_TAG}")
    extra_p = etree.SubElement(sub, fh.HP_P_TAG)
    extra_run = etree.SubElement(extra_p, fh.HP_RUN_TAG)
    extra_t = etree.SubElement(extra_run, fh.HP_T_TAG)
    extra_t.text = "orphan"
    root.append(_make_table([tc]))

    total = fh.apply_section_replacements_xml(
        root,
        [{"section_id": "1", "guide_text_prefix": "guide", "content": "NEW", "clear_cell": True}],
    )
    texts = [(t.text or "") for t in tc.findall(f".//{fh.HP_T_TAG}")]
    assert total == 1
    assert texts == ["NEW"]


def test_apply_section_replacements_xml_clear_cell_false():
    root = etree.Element("root")
    tc = _make_cell(0, 0, "※ guide text")
    sub = tc.find(f"./{fh.HP_SUBLIST_TAG}")
    extra_p = etree.SubElement(sub, fh.HP_P_TAG)
    extra_run = etree.SubElement(extra_p, fh.HP_RUN_TAG)
    extra_t = etree.SubElement(extra_run, fh.HP_T_TAG)
    extra_t.text = "keep-me"
    root.append(_make_table([tc]))

    total = fh.apply_section_replacements_xml(
        root,
        [{"section_id": "1", "guide_text_prefix": "guide", "content": "NEW", "clear_cell": False}],
    )
    texts = [(t.text or "") for t in tc.findall(f".//{fh.HP_T_TAG}")]
    assert total == 1
    assert "NEW" in texts
    assert "keep-me" in texts


def test_apply_table_cell_fills_xml_celladdr_lookup():
    root = etree.Element("root")
    label = _make_cell(0, 0, "LABEL")
    target = _make_cell(1, 0, "OLD")
    root.append(_make_table([label, target]))

    total = fh.apply_table_cell_fills_xml(root, [{"find_label": "LABEL", "value": "VALUE"}])
    assert total == 1
    assert _first_text(target) == "VALUE"


def test_apply_table_cell_fills_xml_fills_empty_target_cell():
    root = etree.Element("root")
    label = _make_cell(0, 0, "LABEL")
    target = _make_cell(1, 0, with_text=False)
    root.append(_make_table([label, target]))

    total = fh.apply_table_cell_fills_xml(root, [{"find_label": "LABEL", "value": "VALUE"}])
    assert total == 1
    assert _first_text(target) == "VALUE"


def test_apply_table_cell_fills_xml_fallback_does_not_cross_into_other_table():
    root = etree.Element("root")
    root.append(_make_table([_make_cell(0, 0, "LABEL")]))
    body_p = etree.SubElement(root, fh.HP_P_TAG)
    body_run = etree.SubElement(body_p, fh.HP_RUN_TAG)
    body_t = etree.SubElement(body_run, fh.HP_T_TAG)
    body_t.text = "BODY_TEXT"
    fallback_target = _make_cell(0, 0, "TARGET")
    root.append(_make_table([fallback_target]))

    total = fh.apply_table_cell_fills_xml(
        root,
        [{"find_label": "LABEL", "value": "VALUE", "target_offset": {"col": 99, "row": 0}}],
    )

    assert total == 0
    assert body_t.text == "BODY_TEXT"
    assert _first_text(fallback_target) == "TARGET"


def test_apply_multi_paragraph_fills_injects_multiple_paragraphs():
    root = etree.Element("root")
    tc = _make_cell(0, 0, "※ TARGET")
    root.append(_make_table([tc]))
    total = fh.apply_multi_paragraph_fills(
        root,
        [
            {
                "section_id": "2-1",
                "guide_text_prefix": "TARGET",
                "paragraphs": ["P1", "P2", "P3"],
            }
        ],
    )
    sub = tc.find(f"./{fh.HP_SUBLIST_TAG}")
    paragraphs = sub.findall(f"./{fh.HP_P_TAG}")
    texts = ["".join((t.text or "") for t in p.findall(f".//{fh.HP_T_TAG}")) for p in paragraphs]
    assert total == 1
    assert texts == ["P1", "P2", "P3"]


def test_apply_multi_paragraph_fills_deepcopy_preserves_style_and_isolation():
    root = etree.Element("root")
    tc = _make_cell(0, 0, "※ TARGET")
    root.append(_make_table([tc]))

    fh.apply_multi_paragraph_fills(
        root,
        [
            {
                "section_id": "2-1",
                "guide_text_prefix": "TARGET",
                "paragraphs": ["A", "B"],
            }
        ],
    )

    sub = tc.find(f"./{fh.HP_SUBLIST_TAG}")
    paragraphs = sub.findall(f"./{fh.HP_P_TAG}")
    assert len(paragraphs) == 2
    assert len({id(p) for p in paragraphs}) == 2
    for p in paragraphs:
        assert p.get("paraPrIDRef") == "31"
        assert p.get("styleIDRef") == "0"
        assert p.find(f"./{{{NS}}}linesegarray") is not None
        run = p.find(f"./{fh.HP_RUN_TAG}")
        assert run is not None and run.get("charPrIDRef") == "31"


def test_full_fill_cycle_real_template(tmp_path):
    output = tmp_path / "filled.hwpx"
    plan = {
        "template_file": str(TEMPLATE_PATH),
        "output_file": str(output),
        "simple_replacements": [{"find": "OO학과 교수 재직(00년)", "replace": "테스트 경력"}],
        "section_replacements": [
            {
                "section_id": "1-1",
                "guide_text_prefix": "※ 과거 폐업 원인을",
                "content": "섹션 테스트 내용",
                "clear_cell": True,
            }
        ],
        "table_cell_fills": [{"find_label": "과제명", "value": "테스트 과제명"}],
        "multi_paragraph_fills": [
            {
                "section_id": "2-1",
                "guide_text_prefix": "신청하기 이전까지",
                "paragraphs": ["문단1", "문단2", "문단3"],
            }
        ],
    }
    total = fh.fill_hwpx(plan, str(output))

    assert total >= 4
    assert output.exists()
    with zipfile.ZipFile(output) as zf:
        names = zf.namelist()
        assert "Contents/section0.xml" in names
        xml = zf.read("Contents/section0.xml").decode("utf-8")
        assert "테스트 과제명" in xml
        assert "문단1" in xml


def test_inspect_cli_includes_table_context():
    proc = subprocess.run(
        [sys.executable, str(SCRIPT_PATH), "--inspect", str(TEMPLATE_PATH), "-q", "과제명"],
        check=True,
        capture_output=True,
        text=True,
    )
    assert "[T4 R0 C0]" in proc.stdout


def test_analyze_cli_outputs_valid_schema():
    proc = subprocess.run(
        [sys.executable, str(SCRIPT_PATH), "--analyze", str(TEMPLATE_PATH)],
        check=True,
        capture_output=True,
        text=True,
    )
    data = json.loads(proc.stdout)
    assert data["total_text_elements"] == 382
    assert len(data["tables"]) == 28
    assert len(data["guide_texts"]) > 0
    assert len(data["placeholders"]) > 0


def test_analyze_excludes_comma_formatted_amount_placeholders():
    data = fh.analyze_template(str(TEMPLATE_PATH))
    placeholder_texts = {entry["text"] for entry in data["placeholders"]}
    assert "3,448,000" not in placeholder_texts
    assert "7,652,000" not in placeholder_texts
    assert "7,000,000" not in placeholder_texts
    assert any("OO" in text or "○○" in text for text in placeholder_texts)


def test_full_fill_cycle_with_reverse_conversion_if_available(tmp_path):
    if not HWP2MD_BIN.exists():
        pytest.skip("bin/hwp2md not found")

    output = tmp_path / "filled_reverse.hwpx"
    plan = {
        "template_file": str(TEMPLATE_PATH),
        "output_file": str(output),
        "table_cell_fills": [
            {"find_label": "과제명", "value": "역변환 과제명"},
            {"find_label": "기업명", "value": "역변환 기업명"},
        ],
    }
    fh.fill_hwpx(plan, str(output))

    md_path = tmp_path / "verify.md"
    subprocess.run([str(HWP2MD_BIN), str(output), "-o", str(md_path)], check=True)
    md = md_path.read_text(encoding="utf-8")
    assert "역변환 과제명" in md
    assert "역변환 기업명" in md


def test_e2e_fill_with_sample_plan(tmp_path):
    plan = json.loads(SAMPLE_PLAN_PATH.read_text(encoding="utf-8"))
    output = tmp_path / "e2e_fill_test.hwpx"
    plan["output_file"] = str(output)
    plan_path = tmp_path / "sample_plan.json"
    plan_path.write_text(json.dumps(plan, ensure_ascii=False), encoding="utf-8")

    subprocess.run([sys.executable, str(SCRIPT_PATH), str(plan_path)], check=True)
    assert output.exists()

    with zipfile.ZipFile(output) as zf:
        assert "Contents/section0.xml" in zf.namelist()
        tree = etree.fromstring(zf.read("Contents/section0.xml"))

    # XML is the source of truth for paragraph structure in table cells.
    target_paragraphs = None
    expected_paragraphs = [
        "테스트 준비현황 문단 1",
        "테스트 준비현황 문단 2",
        "테스트 준비현황 문단 3",
    ]
    for tc in tree.findall(f".//{fh.HP_TC_TAG}"):
        paragraphs = []
        for p in tc.findall(f"./{fh.HP_SUBLIST_TAG}/{fh.HP_P_TAG}"):
            text = "".join((t.text or "") for t in p.findall(f".//{fh.HP_T_TAG}")).strip()
            if text:
                paragraphs.append(text)
        if paragraphs[:3] == expected_paragraphs:
            target_paragraphs = paragraphs
            break
    assert target_paragraphs is not None, "multi_paragraph_fills content not found as separate XML paragraphs"
    assert target_paragraphs[:3] == expected_paragraphs

    if HWP2MD_BIN.exists():
        verify_path = tmp_path / "e2e_verify.md"
        subprocess.run([str(HWP2MD_BIN), str(output), "-o", str(verify_path)], check=True)
        md = verify_path.read_text(encoding="utf-8")
        # Reverse markdown check is for content presence only.
        for text in expected_paragraphs:
            assert text in md
        assert "테스트 과제명" in md
        assert "테스트 기업명" in md
    else:
        with zipfile.ZipFile(output) as zf:
            xml = zf.read("Contents/section0.xml").decode("utf-8")
            assert "테스트 과제명" in xml
            assert "테스트 기업명" in xml
