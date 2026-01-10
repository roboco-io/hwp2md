# HWP 파일 포맷 조사 보고서

## 개요

HWP(Hangul Word Processor)는 한컴오피스의 워드프로세서 파일 포맷으로, 버전마다 호환되지 않는 포맷을 가진 것으로 악명이 높다. 본 문서는 hwp2md 도구 개발을 위한 HWP 포맷 조사 결과를 정리한다.

## HWP 버전별 분류

| 버전 | 컨테이너 | 인코딩 | 확장자 | 특징 |
|------|----------|--------|--------|------|
| HWP 2.x/3.x | Flat binary | Legacy Korean (EUC-KR 계열) | `.hwp` | Pre-OLE, 비표준 구조, 문서화 부족 |
| HWP 5.x | OLE/Compound File Binary | Unicode (UTF-16) | `.hwp` | 현재 주류 포맷, MS Word 컨버터 지원 |
| HWPX | ZIP + XML | UTF-8 | `.hwpx` | 개방형 포맷, OOXML/ODF와 유사한 구조 |

---

## 1. HWP 3.0 (레거시 바이너리 포맷)

### 컨테이너 구조

- **Flat binary file** - OLE Compound File이 아님
- 독자적인 바이너리 레코드 시퀀스로 구성:
  - 파일 헤더 (버전, 플래그)
  - 문서 섹션 (텍스트, 문단/문자 속성, 스타일)
  - 임베디드 객체 (이미지, 수식)

### 레코드 구조 (리버스 엔지니어링 결과)

```
+----------------+----------------+------------------+
| Record Type    | Length         | Payload          |
| (2 bytes)      | (4 bytes)      | (variable)       |
+----------------+----------------+------------------+
```

- 레코드는 tightly packed
- 마이너 버전에 따라 정렬 규칙이 다름
- 엔디언 등 공식 문서화 없음

### 인코딩

- EUC-KR 확장 등 한국어 전용 레거시 인코딩
- 16비트 또는 8비트 코드 유닛
- 제어 코드: 문단 구분, 스타일 전환, 객체 앵커

### 지원 한계

- 공식 스펙 부재로 리버스 엔지니어링에 의존
- 한컴 API 및 MS 컨버터에서 미지원
- LibreOffice의 Hangul 97 필터로 제한적 지원

---

## 2. HWP 5.x (OLE/Compound 바이너리 포맷)

### OLE Compound File 컨테이너

HWP 5.x는 Microsoft의 **Compound File Binary Format (CFBF)** 사용:

```
HWP 5.x 파일 구조
├── FileHeader (스트림) - 최상위 HWP 헤더
├── DocInfo (스트림) - 문서 속성/설정
├── BodyText (스트림) - 본문 텍스트와 문단 레코드
│   ├── Section0
│   ├── Section1
│   └── ...
├── BinData (스토리지) - 임베디드 이미지/객체
│   ├── BIN0001.png
│   ├── BIN0002.jpg
│   └── ...
├── Scripts (스토리지) - 매크로/스크립트
└── 기타 메타데이터 스트림
```

### 바이너리 레코드 포맷

각 스트림 내부에서 레코드 기반 바이너리 구조 사용:

```
+----------------+----------------+------------------+
| Record ID      | Length         | Record Data      |
| (2 bytes)      | (4 bytes)      | (N bytes)        |
+----------------+----------------+------------------+
```

#### 주요 레코드 타입

| 카테고리 | 레코드 |
|----------|--------|
| 문서 구조 | Section, Paragraph, Table, Cell, Field |
| 속성 | 폰트, 크기, 정렬, 간격 |
| 스타일 | Named property sets |
| 객체 | 이미지 참조, OLE 임베딩, 수식 |
| 기타 | 북마크, 머리글/바닥글, 각주/미주 |

### 인코딩

- **Unicode 지원** (UTF-16 계열)
- 텍스트와 제어 코드가 동일 스트림에 혼재
  - 문단 마커
  - 인라인 객체 앵커
  - 필드 코드

### 호환성

- Microsoft의 공식 "Hanword HWP document converter for Microsoft Word"에서 지원
- OnlyOffice 등 서드파티 컨버터 지원
- 마이너 버전 간 레코드 추가/변경으로 인한 호환성 이슈 존재

---

## 3. HWPX (XML 기반 포맷)

### 컨테이너 구조

**ZIP 컨테이너** 내 XML 문서와 미디어 파일:

```
HWPX 파일 구조 (ZIP 압축 해제 시)
├── [Content_Types].xml
├── version.xml
├── Contents/
│   ├── header.xml
│   ├── section0.xml
│   ├── section1.xml
│   └── ...
├── Settings/
│   ├── settings.xml
│   └── ...
├── BinData/
│   ├── image1.png
│   ├── image2.jpg
│   └── ...
└── META-INF/
    └── manifest.xml
```

### 특징

