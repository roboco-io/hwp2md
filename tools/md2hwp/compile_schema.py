"""Compile 사업계획서 schema JSON → fill_plan.json for fill_hwpx.py"""
import json
import sys
import re

GUIDE_TEXT_MAP = {
    "1-1_폐업원인분석": "※ 과거 폐업 원인을",
    "1-2_목표시장": "재창업 아이템 진출 목표시장",
    "2-1_준비현황": "신청하기 이전까지",
    "2-2_구체화방안": "재창업 아이템의 핵심 기능",
    "3-1_비즈니스모델": "재창업 아이템의 가치 전달",
    "3-2_사업화전략": "정의된 목표시장(고객)에 진입",
    "4-1_보유역량": "대표자 및 조직이 사업화를 위해",
    "4-2_조직구성계획": "협약기간 동안 조직 구성",
}

# Template placeholder text → replacement mapping
TIMELINE_PLACEHOLDERS = [
    ("시제품 개발·개선 완료", "목표"),
    ("개발 또는 개선 하려는 내용", "세부내용"),
    ("웹사이트 오픈", "목표"),
    ("웹사이트 기능 및 용도 등", "세부내용"),
    ("시장 검증", "목표"),
    ("검증 대상 및 방법 등", "세부내용"),
]

BUDGET_PLACEHOLDERS = [
    ("DMD소켓 구입(00개×0000원)", 0),  # row index for 재료비 line 1
    ("전원IC류 구입(00개×000원)", 1),   # row index for 재료비 line 2
    ("시금형제작 외주용역", 2),          # 외주용역비
    ("1명 x 0개월 x 0000원", 3),        # 인건비
]

BUDGET_AMOUNT_PLACEHOLDERS = [
    ("3,448,000", 0, "정부지원"),
    ("7,652,000", 1, "정부지원"),
    ("7,000,000", 2, "현금"),
    ("3,000,000", 3, "현물"),
]

TEAM_PLACEHOLDERS = [
    ("공동대표", 0, "직위"),
    ("S/W 개발 총괄", 0, "담당업무"),
    ("OO학과 교수 재직(00년)", 0, "보유역량"),
    ("팀장", 1, "직위"),
    ("홍보 및 마케팅", 1, "담당업무"),
    ("OO학과 전공, 관련 경력(00년 이상)", 1, "보유역량"),
]


def compile_schema(schema, template_path, output_path):
    fill_plan = {
        "template_file": template_path,
        "output_file": output_path,
        "simple_replacements": [],
        "table_cell_fills": [],
        "multi_paragraph_fills": [],
    }

    # --- meta → table_cell_fills ---
    meta = schema["meta"]
    fill_plan["table_cell_fills"].extend([
        {"find_label": "과제명", "value": meta["과제명"]},
        {"find_label": "기업명", "value": meta["기업명"]},
        {"find_label": "아이템(서비스) 개요", "value": meta["아이템_개요"]},
    ])

    # --- sections → multi_paragraph_fills ---
    for key, section in schema["sections"].items():
        if key not in GUIDE_TEXT_MAP:
            print(f"WARN: Unknown section key: {key}", file=sys.stderr)
            continue
        paragraphs = section.get("paragraphs", [])
        if not paragraphs:
            print(f"WARN: Empty paragraphs for {key}", file=sys.stderr)
            continue
        fill_plan["multi_paragraph_fills"].append({
            "section_id": key.split("_")[0],
            "guide_text_prefix": GUIDE_TEXT_MAP[key],
            "paragraphs": paragraphs,
        })

    # --- 폐업이력 → simple_replacements (1st company only) ---
    closure = schema.get("폐업이력", {})
    total_count = closure.get("총_폐업횟수", 0)
    if total_count > 0:
        fill_plan["simple_replacements"].append(
            {"find": "폐업 이력(총 폐업 횟수 : 0회)", "replace": f"폐업 이력(총 폐업 횟수 : {total_count}회)"}
        )
    companies = closure.get("companies", [])
    if companies:
        c = companies[0]
        mappings = [
            ("○○○○", c.get("기업명", "")),
            ("개인 / 법인", c.get("기업구분", "")),
            ("2000.00.00.(개업연월일 또는 회사성립연월일)~2000.00.00.(폐업일)", c.get("사업기간", "")),
            ("아이템 간략히 소개", c.get("아이템_개요", "")),
            ("폐업을 하게된 원인 및 사유 등을 간략기 기재", c.get("폐업원인", "")),
        ]
        for find, replace in mappings:
            if replace:
                fill_plan["simple_replacements"].append({"find": find, "replace": replace})

    # --- 추진일정 → simple_replacements ---
    timeline = schema.get("추진일정", {})
    rows = timeline.get("rows", [])
    for i, row in enumerate(rows):
        if i < len(TIMELINE_PLACEHOLDERS) // 2:
            goal_ph = TIMELINE_PLACEHOLDERS[i * 2]
            detail_ph = TIMELINE_PLACEHOLDERS[i * 2 + 1]
            if row.get("목표"):
                fill_plan["simple_replacements"].append({"find": goal_ph[0], "replace": row["목표"]})
            if row.get("세부내용"):
                fill_plan["simple_replacements"].append({"find": detail_ph[0], "replace": row["세부내용"]})
    # Timeline dates: "~ 00월" (rows 1,2), "~00월" (row 3)
    date_placeholders = ["~ 00월", "~ 00월", "~00월"]
    for i, row in enumerate(rows):
        if i < len(date_placeholders) and row.get("일정"):
            fill_plan["simple_replacements"].append({
                "find": date_placeholders[i],
                "replace": row["일정"],
                "occurrence": 1,
            })

    # --- 사업비 → simple_replacements ---
    budget = schema.get("사업비", {})
    budget_rows = budget.get("rows", [])
    for i, row in enumerate(budget_rows):
        if i < len(BUDGET_PLACEHOLDERS):
            ph_text, _ = BUDGET_PLACEHOLDERS[i][:2]
            if row.get("산출근거"):
                # Strip leading "• " if present for matching
                replace_text = row["산출근거"]
                if not replace_text.startswith("•"):
                    replace_text = "• " + replace_text
                fill_plan["simple_replacements"].append({"find": ph_text, "replace": replace_text.lstrip("• ")})

    # Budget amounts
    for ph_amount, row_idx, col_type in BUDGET_AMOUNT_PLACEHOLDERS:
        if row_idx < len(budget_rows):
            row = budget_rows[row_idx]
            amount_key = {"정부지원": "정부지원", "현금": "현금", "현물": "현물"}.get(col_type)
            if amount_key and row.get(amount_key, 0) > 0:
                fill_plan["simple_replacements"].append({
                    "find": ph_amount,
                    "replace": f"{row[amount_key]:,}",
                })

    # --- 인력현황 → simple_replacements ---
    team = schema.get("인력현황", {})
    members = team.get("members", [])
    current_count = team.get("현재_재직인원", 0)
    planned_hire = team.get("추가_고용계획", 0)

    for ph_text, member_idx, field in TEAM_PLACEHOLDERS:
        if member_idx < len(members):
            value = members[member_idx].get(field, "")
            if value:
                fill_plan["simple_replacements"].append({"find": ph_text, "replace": value})

    # Personnel count replacements
    if current_count > 0:
        fill_plan["simple_replacements"].append({"find": "00\n명\n추가 고용계획", "replace": f"{current_count:02d}\n명\n추가 고용계획", "occurrence": 1})

    return fill_plan


