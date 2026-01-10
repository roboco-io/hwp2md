# HWPX와 Markdown 간의 차이점

이 문서는 HWPX 포맷을 Markdown으로 변환할 때 발생하는 구조적 차이점과 변환기의 처리 방식을 설명합니다.

## 개요

HWPX는 한글 워드프로세서의 XML 기반 포맷으로, Markdown보다 훨씬 풍부한 레이아웃과 서식 기능을 지원합니다. 변환 과정에서 일부 기능은 Markdown의 제한으로 인해 대체 방식으로 처리됩니다.

## 차이점 목록

### 1. 중첩 테이블 (Nested Tables)

| 항목 | 설명 |
|------|------|
| **HWPX** | 테이블 셀 안에 또 다른 테이블 삽입 가능 |
| **Markdown** | 중첩 테이블 미지원 |
| **변환 방식** | 내부 테이블을 텍스트로 변환하여 부모 셀에 인라인으로 포함 |

**예시:**
- HWPX: 연구논문 요약서 폼에서 SCI/SSCI 체크박스가 별도 테이블로 구성
- Markdown: `□ SCI | □ SSCI | □ A&HCI` 형태로 텍스트 변환

**관련 코드:** `internal/parser/hwpx/parser.go` - `convertTableToText()` 함수

---

### 2. 셀 병합 (Cell Span)

| 항목 | 설명 |
|------|------|
| **HWPX** | `cellSpan`, `rowSpan` 속성으로 셀 병합 지원 |
| **Markdown** | 표준 Markdown은 셀 병합 미지원 |
| **변환 방식** | 세로 병합(rowspan)은 `〃` 표시, 가로 병합(colspan)은 빈 셀로 처리 |

**예시:**

원본 HWPX (2x2 셀이 세로로 병합된 경우):
```
| 항목 | 값1 |
|      | 값2 |
```

변환된 Markdown:
```
| 항목 | 값1 |
| 〃   | 값2 |
```

**설계 결정:**
- 세로 병합: `〃` (동일 부호) - 위 셀과 같은 내용임을 시각적으로 표시
- 가로 병합: 빈 셀 유지 - 테이블 구조 보존

**관련 코드:**
- `internal/parser/hwpx/parser.go` - `cellSpan` 파싱 로직
- `internal/cli/convert.go` - `writeMarkdownTable()` 함수

---

### 3. 전각/반각 공백 (Full-width/Half-width Space)

| 항목 | 설명 |
|------|------|
| **HWPX** | `<hp:fwSpace/>`, `<hp:hwSpace/>` 요소로 특수 공백 표현 |
| **Markdown** | 일반 공백만 지원 |
| **변환 방식** | 모든 특수 공백을 일반 공백 문자로 변환 |

**예시:**
- HWPX: `①<hp:fwSpace/>소통·공감`
- Markdown: `① 소통·공감`

**관련 코드:** `internal/parser/hwpx/parser.go` - `readElementText()` 함수

---

### 4. 정보 박스 테이블 (Info Box Tables)

| 항목 | 설명 |
|------|------|
| **HWPX** | 단일 컬럼 테이블로 강조 박스 표현 |
| **Markdown** | 테이블보다 목록/인용이 적합 |
| **변환 방식** | `[제목]` 또는 `【제목】` 패턴의 단일 컬럼 테이블을 목록 형식으로 변환 |

**예시:**
- HWPX: 법령 인용 박스 `【국가공무원법 제33조】`
- Markdown: `**【국가공무원법 제33조】**` + 번호 목록

**관련 코드:** `internal/cli/convert.go` - `isInfoBoxTable()`, `writeLegalContentAsText()` 함수

---

### 5. 이미지 및 OLE 객체

| 항목 | 설명 |
|------|------|
| **HWPX** | 바이너리 데이터로 이미지 임베드, OLE 객체 지원 |
| **Markdown** | 이미지 링크만 지원, OLE 미지원 |
| **변환 방식** | 이미지는 Base64 데이터 URI 또는 외부 파일 참조로 변환, OLE는 무시 |

---

### 6. 글꼴 및 스타일

| 항목 | 설명 |
|------|------|
| **HWPX** | 다양한 글꼴, 크기, 색상, 밑줄 스타일 지원 |
| **Markdown** | 굵게(`**`), 기울임(`*`), 취소선(`~~`)만 지원 |
| **변환 방식** | 기본 서식만 변환, 복잡한 스타일은 무시 |

---

### 7. 페이지 레이아웃

| 항목 | 설명 |
|------|------|
| **HWPX** | 페이지 크기, 여백, 단 나누기, 머리글/바닥글 지원 |
| **Markdown** | 레이아웃 개념 없음 |
| **변환 방식** | 모든 페이지 레이아웃 정보 무시, 콘텐츠만 추출 |

---

### 8. 각주 및 미주

| 항목 | 설명 |
|------|------|
| **HWPX** | 각주(`footnote`), 미주(`endnote`) 지원 |
| **Markdown** | 일부 확장에서만 각주 지원 |
| **변환 방식** | 현재 미구현 (향후 지원 예정) |

---

### 9. 목차 (Table of Contents)

| 항목 | 설명 |
|------|------|
| **HWPX** | 자동 목차 생성 기능 |
| **Markdown** | 자동 목차 미지원 |
| **변환 방식** | 목차를 정적 텍스트로 변환 |

---

### 10. 수식 (Equations)

| 항목 | 설명 |
|------|------|
| **HWPX** | 한글 수식 에디터 포맷 |
| **Markdown** | LaTeX 수식 (일부 렌더러에서 지원) |
| **변환 방식** | 현재 미구현 (향후 LaTeX 변환 예정) |

---

## 변환 품질 향상

Stage 2 (LLM 포맷팅)를 사용하면 위의 제한사항으로 인해 발생하는 가독성 문제를 개선할 수 있습니다:

```bash
# Stage 1만 (파서)
hwp2md convert document.hwpx

# Stage 2 포함 (LLM 포맷팅)
hwp2md convert document.hwpx --llm
```

## 기여하기

새로운 차이점을 발견하거나 변환 방식 개선 제안이 있으시면 [GitHub Issues](https://github.com/roboco-io/hwp2md/issues)에 등록해 주세요.

---

*이 문서는 hwp2md 변환기 개발 과정에서 발견된 차이점을 기록합니다.*
*마지막 업데이트: 2026-01-10*
