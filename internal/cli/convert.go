package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/roboco-io/hwp2markdown/internal/config"
	"github.com/roboco-io/hwp2markdown/internal/ir"
	"github.com/roboco-io/hwp2markdown/internal/llm"
	"github.com/roboco-io/hwp2markdown/internal/llm/anthropic"
	"github.com/roboco-io/hwp2markdown/internal/llm/gemini"
	"github.com/roboco-io/hwp2markdown/internal/llm/ollama"
	"github.com/roboco-io/hwp2markdown/internal/llm/openai"
	"github.com/roboco-io/hwp2markdown/internal/llm/upstage"
	"github.com/roboco-io/hwp2markdown/internal/parser"
	"github.com/roboco-io/hwp2markdown/internal/parser/hwpx"
	"github.com/spf13/cobra"
)

var (
	convertOutput      string
	convertUseLLM      bool
	convertProvider    string
	convertModel       string
	convertBaseURL     string
	convertExtractImgs bool
	convertImagesDir   string
	convertVerbose     bool
	convertQuiet       bool
)

var convertCmd = &cobra.Command{
	Use:   "convert <file>",
	Short: "HWP/HWPX 문서를 Markdown으로 변환",
	Long: `HWP/HWPX 문서를 Markdown으로 변환합니다.

기본적으로 Stage 1(파싱)만 수행하여 기본 Markdown을 생성합니다.
--llm 플래그를 사용하면 Stage 2(LLM 포맷팅)를 활성화하여
더 자연스러운 Markdown을 생성할 수 있습니다.

환경 변수:
  HWP2MD_LLM=true       Stage 2 활성화
  HWP2MD_MODEL=xxx      모델 이름 (프로바이더 자동 감지)
  HWP2MD_BASE_URL=xxx   프라이빗 API 엔드포인트 (Bedrock, 로컬 서버 등)

모델 이름 예시:
  claude-*              → Anthropic
  gpt-*                 → OpenAI
  gemini-*              → Google Gemini
  solar-*               → Upstage
  그 외                  → Ollama (로컬)

프라이빗 테넌시 예시:
  --base-url https://bedrock.us-east-1.amazonaws.com  # AWS Bedrock
  --base-url http://localhost:8080                     # 로컬 서버
  --base-url https://your-azure-endpoint.openai.azure.com  # Azure OpenAI

예시:
  hwp2markdown convert document.hwpx
  hwp2markdown convert document.hwpx -o output.md
  hwp2markdown convert document.hwpx --llm
  hwp2markdown convert document.hwpx --llm --model gpt-4o
  hwp2markdown convert document.hwpx --llm --model solar-pro
  hwp2markdown convert document.hwpx --llm --base-url http://localhost:8080
  hwp2markdown convert document.hwpx --extract-images ./images`,
	Args: cobra.ExactArgs(1),
	RunE: runConvert,
}

func init() {
	convertCmd.Flags().StringVarP(&convertOutput, "output", "o", "", "출력 파일 경로 (기본: stdout)")
	convertCmd.Flags().BoolVar(&convertUseLLM, "llm", false, "LLM 포맷팅 활성화 (Stage 2)")
	convertCmd.Flags().StringVar(&convertProvider, "provider", "", "LLM 프로바이더 (openai, anthropic, gemini, upstage, ollama)")
	convertCmd.Flags().StringVar(&convertModel, "model", "", "LLM 모델 이름")
	convertCmd.Flags().StringVar(&convertBaseURL, "base-url", "", "프라이빗 API 엔드포인트 (Bedrock, Azure, 로컬 서버 등)")
	convertCmd.Flags().BoolVar(&convertExtractImgs, "extract-images", false, "이미지 추출 활성화")
	convertCmd.Flags().StringVar(&convertImagesDir, "images-dir", "./images", "추출된 이미지 저장 디렉토리")
	convertCmd.Flags().BoolVarP(&convertVerbose, "verbose", "v", false, "상세 출력")
	convertCmd.Flags().BoolVarP(&convertQuiet, "quiet", "q", false, "조용한 모드")

	rootCmd.AddCommand(convertCmd)
}

