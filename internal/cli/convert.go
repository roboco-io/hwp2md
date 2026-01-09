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
	"github.com/roboco-io/hwp2markdown/internal/llm/openai"
	"github.com/roboco-io/hwp2markdown/internal/parser"
	"github.com/roboco-io/hwp2markdown/internal/parser/hwpx"
	"github.com/spf13/cobra"
)

var (
	convertOutput      string
	convertUseLLM      bool
	convertProvider    string
	convertModel       string
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
  HWP2MD_PROVIDER=xxx   LLM 프로바이더 (openai, anthropic, gemini, ollama)
  HWP2MD_MODEL=xxx      모델 이름

예시:
  hwp2markdown convert document.hwpx
  hwp2markdown convert document.hwpx -o output.md
  hwp2markdown convert document.hwpx --llm --provider anthropic
  hwp2markdown convert document.hwpx --extract-images ./images`,
	Args: cobra.ExactArgs(1),
	RunE: runConvert,
}

func init() {
	convertCmd.Flags().StringVarP(&convertOutput, "output", "o", "", "출력 파일 경로 (기본: stdout)")
	convertCmd.Flags().BoolVar(&convertUseLLM, "llm", false, "LLM 포맷팅 활성화 (Stage 2)")
	convertCmd.Flags().StringVar(&convertProvider, "provider", "", "LLM 프로바이더 (openai, anthropic, gemini, ollama)")
	convertCmd.Flags().StringVar(&convertModel, "model", "", "LLM 모델 이름")
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

func formatWithLLM(cmd *cobra.Command, doc *ir.Document) (string, *llm.FormatResult, error) {
	// Determine provider
	providerName := convertProvider
	if providerName == "" {
		providerName = config.GetEnvOrDefault("HWP2MD_PROVIDER", "anthropic")
	}

	// Determine model
	model := convertModel
	if model == "" {
		model = os.Getenv("HWP2MD_MODEL")
	}

	// Create provider
	var provider llm.Provider
	var err error

	switch providerName {
	case "openai":
		provider, err = openai.New(openai.Config{
			Model: model,
		})
	case "anthropic":
		provider, err = anthropic.New(anthropic.Config{
			Model: model,
		})
	default:
		return "", nil, fmt.Errorf("지원하지 않는 프로바이더: %s (지원: openai, anthropic)", providerName)
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

	// Write rows
	for i, row := range t.Cells {
		sb.WriteString("|")
		for _, cell := range row {
			text := strings.ReplaceAll(cell.Text, "\n", " ")
			sb.WriteString(fmt.Sprintf(" %s |", text))
		}
		sb.WriteString("\n")

		// Write separator after header row
		if i == 0 {
			sb.WriteString("|")
			for range row {
				sb.WriteString(" --- |")
			}
			sb.WriteString("\n")
		}
	}
	sb.WriteString("\n")
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
