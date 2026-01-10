package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "hwp2md [file]",
	Short: "HWP/HWPX 문서를 Markdown으로 변환",
	Long: `hwp2md은 HWP(한글 워드프로세서) 문서를 Markdown으로 변환하는 CLI 도구입니다.

지원 포맷:
  - HWPX (XML 기반 개방형 포맷)
  - HWP 5.x (OLE/Compound 바이너리 포맷)

사용 예시:
  hwp2md document.hwpx
  hwp2md document.hwpx -o output.md
  hwp2md document.hwpx --llm
  hwp2md convert document.hwpx --extract-images ./images`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// 파일 인자가 있으면 convert 실행
		if len(args) == 1 && isHWPFile(args[0]) {
			return runConvert(cmd, args)
		}
		// 아니면 help 출력
		return cmd.Help()
	},
}

// isHWPFile checks if the file has HWP/HWPX extension
func isHWPFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".hwp") || strings.HasSuffix(lower, ".hwpx")
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "버전 정보 출력",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("hwp2md %s\n", version)
	},
}

func SetVersion(v string) {
	version = v
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(versionCmd)

	// Root 명령어에도 convert 플래그 추가 (파일만 넘길 때 사용)
	rootCmd.Flags().StringVarP(&convertOutput, "output", "o", "", "출력 파일 경로 (기본: stdout)")
	rootCmd.Flags().BoolVar(&convertUseLLM, "llm", false, "LLM 포맷팅 활성화 (Stage 2)")
	rootCmd.Flags().StringVar(&convertProvider, "provider", "", "LLM 프로바이더 (openai, anthropic, gemini, ollama)")
	rootCmd.Flags().StringVar(&convertModel, "model", "", "LLM 모델 이름")
	rootCmd.Flags().BoolVar(&convertExtractImgs, "extract-images", false, "이미지 추출 활성화")
	rootCmd.Flags().StringVar(&convertImagesDir, "images-dir", "./images", "추출된 이미지 저장 디렉토리")
	rootCmd.Flags().BoolVarP(&convertVerbose, "verbose", "v", false, "상세 출력")
	rootCmd.Flags().BoolVarP(&convertQuiet, "quiet", "q", false, "조용한 모드")
}