func runConvert(cmd *cobra.Command, args []string) error {
	inputPath := args[0]

	// Check file exists
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		return fmt.Errorf("파일을 찾을 수 없습니다: %s", inputPath)
	}

	// Detect format
	format := parser.DetectFormat(inputPath)
	if format == parser.FormatUnknown {
		return fmt.Errorf("지원하지 않는 파일 형식입니다: %s", filepath.Ext(inputPath))
	}

	if !convertQuiet && convertVerbose {
		fmt.Fprintf(cmd.ErrOrStderr(), "입력 파일: %s\n", inputPath)
		fmt.Fprintf(cmd.ErrOrStderr(), "파일 형식: %s\n", format)
	}

	// Parse document (Stage 1)
	doc, err := parseDocumentForConvert(inputPath, format)
	if err != nil {
		return fmt.Errorf("문서 파싱 실패: %w", err)
	}

	if !convertQuiet && convertVerbose {
		fmt.Fprintf(cmd.ErrOrStderr(), "파싱 완료: %d 블록\n", len(doc.Content))
	}

	// Check if LLM should be used
	useLLM := convertUseLLM || config.GetEnvBool("HWP2MD_LLM")

	var markdown string
	if useLLM {
		if !convertQuiet {
			fmt.Fprintf(cmd.ErrOrStderr(), "LLM 포맷팅 중...\n")
		}
		// Stage 2: LLM formatting
		var result *llm.FormatResult
		markdown, result, err = formatWithLLM(cmd, doc)
		if err != nil {
			return fmt.Errorf("LLM 포맷팅 실패: %w", err)
		}
		// Show token usage
		if !convertQuiet && result != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "모델: %s\n", result.Model)
			fmt.Fprintf(cmd.ErrOrStderr(), "토큰 사용량: 입력 %d, 출력 %d, 총 %d\n",
				result.Usage.InputTokens, result.Usage.OutputTokens, result.Usage.TotalTokens)
		}
	} else {
		// Stage 1 only: Basic markdown conversion
		markdown = convertToBasicMarkdown(doc)
	}

	// Write output
	if convertOutput == "" {
		fmt.Println(markdown)
	} else {
		if err := os.WriteFile(convertOutput, []byte(markdown), 0644); err != nil {
			return fmt.Errorf("파일 저장 실패: %w", err)
		}
		if !convertQuiet {
			fmt.Fprintf(cmd.ErrOrStderr(), "변환 완료: %s\n", convertOutput)
		}
	}

	return nil
}

func parseDocumentForConvert(path string, format parser.Format) (*ir.Document, error) {
	opts := parser.Options{
		ExtractImages: convertExtractImgs,
		ImageDir:      convertImagesDir,
	}

	switch format {
	case parser.FormatHWPX:
		p, err := hwpx.New(path, opts)
		if err != nil {
			return nil, err
		}
		defer p.Close()
		return p.Parse()

	case parser.FormatHWP:
		return nil, fmt.Errorf("HWP 5.x 형식은 아직 지원하지 않습니다")

	default:
		return nil, fmt.Errorf("알 수 없는 형식: %s", format)
	}
}

// detectProviderFromModel auto-detects the LLM provider based on model name.
// Returns "anthropic" as default if model is empty or unrecognized.
func detectProviderFromModel(model string) string {
	model = strings.ToLower(model)

	switch {
	case model == "":
		// Default to anthropic when no model specified
		return "anthropic"
	case strings.HasPrefix(model, "claude"):
		return "anthropic"
	case strings.HasPrefix(model, "gpt"), strings.HasPrefix(model, "o1"), strings.HasPrefix(model, "o3"):
		return "openai"
	case strings.HasPrefix(model, "gemini"):
		return "gemini"
	case strings.HasPrefix(model, "solar"):
		return "upstage"
	default:
		// Unknown model names are assumed to be Ollama (local models)
		return "ollama"
	}
}

