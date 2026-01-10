# HWP → Markdown 변환 기존 솔루션 조사

## 요약

HWP를 Markdown으로 직접 변환하는 라이브러리는 많지 않으며, 대부분 중간 포맷(DOCX/HTML/JSON)을 거치는 방식을 사용한다.

---

## 1. 오픈소스 라이브러리

### 1.1 unhwp (Rust)

**HWP/HWPX → Markdown 직접 변환 지원**

| 항목 | 내용 |
|------|------|
| 언어 | Rust |
| 라이선스 | MIT |
| 저장소 | https://lib.rs/crates/unhwp |
| 지원 포맷 | HWP, HWPX |
| 출력 | Markdown, Plain Text |

#### 사용 예시

```rust
use unhwp::{parse_file, to_markdown};

fn main() -> unhwp::Result<()> {
    let markdown = to_markdown("document.hwp")?;
    std::fs::write("output.md", markdown)?;
    Ok(())
}
```

#### CLI 사용

```bash
# Markdown 변환
unhwp-cli document.hwp -o output.md

# 텍스트 추출
unhwp-cli document.hwp --text

# 정리된 출력
unhwp-cli document.hwp --cleanup
```

#### 특징
- 고성능 Rust 구현
- 구조화된 Markdown 출력 + 에셋 추출
- 다른 언어에서 subprocess로 호출 가능

---

### 1.2 hwpjs (JavaScript/TypeScript)

**HWP → JSON 파싱 (Markdown은 직접 구현 필요)**

| 항목 | 내용 |
|------|------|
| 언어 | Rust + JavaScript |
| npm 패키지 | `@ohah/hwpjs` |
| 저장소 | https://github.com/ohah/hwpjs |
| 지원 포맷 | HWP |
| 출력 | JSON |

#### 사용 방식

1. `@ohah/hwpjs`로 HWP → JSON 파싱
2. JSON 객체를 순회하며 Markdown 생성:
   - 문단 → 평문/제목 (`#`, `##`)
   - 목록 → `-`, `1.`
   - 표 → Markdown table
   - 글자 속성 → `**`, `*`

#### 특징
- Rust 핵심 로직 + JS 바인딩
- React Native, Web 환경 지원
- Markdown 변환기는 직접 구현 필요

---

### 1.3 hwpers (Rust)

**HWP 파싱 라이브러리 (렌더링 포커스)**

| 항목 | 내용 |
|------|------|
| 언어 | Rust |
| 저장소 | https://github.com/Indosaram/hwpers |
| 지원 포맷 | HWP |
| 출력 | 문서 모델 (AST) |

- 레이아웃 렌더링에 초점
- Markdown 출력은 직접 구현 필요

---

### 1.4 md2hml (Python) - 역방향

**Markdown → HWP(HML) 변환**

| 항목 | 내용 |
|------|------|
| 언어 | Python |
| 저장소 | https://github.com/msjang/md2hml |
| 방향 | Markdown → HML (HWP 호환 XML) |

- HWP → Markdown에는 직접 사용 불가
- HML 구조 이해에 참고 가능

---

## 2. 상용/클라우드 서비스

### 2.1 Vertopal

**HWP → Markdown 온라인 변환**

| 항목 | 내용 |
|------|------|
| 유형 | 웹 서비스 + CLI |
| URL | https://www.vertopal.com/en/convert/hwp-to-markdown |
| 지원 | HWP → Markdown, MD, TXT |
| 플랫폼 | macOS, Windows, Linux |

- 웹 UI 및 CLI 제공
- 무료 티어 있음 (제한적)

---

### 2.2 리베로AI 문서변환 서비스

**HWP/HWPX → Markdown REST API**

| 항목 | 내용 |
|------|------|
| 유형 | REST API |
| URL | https://liberoai.com (추정) |
| 지원 | HWP, HWPX, PDF, Office → Markdown |
| 무료 | 하루 50건 (API 기준) |

#### 특징
- 리베로 파서: 구조/서식 유지하며 Markdown 변환
- 리베로 비전: 문장/표/이미지 분리 후처리
- 어떤 언어든 HTTP 클라이언트로 호출 가능

