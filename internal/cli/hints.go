package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"syscall"
)

// hintError attaches a troubleshooting hint to an error.
type hintError struct {
	err  error
	hint string
}

func (e *hintError) Error() string { return e.err.Error() }
func (e *hintError) Unwrap() error { return e.err }

// withHint wraps err with a hint. It returns nil when err is nil
// and returns err unchanged when hint is empty.
func withHint(err error, hint string) error {
	if err == nil || hint == "" {
		return err
	}
	return &hintError{err: err, hint: hint}
}

// apiKeyEnvVars maps a provider name to the environment variable for its API key.
var apiKeyEnvVars = map[string]string{
	"anthropic": "ANTHROPIC_API_KEY",
	"openai":    "OPENAI_API_KEY",
	"gemini":    "GOOGLE_API_KEY",
	"upstage":   "UPSTAGE_API_KEY",
}

// apiKeyHint returns a hint for a missing API key of the provider.
// It returns "" when the provider does not use an API key.
func apiKeyHint(provider string) string {
	env, ok := apiKeyEnvVars[provider]
	if !ok {
		return ""
	}
	return fmt.Sprintf("'export %s=your-key'로 API 키를 설정하세요.\n"+
		"      로컬 LLM을 쓰려면 --provider ollama를 사용하세요.", env)
}

// isAPIKeyError reports whether err is a missing API key error from a provider.
func isAPIKeyError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "API key not")
}

// errorHint returns the troubleshooting hint for err, or "" if none applies.
// An explicit hint attached with withHint takes precedence over detected causes.
func errorHint(err error) string {
	if err == nil {
		return ""
	}
	var he *hintError
	if errors.As(err, &he) {
		return he.hint
	}

	msg := strings.ToLower(err.Error())
	switch {
	case errors.Is(err, context.DeadlineExceeded),
		strings.Contains(msg, "deadline exceeded"),
		strings.Contains(msg, "client.timeout"):
		return "요청 시간이 초과되었습니다. --timeout 10m처럼 타임아웃을 늘리거나\n" +
			"      HWP2MD_TIMEOUT 환경 변수를 설정하세요."
	case errors.Is(err, syscall.ECONNREFUSED), strings.Contains(msg, "connection refused"):
		return "서버에 연결할 수 없습니다. --base-url 값을 확인하세요.\n" +
			"      Ollama를 사용한다면 'ollama serve'로 서버를 먼저 실행하세요."
	}
	return ""
}

// printError writes the error and its hint (if any) to w.
func printError(w io.Writer, err error) {
	fmt.Fprintf(w, "오류: %v\n", err)
	if hint := errorHint(err); hint != "" {
		fmt.Fprintf(w, "힌트: %s\n", hint)
	}
}
