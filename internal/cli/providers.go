package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type providerInfo struct {
	Name         string
	DefaultModel string
	EnvKey       string
	Description  string
}

var providers = []providerInfo{
	{
		Name:         "anthropic",
		DefaultModel: "claude-sonnet-4-20250514",
		EnvKey:       "ANTHROPIC_API_KEY",
		Description:  "Anthropic Claude API",
	},
	{
		Name:         "openai",
		DefaultModel: "gpt-4o-mini",
		EnvKey:       "OPENAI_API_KEY",
		Description:  "OpenAI GPT API (Azure, Bedrock 호환)",
	},
	{
		Name:         "gemini",
		DefaultModel: "gemini-1.5-flash",
		EnvKey:       "GOOGLE_API_KEY",
		Description:  "Google Gemini API",
	},
	{
		Name:         "upstage",
		DefaultModel: "solar-pro",
		EnvKey:       "UPSTAGE_API_KEY",
		Description:  "Upstage Solar API",
	},
	{
		Name:         "ollama",
		DefaultModel: "llama3.2",
		EnvKey:       "OLLAMA_HOST",
		Description:  "Local Ollama server",
	},
}

var providersCmd = &cobra.Command{
	Use:   "providers",
	Short: "사용 가능한 LLM 프로바이더 목록",
	Long: `Stage 2 (LLM 포맷팅)에서 사용 가능한 LLM 프로바이더 목록을 표시합니다.

각 프로바이더는 해당 환경 변수에 API 키가 설정되어 있어야 사용할 수 있습니다.
(ollama는 로컬 서버로 API 키가 필요하지 않습니다)

사용 예시:
  hwp2md convert document.hwpx --llm --provider anthropic
  hwp2md convert document.hwpx --llm --provider openai --model gpt-4o`,
	Run: runProviders,
}

func init() {
	rootCmd.AddCommand(providersCmd)
}

func runProviders(cmd *cobra.Command, args []string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "프로바이더\t기본 모델\t환경 변수\t상태\t설명")
	fmt.Fprintln(w, "---------\t--------\t-------\t----\t----")

	for _, p := range providers {
		status := checkProviderStatus(p)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			p.Name, p.DefaultModel, p.EnvKey, status, p.Description)
	}
}

func checkProviderStatus(p providerInfo) string {
	if p.Name == "ollama" {
		// Ollama doesn't require API key
		return "✓ 사용가능"
	}

	if os.Getenv(p.EnvKey) != "" {
		return "✓ 설정됨"
	}
	return "✗ 미설정"
}