- OOXML (.docx), ODF (.odt)와 유사한 구조
- 모든 의미론이 XML 요소/속성으로 표현
- UTF-8 인코딩
- 한컴에서 공개 스펙 제공

### 장점

- 범용 XML 도구로 파싱/변환 가능
- 스키마 확장에 강건 (미지원 요소 스킵 가능)
- 정부 및 공공기관에서 아카이빙 및 상호운용성을 위해 권장

---

## 4. 임베디드 OLE 객체

HWP 5.x의 복잡성:

1. **외부 컨테이너**: `.hwp` 파일 자체가 OLE compound document
2. **내부 임베딩**: 문서 내 Excel 시트 등 OLE 객체 포함 가능
   - `BinData` 스토리지 내 별도 바이너리 블롭으로 저장
   - 래퍼 레코드 (객체 타입, 크기) + 원본 OLE 패키지 데이터

### 추출 시 고려사항

- HWP 레코드와 OLE 패키지 둘 다 파싱 필요
- 비-Windows 환경에서 CFBF 라이브러리 필요

---

## 5. 호환성 문제의 원인

### 5.1 동일 확장자의 다른 포맷

`.hwp` 확장자가 HWP 2.x/3.x와 HWP 5.x 모두에 사용됨. "HWP 지원"이라고 해도 특정 버전만 지원하는 경우가 대부분.

### 5.2 비공개 스펙

오랜 기간 내부 스펙이 비공개되어 리버스 엔지니어링에 의존. 이로 인해:
- 미지원 레코드 무시
- 코너 케이스 오해석
- 고급 레이아웃/필드 손실

### 5.3 주요 내부 재설계

```
v3 → v5 전환:
- 새 컨테이너 (OLE)
- 새 Unicode 텍스트 모델
- 새 레이아웃/객체 레코드

v5 → HWPX 전환:
- 바이너리 → XML
- 완전히 다른 파서 필요
```

### 5.4 로케일 의존성

- 초기 HWP는 한국어 인코딩과 Windows GDI/폰트에 깊이 통합
- 타 플랫폼에서 정확한 렌더링 어려움

### 5.5 불완전한 포맷 매핑

HWP 고유 기능 (정부 양식, 특수 번호매기기 등)이 OOXML/ODF에 1:1 대응 없음

### 5.6 파편화된 도구 지원

| 도구 | 지원 범위 |
|------|-----------|
| Microsoft HWP Converter | HWP 5.x만 |
| 일부 웹 서비스 | HWPX만 |
| LibreOffice 레거시 필터 | Hangul 97만 |

---

## 6. 기술 문서 및 리소스

### 공식 자료

- 한컴: HWP 파일 포맷 공개 발표 (5.x, HWPX SDK 제공)
- HWPX XML 스키마 공개

### Microsoft

- [Hanword HWP document converter for Microsoft Word](https://www.microsoft.com/en-us/download/details.aspx?id=36772) - HWP 5.0 → DOCX 변환 지원

### 오픈소스 및 커뮤니티

- LibreOffice: Hangul 97 임포트 필터 (리버스 엔지니어링 기반)
- [OnlyOffice HWP/HWPX 지원](https://www.onlyoffice.com/blog/2025/02/how-to-open-hwp-and-hwpx-files)
- 한국어 커뮤니티 및 학술 논문: 레코드 레이아웃, 스트림 구조 문서화

---

## 7. hwp2md 개발 시 고려사항

### 권장 우선순위

1. **HWPX** - XML 기반으로 파싱 용이, 공개 스펙
2. **HWP 5.x** - 주류 포맷, OLE 라이브러리 필요
3. **HWP 3.x** - 레거시, 지원 시 리버스 엔지니어링 필요

### 기술 스택 고려

| 포맷 | 필요 라이브러리 |
|------|-----------------|
| HWPX | ZIP 압축 해제 + XML 파서 |
| HWP 5.x | CFBF/OLE 라이브러리 + 바이너리 레코드 파서 |
| HWP 3.x | 커스텀 바이너리 파서 (리버스 엔지니어링 기반) |

### 주요 변환 도전과제

1. 복잡한 테이블 레이아웃 → Markdown 테이블
2. 임베디드 이미지 추출 및 참조
3. 스타일/서식 → Markdown 문법 매핑
4. 각주/미주 처리
5. 수식 변환

---

## 참고 자료

- [Wikipedia - Hangul (word processor)](https://en.wikipedia.org/wiki/Hangul_(word_processor))
- [Microsoft HWP Converter](https://www.microsoft.com/en-us/download/details.aspx?id=36772)
- [OnlyOffice HWP/HWPX 가이드](https://www.onlyoffice.com/blog/2025/02/how-to-open-hwp-and-hwpx-files)
- [online-convert.com HWP 포맷 설명](https://www.online-convert.com/file-format/hwp)
- [CloudConvert HWP 컨버터](https://cloudconvert.com/hwp-converter)