func formatWithLLM(cmd *cobra.Command, doc *ir.Document) (string, *llm.FormatResult, error) {
	// Determine model (from flag or env)
	model := convertModel
	if model == "" {
		model = os.Getenv("HWP2MD_MODEL")
	}

	// Determine base URL (from flag or env) for private tenancy
	baseURL := convertBaseURL
	if baseURL == "" {
		baseURL = os.Getenv("HWP2MD_BASE_URL")
	}

	// Auto-detect provider from model name, or use explicit flag
	providerName := convertProvider
	if providerName == "" {
		providerName = detectProviderFromModel(model)
	}

	// Create provider
	var provider llm.Provider
	var err error

	switch providerName {
	case "openai":
		provider, err = openai.New(openai.Config{
			Model:   model,
			BaseURL: baseURL,
		})
	case "anthropic":
		provider, err = anthropic.New(anthropic.Config{
			Model:   model,
			BaseURL: baseURL,
		})
	case "gemini":
		// Gemini does not support custom base URL (uses Google API only)
		provider, err = gemini.New(gemini.Config{
			Model: model,
		})
	case "upstage":
		provider, err = upstage.New(upstage.Config{
			Model:   model,
			BaseURL: baseURL,
		})
	case "ollama":
		provider, err = ollama.New(ollama.Config{
			Model:   model,
			BaseURL: baseURL,
		})
	default:
		return "", nil, fmt.Errorf("지원하지 않는 프로바이더: %s (지원: openai, anthropic, gemini, upstage, ollama)", providerName)
	}

	if err != nil {
		return "", nil, fmt.Errorf("프로바이더 초기화 실패: %w", err)
	}

	// Format options
	opts := llm.DefaultFormatOptions()

	// Call LLM
	ctx := context.Background()
	result, err := provider.Format(ctx, doc, opts)
	if err != nil {
		return "", nil, err
	}

	return result.Markdown, result, nil
}

func convertToBasicMarkdown(doc *ir.Document) string {
	var sb strings.Builder

	// Metadata as YAML front matter (optional)
	if doc.Metadata.Title != "" || doc.Metadata.Author != "" {
		sb.WriteString("---\n")
		if doc.Metadata.Title != "" {
			sb.WriteString(fmt.Sprintf("title: %s\n", doc.Metadata.Title))
		}
		if doc.Metadata.Author != "" {
			sb.WriteString(fmt.Sprintf("author: %s\n", doc.Metadata.Author))
		}
		sb.WriteString("---\n\n")
	}

	// Content
	for _, block := range doc.Content {
		switch block.Type {
		case ir.BlockTypeParagraph:
			if block.Paragraph != nil {
				writeMarkdownParagraph(&sb, block.Paragraph)
			}
		case ir.BlockTypeTable:
			if block.Table != nil {
				writeMarkdownTable(&sb, block.Table)
			}
		case ir.BlockTypeImage:
			if block.Image != nil {
				writeMarkdownImage(&sb, block.Image)
			}
		case ir.BlockTypeList:
			if block.List != nil {
				writeMarkdownList(&sb, block.List)
			}
		}
	}

	return sb.String()
}

func writeMarkdownParagraph(sb *strings.Builder, p *ir.Paragraph) {
	text := strings.TrimSpace(p.Text)
	if text == "" {
		return
	}

	// Handle headings
	if p.Style.HeadingLevel > 0 && p.Style.HeadingLevel <= 6 {
		prefix := strings.Repeat("#", p.Style.HeadingLevel)
		sb.WriteString(fmt.Sprintf("%s %s\n\n", prefix, text))
		return
	}

	sb.WriteString(text + "\n\n")
}

func writeMarkdownTable(sb *strings.Builder, t *ir.TableBlock) {
	if len(t.Cells) == 0 {
		return
	}

	// Check if this is an "info-box" style table that should be converted to list format
	if isInfoBoxTable(t) {
		writeInfoBoxAsText(sb, t)
		return
	}

	numCols := t.Cols
	numRows := len(t.Cells)

	// Build a grid that tracks which cells are occupied by rowSpan
	// occupiedBy[row][col] points to the original cell (row, col) that occupies this position
	type cellRef struct {
		row, col int
	}
	occupiedBy := make([][]cellRef, numRows)
	for i := range occupiedBy {
		occupiedBy[i] = make([]cellRef, numCols)
		for j := range occupiedBy[i] {
			occupiedBy[i][j] = cellRef{-1, -1}
		}
	}

	// Mark cells based on rowSpan and colSpan
	for i, row := range t.Cells {
		for j, cell := range row {
			if occupiedBy[i][j].row == -1 {
				// This cell is not occupied, mark it and its span area
				rowSpan := cell.RowSpan
				if rowSpan < 1 {
					rowSpan = 1
				}
				colSpan := cell.ColSpan
				if colSpan < 1 {
					colSpan = 1
				}

				for r := i; r < i+rowSpan && r < numRows; r++ {
					for c := j; c < j+colSpan && c < numCols; c++ {
						occupiedBy[r][c] = cellRef{i, j}
					}
				}
			}
		}
	}

	// Write rows
	for i := range t.Cells {
		sb.WriteString("|")
		for j := 0; j < numCols; j++ {
			ref := occupiedBy[i][j]
			var text string
			if ref.row == i && ref.col == j {
				// This is the original cell
				text = strings.ReplaceAll(t.Cells[i][j].Text, "\n", " ")
			} else {
				// This cell is covered by a span from another cell
				// Leave empty for Markdown compatibility
				text = ""
			}
			sb.WriteString(fmt.Sprintf(" %s |", text))
		}
		sb.WriteString("\n")

		// Write separator after header row
		if i == 0 {
			sb.WriteString("|")
			for j := 0; j < numCols; j++ {
				sb.WriteString(" --- |")
			}
			sb.WriteString("\n")
		}
	}
	sb.WriteString("\n")
}