def preflight(schema):
    """Preflight validation. Returns (errors, warnings)."""
    errors = []
    warnings = []

    # Required fields
    meta = schema.get("meta", {})
    for field in ["과제명", "기업명", "아이템_개요"]:
        if not meta.get(field):
            errors.append(f"BLOCK: meta.{field} 비어있음")

    # Sections not empty
    for key, section in schema.get("sections", {}).items():
        if not section.get("paragraphs"):
            errors.append(f"BLOCK: sections.{key}.paragraphs 비어있음")

    # Placeholder remnants - check only meta and sections paragraphs
    placeholder_re = re.compile(r'(?:OO[^대학]|(?<!\d)00개|0000원)')
    for field in ["과제명", "기업명", "아이템_개요"]:
        val = meta.get(field, "")
        for m in placeholder_re.finditer(val):
            warnings.append(f"WARN: meta.{field}에 플레이스홀더 잔존: '{m.group()}'")
    for key, section in schema.get("sections", {}).items():
        for i, p in enumerate(section.get("paragraphs", [])):
            for m in placeholder_re.finditer(p):
                warnings.append(f"WARN: sections.{key}.paragraphs[{i}]에 플레이스홀더 잔존: '{m.group()}'")

    # Budget validation
    budget = schema.get("사업비", {})
    budget_rows = budget.get("rows", [])
    total_gov = sum(r.get("정부지원", 0) for r in budget_rows)
    total_cash = sum(r.get("현금", 0) for r in budget_rows)
    total_inkind = sum(r.get("현물", 0) for r in budget_rows)
    total = total_gov + total_cash + total_inkind

    if total_gov > 100_000_000:
        errors.append(f"BLOCK: 정부지원 {total_gov:,}원 > 1억원 한도 초과")
    if total > 0:
        if total_gov / total > 0.75:
            errors.append(f"BLOCK: 정부지원 비율 {total_gov/total:.1%} > 75%")
        if total_cash / total < 0.05:
            warnings.append(f"WARN: 현금 비율 {total_cash/total:.1%} < 5% (권장)")
        if total_inkind / total > 0.20:
            warnings.append(f"WARN: 현물 비율 {total_inkind/total:.1%} > 20%")

    # Short paragraphs
    for key, section in schema.get("sections", {}).items():
        for i, p in enumerate(section.get("paragraphs", [])):
            if len(p) < 10:
                warnings.append(f"WARN: sections.{key}.paragraphs[{i}] 길이 {len(p)}자 (너무 짧음)")

    return errors, warnings


if __name__ == "__main__":
    input_file = sys.argv[1]
    template = sys.argv[2] if len(sys.argv) > 2 else "testdata/hwpx_20260302_200059.hwpx"
    output = sys.argv[3] if len(sys.argv) > 3 else "/tmp/compiled_output.hwpx"

    with open(input_file) as f:
        schema = json.load(f)

    # Preflight
    print("=== Preflight 검증 ===")
    errors, warnings = preflight(schema)
    for e in errors:
        print(f"  {e}")
    for w in warnings:
        print(f"  {w}")

    if errors:
        print(f"\n{len(errors)} BLOCK 에러 발견. 수정 후 재시도하세요.")
        sys.exit(1)

    print(f"  결과: {len(errors)} 에러, {len(warnings)} 경고\n")

    # Compile
    fill_plan = compile_schema(schema, template, output)

    output_json = "/tmp/compiled_fill_plan.json"
    with open(output_json, "w") as f:
        json.dump(fill_plan, f, ensure_ascii=False, indent=2)

    print(f"=== fill_plan.json 컴파일 완료 ===")
    print(f"  table_cell_fills: {len(fill_plan['table_cell_fills'])}개")
    print(f"  multi_paragraph_fills: {len(fill_plan['multi_paragraph_fills'])}개")
    print(f"  simple_replacements: {len(fill_plan['simple_replacements'])}개")
    print(f"  출력: {output_json}")
