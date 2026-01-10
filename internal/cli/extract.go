package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/roboco-io/hwp2md/internal/ir"
	"github.com/roboco-io/hwp2md/internal/parser"
	"github.com/roboco-io/hwp2md/internal/parser/hwpx"
	"github.com/spf13/cobra"
)

var (
	extractOutput      string
	extractFormat      string
	extractImagesFlag  bool
	extractImagesDir   string
	extractPrettyPrint bool
)

var extractCmd = &cobra.Command{
	Use:   "extract <file>",
	Short: "HWP/HWPX 문서에서 IR(중간 표현) 추출",
	Long: `HWP/HWPX 문서를 파싱하여 IR(Intermediate Representation)을 추출합니다.

Stage 1만 실행하며, LLM 포맷팅 없이 구조화된 데이터를 출력합니다.
출력 형식은 JSON 또는 텍스트(요약)를 지원합니다.

예시:
  hwp2md extract document.hwpx
  hwp2md extract document.hwpx -o output.json
  hwp2md extract document.hwpx --format text
  hwp2md extract document.hwpx --extract-images ./images`,
	Args: cobra.ExactArgs(1),
	RunE: runExtract,
}

func init() {
	extractCmd.Flags().StringVarP(&extractOutput, "output", "o", "", "출력 파일 경로 (기본: stdout)")
	extractCmd.Flags().StringVarP(&extractFormat, "format", "f", "json", "출력 형식 (json, text)")
	extractCmd.Flags().BoolVar(&extractImagesFlag, "extract-images", false, "이미지 추출 활성화")
	extractCmd.Flags().StringVar(&extractImagesDir, "images-dir", "./images", "추출된 이미지 저장 디렉토리")
	extractCmd.Flags().BoolVar(&extractPrettyPrint, "pretty", true, "JSON 들여쓰기 적용")

	rootCmd.AddCommand(extractCmd)
}

func runExtract(cmd *cobra.Command, args []string) error {
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

	// Parse document
	doc, err := parseDocument(inputPath, format)
	if err != nil {
		return fmt.Errorf("문서 파싱 실패: %w", err)
	}

	// Format output
	output, err := formatOutput(doc, extractFormat)
	if err != nil {
		return fmt.Errorf("출력 포맷팅 실패: %w", err)
	}

	// Write output
	if extractOutput == "" {
		fmt.Println(output)
	} else {
		if err := os.WriteFile(extractOutput, []byte(output), 0644); err != nil {
			return fmt.Errorf("파일 저장 실패: %w", err)
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "IR 추출 완료: %s\n", extractOutput)
	}

	return nil
}

func parseDocument(path string, format parser.Format) (*ir.Document, error) {
	opts := parser.Options{
		ExtractImages: extractImagesFlag,
		ImageDir:      extractImagesDir,
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

func formatOutput(doc *ir.Document, format string) (string, error) {
	switch format {
	case "json":
		var data []byte
		var err error
		if extractPrettyPrint {
			data, err = json.MarshalIndent(doc, "", "  ")
		} else {
			data, err = json.Marshal(doc)
		}
		if err != nil {
			return "", err
		}
		return string(data), nil

	case "text":
		return formatAsText(doc), nil

	default:
		return "", fmt.Errorf("지원하지 않는 출력 형식: %s", format)
	}
}

func formatAsText(doc *ir.Document) string {
	var result string

	// Metadata
	if doc.Metadata.Title != "" {
		result += fmt.Sprintf("제목: %s\n", doc.Metadata.Title)
	}
	if doc.Metadata.Author != "" {
		result += fmt.Sprintf("작성자: %s\n", doc.Metadata.Author)
	}
	if result != "" {
		result += "\n---\n\n"
	}

	// Content
	for _, block := range doc.Content {
		switch block.Type {
		case ir.BlockTypeParagraph:
			if block.Paragraph != nil {
				result += block.Paragraph.Text + "\n\n"
			}
		case ir.BlockTypeTable:
			if block.Table != nil {
				result += formatTableAsText(block.Table) + "\n"
			}
		case ir.BlockTypeImage:
			if block.Image != nil {
				alt := block.Image.Alt
				if alt == "" {
					alt = block.Image.ID
				}
				result += fmt.Sprintf("[이미지: %s]\n\n", alt)
			}
		case ir.BlockTypeList:
			if block.List != nil {
				result += formatListAsText(block.List) + "\n"
			}
		}
	}

	return result
}

func formatTableAsText(table *ir.TableBlock) string {
	var result string
	for i, row := range table.Cells {
		for j, cell := range row {
			if j > 0 {
				result += " | "
			}
			result += cell.Text
		}
		result += "\n"
		if i == 0 && table.HasHeader {
			for j := range row {
				if j > 0 {
					result += " | "
				}
				result += "---"
			}
			result += "\n"
		}
	}
	return result
}

func formatListAsText(list *ir.ListBlock) string {
	var result string
	for i, item := range list.Items {
		prefix := "- "
		if list.Ordered {
			prefix = fmt.Sprintf("%d. ", i+1)
		}
		result += prefix + item.Text + "\n"
	}
	return result
}