// isInfoBoxTable detects "info-box" style tables that should be converted to text format.
// Pattern 1: A table with a title cell (containing brackets like [제목]) and a single content cell
// that spans the full width and contains bullet-like content (○, ※, -, etc.)
// Pattern 2: A single-column table with 【법령 제목】 style headers containing legal/regulatory content
func isInfoBoxTable(t *ir.TableBlock) bool {
	// Pattern 2: Single-column table with 【title】 pattern (legal/regulatory content)
	if t.Cols == 1 && len(t.Cells) >= 1 {
		for _, row := range t.Cells {
			for _, cell := range row {
				text := strings.TrimSpace(cell.Text)
				// Check for 【】 bracket pattern (commonly used for legal references)
				if strings.Contains(text, "【") && strings.Contains(text, "】") {
					return true
				}
			}
		}
	}

	// Pattern 1: Multi-column table with [title] pattern
	if len(t.Cells) < 2 || t.Cols < 2 {
		return false
	}

	// Look for a title cell with brackets pattern like [고려사항]
	var hasTitle bool
	var titleText string
	for _, row := range t.Cells {
		for _, cell := range row {
			text := strings.TrimSpace(cell.Text)
			if strings.HasPrefix(text, "[") && strings.HasSuffix(text, "]") {
				hasTitle = true
				titleText = text
				break
			}
		}
		if hasTitle {
			break
		}
	}

	if !hasTitle {
		return false
	}

	// Check if the last row has a cell that spans full width with bullet content
	lastRow := t.Cells[len(t.Cells)-1]
	for _, cell := range lastRow {
		if cell.ColSpan >= t.Cols-1 {
			text := strings.TrimSpace(cell.Text)
			// Check for bullet-like patterns
			if strings.Contains(text, "○") || strings.Contains(text, "※") ||
				strings.HasPrefix(text, "-") || strings.Contains(titleText, "고려사항") ||
				strings.Contains(titleText, "참고") || strings.Contains(titleText, "안내") {
				return true
			}
		}
	}

	return false
}

// writeInfoBoxAsText converts an info-box table to readable text format
func writeInfoBoxAsText(sb *strings.Builder, t *ir.TableBlock) {
	// Check if this is a 【】 pattern (legal/regulatory content)
	if t.Cols == 1 && len(t.Cells) >= 1 {
		for _, row := range t.Cells {
			for _, cell := range row {
				text := strings.TrimSpace(cell.Text)
				if strings.Contains(text, "【") && strings.Contains(text, "】") {
					writeLegalContentAsText(sb, text)
					return
				}
			}
		}
	}

	// Handle [title] pattern
	// Find and write title
	var title string
	for _, row := range t.Cells {
		for _, cell := range row {
			text := strings.TrimSpace(cell.Text)
			if strings.HasPrefix(text, "[") && strings.HasSuffix(text, "]") {
				title = text
				break
			}
		}
		if title != "" {
			break
		}
	}

	if title != "" {
		sb.WriteString("**" + title + "**\n\n")
	}

	// Find and write content from the last row (usually the full-width content cell)
	lastRow := t.Cells[len(t.Cells)-1]
	for _, cell := range lastRow {
		text := strings.TrimSpace(cell.Text)
		if text == "" {
			continue
		}

		// Process the content: convert ○ to bullet points, ※ to indented notes
		lines := strings.Split(text, "○")
		for i, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			if i == 0 && !strings.HasPrefix(text, "○") {
				// First part before any ○, write as-is
				sb.WriteString(line + "\n\n")
			} else {
				// Convert ○ items to bullet points
				// Split by ※ for sub-notes
				parts := strings.Split(line, "※")
				for j, part := range parts {
					part = strings.TrimSpace(part)
					if part == "" {
						continue
					}
					if j == 0 {
						sb.WriteString("- " + part + "\n")
					} else {
						sb.WriteString("  - ※ " + part + "\n")
					}
				}
			}
		}
		sb.WriteString("\n")
	}
}