---

### 2.3 간접 변환 서비스

#### Microsoft Hanword HWP Converter

- URL: https://www.microsoft.com/en-us/download/details.aspx?id=36772
- HWP 5.0 → DOCX 변환
- 이후 Pandoc으로 DOCX → Markdown 변환 가능

#### CloudConvert

- URL: https://cloudconvert.com/hwp-converter
- HWP → PDF, DOCX 등 다양한 포맷
- API 제공

#### NoMoreHWP

- URL: https://nomorehwp.com
- HWP → PDF, Word 변환
- 한국 서비스

---

## 3. 간접 변환 파이프라인

### Pandoc 기반 워크플로

Pandoc은 HWP를 직접 읽지 못하므로 중간 변환 필요:

```
HWP → DOCX/HTML → Pandoc → Markdown
```

#### 예시

```bash
# 1. HWP → DOCX (한글 프로그램 또는 변환 서비스 이용)
# 2. DOCX → Markdown
pandoc input.docx -o output.md
```

---

## 4. 언어별 현실적 선택지

| 언어 | 권장 방식 | 도구 |
|------|-----------|------|
| **Rust** | 직접 변환 | `unhwp` 라이브러리 |
| **JavaScript/TypeScript** | JSON 파싱 후 변환 | `@ohah/hwpjs` + 커스텀 렌더러 |
| **Python** | CLI 호출 또는 API | `unhwp-cli` subprocess 또는 리베로AI API |
| **Java** | CLI 호출 또는 API | `unhwp-cli` 또는 리베로AI API |
| **기타** | CLI 호출 | `unhwp-cli` subprocess |

---

## 5. 한글과컴퓨터 공식 SDK

### 공개 정보

- **HML 포맷**: HWP 호환 XML 포맷 공개
- **한/글 컨트롤, HWP 뷰어 SDK**: 문서 보기/편집/변환 기능 제공
- **Markdown 직접 지원**: 공식 문서에서 확인되지 않음

### 현실적 접근

공식 SDK 사용 시에도:
1. HWP → 텍스트/HTML/RTF/DOCX로 내보내기
2. 별도 변환기(Pandoc 등)로 Markdown 변환

---

## 6. 결론 및 권장사항

### 완전 오픈소스 구현 시

1. **Rust 환경**: `unhwp` 라이브러리 직접 사용
2. **다른 언어**: `unhwp-cli`를 subprocess로 호출
3. **JS/TS 환경**: `@ohah/hwpjs`로 JSON 파싱 후 Markdown 렌더러 구현

### 빠른 구현 시

- 리베로AI REST API 또는 Vertopal 서비스 활용
- 온프레미스 필요 시 `unhwp-cli` 배포

### hwp2md 프로젝트 방향

| 옵션 | 장점 | 단점 |
|------|------|------|
| `unhwp` 포팅/래핑 | 검증된 파서, MIT 라이선스 | Rust 의존성 |
| `hwpjs` 기반 구현 | JS 생태계, 웹 호환 | Markdown 변환 직접 구현 필요 |
| 자체 파서 구현 | 완전한 제어 | 개발 비용 높음 |
| 외부 서비스 래핑 | 빠른 구현 | 외부 의존성, 비용 |

---

## 참고 링크

### 오픈소스
- unhwp: https://lib.rs/crates/unhwp
- hwpjs: https://github.com/ohah/hwpjs
- hwpers: https://github.com/Indosaram/hwpers
- md2hml: https://github.com/msjang/md2hml

### 상용 서비스
- Vertopal: https://www.vertopal.com/en/convert/hwp-to-markdown
- Microsoft HWP Converter: https://www.microsoft.com/en-us/download/details.aspx?id=36772
- CloudConvert: https://cloudconvert.com/hwp-converter
- NoMoreHWP: https://nomorehwp.com

### 참고 문서
- Pandoc 사용법: https://wikidocs.net/155996
- 리베로AI 뉴스: https://www.newswire.co.kr/newsRead.php?no=1025977
