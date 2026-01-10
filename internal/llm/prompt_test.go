package llm

import (
	"strings"
	"testing"

	"github.com/roboco-io/hwp2md/internal/ir"
)

func TestBuildPrompt(t *testing.T) {
	doc := ir.NewDocument()
	doc.Metadata.Title = "테스트 문서"
	doc.AddParagraph(ir.NewParagraph("안녕하세요"))

	prompt, err := BuildPrompt(doc)
	if err != nil {
		t.Fatalf("failed to build prompt: %v", err)
	}

	if !strings.Contains(prompt, "테스트 문서") {
		t.Error("expected prompt to contain document title")
	}
	if !strings.Contains(prompt, "안녕하세요") {
		t.Error("expected prompt to contain paragraph text")
	}
	if !strings.Contains(prompt, "json") {
		t.Error("expected prompt to contain json marker")
	}
}

func TestBuildCompactPrompt(t *testing.T) {
	doc := ir.NewDocument()
	doc.Metadata.Title = "테스트"
	doc.Metadata.Author = "작성자"

	p := ir.NewParagraph("본문 텍스트")
	doc.AddParagraph(p)

	prompt := BuildCompactPrompt(doc)

	if !strings.Contains(prompt, "제목: 테스트") {
		t.Error("expected prompt to contain title")
	}
	if !strings.Contains(prompt, "작성자: 작성자") {
		t.Error("expected prompt to contain author")
	}
	if !strings.Contains(prompt, "본문 텍스트") {
		t.Error("expected prompt to contain paragraph text")
	}
}

func TestBuildCompactPrompt_WithHeading(t *testing.T) {
	doc := ir.NewDocument()
	p := ir.NewParagraph("제목입니다")
	p.SetHeading(1)
	doc.AddParagraph(p)

	prompt := BuildCompactPrompt(doc)

	if !strings.Contains(prompt, "[제목 1]") {
		t.Error("expected prompt to contain heading marker")
	}
}

func TestBuildCompactPrompt_WithTable(t *testing.T) {
	doc := ir.NewDocument()
	table := ir.NewTable(2, 2)
	table.SetCell(0, 0, "A1")
	table.SetCell(0, 1, "B1")
	table.SetCell(1, 0, "A2")
	table.SetCell(1, 1, "B2")
	table.SetHeaderRow()
	doc.AddTable(table)

	prompt := BuildCompactPrompt(doc)

	if !strings.Contains(prompt, "[표]") {
		t.Error("expected prompt to contain table marker")
	}
	if !strings.Contains(prompt, "헤더:") {
		t.Error("expected prompt to contain header marker")
	}
	if !strings.Contains(prompt, "A1") {
		t.Error("expected prompt to contain cell content")
	}
}

func TestBuildCompactPrompt_WithImage(t *testing.T) {
	doc := ir.NewDocument()
	img := ir.NewImage("img001")
	img.Alt = "테스트 이미지"
	img.Path = "images/test.png"
	doc.AddImage(img)

	prompt := BuildCompactPrompt(doc)

	if !strings.Contains(prompt, "[이미지:") {
		t.Error("expected prompt to contain image marker")
	}
	if !strings.Contains(prompt, "테스트 이미지") {
		t.Error("expected prompt to contain alt text")
	}
}

func TestBuildCompactPrompt_WithList(t *testing.T) {
	doc := ir.NewDocument()
	list := ir.NewUnorderedList()
	list.AddItem("항목 1")
	list.AddItem("항목 2")
	doc.AddList(list)

	prompt := BuildCompactPrompt(doc)

	if !strings.Contains(prompt, "[순서없는 목록]") {
		t.Error("expected prompt to contain list marker")
	}
	if !strings.Contains(prompt, "항목 1") {
		t.Error("expected prompt to contain list item")
	}
}

func TestBuildCompactPrompt_WithOrderedList(t *testing.T) {
	doc := ir.NewDocument()
	list := ir.NewOrderedList()
	list.AddItem("첫 번째")
	list.AddItem("두 번째")
	doc.AddList(list)

	prompt := BuildCompactPrompt(doc)

	if !strings.Contains(prompt, "[순서있는 목록]") {
		t.Error("expected prompt to contain ordered list marker")
	}
}

func TestSystemPrompt(t *testing.T) {
	if SystemPrompt == "" {
		t.Error("SystemPrompt should not be empty")
	}
	if !strings.Contains(SystemPrompt, "Markdown") {
		t.Error("SystemPrompt should mention Markdown")
	}
	if !strings.Contains(SystemPrompt, "GFM") {
		t.Error("SystemPrompt should mention GFM table format")
	}
}