// writeLegalContentAsText converts legal/regulatory content with 【title】 patterns to readable format
func writeLegalContentAsText(sb *strings.Builder, text string) {
	// Split by 【 to get each section
	sections := strings.Split(text, "【")

	for _, section := range sections {
		section = strings.TrimSpace(section)
		if section == "" {
			continue
		}

		// Find the closing 】 to extract title and content
		closingIdx := strings.Index(section, "】")
		if closingIdx == -1 {
			// No closing bracket, write as plain text
			sb.WriteString(section + "\n\n")
			continue
		}

		title := section[:closingIdx]
		content := strings.TrimSpace(section[closingIdx+len("】"):])

		// Write title as bold heading
		sb.WriteString("**【" + title + "】**\n\n")

		if content == "" {
			continue
		}

		// Process numbered items (1. 2. 3. etc.)
		// Use regex to split by numbered patterns like "1." "2." etc.
		processedContent := processNumberedContent(content)
		sb.WriteString(processedContent)
		sb.WriteString("\n")
	}
}

// processNumberedContent converts numbered content to markdown list format
func processNumberedContent(content string) string {
	var result strings.Builder

	// Simple approach: look for patterns like "1." "2." etc. at reasonable positions
	// and convert them to list items
	lines := splitByNumberedItems(content)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		result.WriteString(line + "\n")
	}

	return result.String()
}

// splitByNumberedItems splits content by numbered patterns (1. 2. 3. etc.)
func splitByNumberedItems(content string) []string {
	var result []string
	var current strings.Builder

	// Pattern: digit followed by period or digit followed by Korean period
	i := 0
	runes := []rune(content)
	for i < len(runes) {
		// Check for number pattern at start or after space/newline
		if i == 0 || runes[i-1] == ' ' || runes[i-1] == '\n' {
			// Check if this is a numbered item (digit followed by . or .)
			if i < len(runes)-1 && runes[i] >= '0' && runes[i] <= '9' {
				// Look ahead for period
				j := i + 1
				for j < len(runes) && runes[j] >= '0' && runes[j] <= '9' {
					j++
				}
				if j < len(runes) && runes[j] == '.' {
					// Found a numbered item
					if current.Len() > 0 {
						result = append(result, current.String())
						current.Reset()
					}
					current.WriteString(string(runes[i:j+1]) + " ")
					i = j + 1
					continue
				}
			}
		}
		current.WriteRune(runes[i])
		i++
	}

	if current.Len() > 0 {
		result = append(result, current.String())
	}

	return result
}

func writeMarkdownImage(sb *strings.Builder, img *ir.ImageBlock) {
	alt := img.Alt
	if alt == "" {
		alt = img.ID
	}
	path := img.Path
	if path == "" {
		path = img.ID
	}
	sb.WriteString(fmt.Sprintf("![%s](%s)\n\n", alt, path))
}

func writeMarkdownList(sb *strings.Builder, l *ir.ListBlock) {
	writeListItems(sb, l.Items, l.Ordered, 0)
	sb.WriteString("\n")
}

func writeListItems(sb *strings.Builder, items []ir.ListItem, ordered bool, depth int) {
	indent := strings.Repeat("  ", depth)
	for i, item := range items {
		prefix := "- "
		if ordered {
			prefix = fmt.Sprintf("%d. ", i+1)
		}
		sb.WriteString(fmt.Sprintf("%s%s%s\n", indent, prefix, item.Text))

		// Handle nested items
		if len(item.Children) > 0 {
			writeListItems(sb, item.Children, ordered, depth+1)
		}
	}
}
