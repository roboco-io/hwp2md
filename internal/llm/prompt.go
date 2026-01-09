package llm

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/roboco-io/hwp2markdown/internal/ir"
)

// SystemPrompt is the default system prompt for document formatting.
const SystemPrompt = `당신은 한글(HWP) 문서를 깔끔한 Markdown으로 변환하는 전문가입니다.

## 역할
주어진 JSON 형식의 문서 구조(IR)를 분석하여 가독성 높은 Markdown 문서로 변환합니다.

## 변환 규칙

### 문단 (paragraph)
- 빈 줄로 문단 구분
- 제목(heading_level > 0)은 적절한 # 레벨 사용
- 인용문(is_quote)은 > 블록인용 사용

### 표 (table)
- GFM 테이블 문법 사용
- 첫 행이 헤더면 구분선 추가
- 셀 내용은 줄바꿈을 공백으로 대체

### 이미지 (image)
- ![대체텍스트](경로) 형식 사용
- 대체텍스트가 없으면 이미지 ID 사용

### 목록 (list)
- 순서 있는 목록: 1. 2. 3.
- 순서 없는 목록: -
- 중첩 목록은 2칸 들여쓰기

### 텍스트 스타일
- 굵게: **텍스트**
- 기울임: *텍스트*
- 취소선: ~~텍스트~~
- 코드: ` + "`텍스트`" + `
- 링크: [텍스트](URL)

## 출력 형식
- Markdown만 출력 (설명이나 코드블록 없이)
- 원본 문서의 구조와 내용 보존
- 불필요한 빈 줄 제거`

// BuildPrompt creates the user prompt from an IR document.
func BuildPrompt(doc *ir.Document) (string, error) {
	// Convert document to JSON for LLM
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal document: %w", err)
	}

	return fmt.Sprintf("다음 JSON 형식의 문서 구조를 Markdown으로 변환해주세요:\n\n```json\n%s\n```", string(data)), nil
}

// BuildCompactPrompt creates a more compact prompt for efficiency.
func BuildCompactPrompt(doc *ir.Document) string {
	var sb strings.Builder

	sb.WriteString("다음 문서를 Markdown으로 변환:\n\n")

	// Metadata
	if doc.Metadata.Title != "" {
		sb.WriteString(fmt.Sprintf("제목: %s\n", doc.Metadata.Title))
	}
	if doc.Metadata.Author != "" {
		sb.WriteString(fmt.Sprintf("작성자: %s\n", doc.Metadata.Author))
	}
	if doc.Metadata.Title != "" || doc.Metadata.Author != "" {
		sb.WriteString("\n---\n\n")
	}

	// Content blocks
	for i, block := range doc.Content {
		if i > 0 {
			sb.WriteString("\n")
		}

		switch block.Type {
		case ir.BlockTypeParagraph:
			if block.Paragraph != nil {
				writeParagraphPrompt(&sb, block.Paragraph)
			}
		case ir.BlockTypeTable:
			if block.Table != nil {
				writeTablePrompt(&sb, block.Table)
			}
		case ir.BlockTypeImage:
			if block.Image != nil {
				writeImagePrompt(&sb, block.Image)
			}
		case ir.BlockTypeList:
			if block.List != nil {
				writeListPrompt(&sb, block.List)
			}
		}
	}

	return sb.String()
}

func writeParagraphPrompt(sb *strings.Builder, p *ir.Paragraph) {
	if p.Style.HeadingLevel > 0 {
		sb.WriteString(fmt.Sprintf("[제목 %d] ", p.Style.HeadingLevel))
	}
	if p.Style.IsQuote {
		sb.WriteString("[인용] ")
	}
	sb.WriteString(p.Text)
	sb.WriteString("\n")
}

func writeTablePrompt(sb *strings.Builder, t *ir.TableBlock) {
	sb.WriteString("[표]\n")
	for i, row := range t.Cells {
		if i == 0 && t.HasHeader {
			sb.WriteString("헤더: ")
		}
		for j, cell := range row {
			if j > 0 {
				sb.WriteString(" | ")
			}
			sb.WriteString(strings.ReplaceAll(cell.Text, "\n", " "))
		}
		sb.WriteString("\n")
	}
}

func writeImagePrompt(sb *strings.Builder, img *ir.ImageBlock) {
	alt := img.Alt
	if alt == "" {
		alt = img.ID
	}
	sb.WriteString(fmt.Sprintf("[이미지: %s, 경로: %s]\n", alt, img.Path))
}

func writeListPrompt(sb *strings.Builder, l *ir.ListBlock) {
	listType := "순서없는"
	if l.Ordered {
		listType = "순서있는"
	}
	sb.WriteString(fmt.Sprintf("[%s 목록]\n", listType))
	for _, item := range l.Items {
		sb.WriteString(fmt.Sprintf("- %s\n", item.Text))
	}
}
