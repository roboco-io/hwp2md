package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/roboco-io/hwp2markdown/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "설정 관리",
	Long: `hwp2markdown 설정을 관리합니다.

설정 파일 위치: ~/.hwp2markdown/config.yaml

하위 명령:
  show    현재 설정 표시
  init    기본 설정 파일 생성
  set     설정 값 변경
  path    설정 파일 경로 표시`,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "현재 설정 표시",
	Long: `현재 적용된 설정을 표시합니다.

환경 변수가 설정되어 있으면 해당 값이 적용됩니다.
설정 파일이 없으면 기본값이 표시됩니다.`,
	RunE: runConfigShow,
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "기본 설정 파일 생성",
	Long: `기본 설정 파일을 ~/.hwp2markdown/config.yaml에 생성합니다.

이미 설정 파일이 있는 경우 오류가 발생합니다.
기존 파일을 덮어쓰려면 --force 플래그를 사용하세요.`,
	RunE: runConfigInit,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "설정 값 변경",
	Long: `설정 값을 변경합니다.

지원하는 키:
  default_provider    기본 LLM 프로바이더 (anthropic, openai, gemini, ollama)
  format.temperature  LLM 온도 (0.0-1.0)
  format.language     출력 언어 (ko, en)

예시:
  hwp2markdown config set default_provider openai
  hwp2markdown config set format.temperature 0.5`,
	Args: cobra.ExactArgs(2),
	RunE: runConfigSet,
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "설정 파일 경로 표시",
	Run: func(cmd *cobra.Command, args []string) {
		loader, err := config.NewLoader()
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "오류: %v\n", err)
			return
		}
		fmt.Println(loader.ConfigPath())
	},
}

var configForce bool

func init() {
	configInitCmd.Flags().BoolVarP(&configForce, "force", "f", false, "기존 설정 파일 덮어쓰기")

	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configPathCmd)

	rootCmd.AddCommand(configCmd)
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	loader, err := config.NewLoader()
	if err != nil {
		return fmt.Errorf("설정 로더 초기화 실패: %w", err)
	}

	cfg, err := loader.LoadRaw()
	if err != nil {
		return fmt.Errorf("설정 로드 실패: %w", err)
	}

	// Show config file status
	if loader.Exists() {
		fmt.Fprintf(cmd.OutOrStdout(), "설정 파일: %s\n\n", loader.ConfigPath())
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "설정 파일: (기본값 사용)\n\n")
	}

	// Display as YAML
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("설정 출력 실패: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))

	// Show environment variable overrides
	fmt.Fprintln(cmd.OutOrStdout(), "환경 변수:")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	envVars := []struct {
		key   string
		desc  string
		value string
	}{
		{"HWP2MD_LLM", "LLM 활성화", os.Getenv("HWP2MD_LLM")},
		{"HWP2MD_MODEL", "모델 (프로바이더 자동 감지)", os.Getenv("HWP2MD_MODEL")},
		{"HWP2MD_BASE_URL", "프라이빗 API 엔드포인트", os.Getenv("HWP2MD_BASE_URL")},
		{"ANTHROPIC_API_KEY", "Anthropic API 키", maskAPIKey(os.Getenv("ANTHROPIC_API_KEY"))},
		{"OPENAI_API_KEY", "OpenAI API 키", maskAPIKey(os.Getenv("OPENAI_API_KEY"))},
		{"GOOGLE_API_KEY", "Google API 키", maskAPIKey(os.Getenv("GOOGLE_API_KEY"))},
		{"UPSTAGE_API_KEY", "Upstage API 키", maskAPIKey(os.Getenv("UPSTAGE_API_KEY"))},
		{"OLLAMA_HOST", "Ollama 호스트", os.Getenv("OLLAMA_HOST")},
	}

	for _, ev := range envVars {
		status := "(미설정)"
		if ev.value != "" {
			status = ev.value
		}
		fmt.Fprintf(w, "  %s\t%s\t%s\n", ev.key, ev.desc, status)
	}
	w.Flush()

	return nil
}

func runConfigInit(cmd *cobra.Command, args []string) error {
	loader, err := config.NewLoader()
	if err != nil {
		return fmt.Errorf("설정 로더 초기화 실패: %w", err)
	}

	if loader.Exists() && !configForce {
		return fmt.Errorf("설정 파일이 이미 존재합니다: %s\n덮어쓰려면 --force 플래그를 사용하세요", loader.ConfigPath())
	}

	if err := loader.Save(config.DefaultConfig()); err != nil {
		return fmt.Errorf("설정 파일 생성 실패: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "설정 파일 생성됨: %s\n", loader.ConfigPath())
	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	loader, err := config.NewLoader()
	if err != nil {
		return fmt.Errorf("설정 로더 초기화 실패: %w", err)
	}

	cfg, err := loader.LoadRaw()
	if err != nil {
		return fmt.Errorf("설정 로드 실패: %w", err)
	}

	// Update config based on key
	switch key {
	case "default_provider":
		validProviders := []string{"anthropic", "openai", "gemini", "ollama"}
		if !contains(validProviders, value) {
			return fmt.Errorf("유효하지 않은 프로바이더: %s (지원: %s)", value, strings.Join(validProviders, ", "))
		}
		cfg.DefaultProvider = value

	case "format.temperature":
		var temp float64
		if _, err := fmt.Sscanf(value, "%f", &temp); err != nil {
			return fmt.Errorf("유효하지 않은 온도 값: %s", value)
		}
		if temp < 0 || temp > 1 {
			return fmt.Errorf("온도는 0.0-1.0 범위여야 합니다: %f", temp)
		}
		cfg.Format.Temperature = temp

	case "format.language":
		validLanguages := []string{"ko", "en"}
		if !contains(validLanguages, value) {
			return fmt.Errorf("유효하지 않은 언어: %s (지원: %s)", value, strings.Join(validLanguages, ", "))
		}
		cfg.Format.Language = value

	default:
		return fmt.Errorf("알 수 없는 설정 키: %s\n지원하는 키: default_provider, format.temperature, format.language", key)
	}

	if err := loader.Save(cfg); err != nil {
		return fmt.Errorf("설정 저장 실패: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "설정 변경됨: %s = %s\n", key, value)
	return nil
}

func maskAPIKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
