package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestResolveLogLevel(t *testing.T) {
	tests := []struct {
		name    string
		flag    string
		verbose bool
		quiet   bool
		env     string
		want    slog.Level
		wantErr bool
	}{
		{name: "default", want: slog.LevelInfo},
		{name: "verbose", verbose: true, want: slog.LevelDebug},
		{name: "quiet", quiet: true, want: slog.LevelError},
		{name: "env", env: "warn", want: slog.LevelWarn},
		{name: "verbose overrides env", verbose: true, env: "error", want: slog.LevelDebug},
		{name: "flag overrides verbose", flag: "warn", verbose: true, want: slog.LevelWarn},
		{name: "flag case insensitive", flag: "DEBUG", want: slog.LevelDebug},
		{name: "invalid flag", flag: "loud", wantErr: true},
		{name: "invalid env", env: "loud", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveLogLevel(tt.flag, tt.verbose, tt.quiet, tt.env)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got level %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCLIHandler(t *testing.T) {
	var buf bytes.Buffer
	logger := newLogger(&buf, slog.LevelDebug).With("file", "a.hwp")

	logger.Debug("파싱 완료", "blocks", 3)
	logger.Info("변환 완료")
	logger.Warn("주의")
	logger.Error("실패")

	want := "[debug] 파싱 완료 file=a.hwp blocks=3\n" +
		"변환 완료 file=a.hwp\n" +
		"경고: 주의 file=a.hwp\n" +
		"오류: 실패 file=a.hwp\n"
	if got := buf.String(); got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestCLIHandlerLevelFilter(t *testing.T) {
	var buf bytes.Buffer
	logger := newLogger(&buf, slog.LevelError)
	logger.Debug("d")
	logger.Info("i")
	logger.Warn("w")
	if buf.Len() != 0 {
		t.Errorf("expected no output below error level, got %q", buf.String())
	}
}

func TestErrorHint(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string // substring; "" means no hint
	}{
		{name: "nil", err: nil},
		{name: "plain", err: errors.New("boom")},
		{name: "explicit", err: withHint(errors.New("x"), "do this"), want: "do this"},
		{name: "wrapped explicit", err: fmt.Errorf("outer: %w", withHint(errors.New("x"), "do this")), want: "do this"},
		{name: "deadline", err: fmt.Errorf("API error: %w", context.DeadlineExceeded), want: "--timeout"},
		{name: "client timeout text", err: errors.New("Client.Timeout exceeded while awaiting headers"), want: "--timeout"},
		{name: "conn refused", err: fmt.Errorf("dial: %w", syscall.ECONNREFUSED), want: "ollama serve"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := errorHint(tt.err)
			if tt.want == "" {
				if got != "" {
					t.Errorf("expected no hint, got %q", got)
				}
				return
			}
			if !strings.Contains(got, tt.want) {
				t.Errorf("hint %q does not contain %q", got, tt.want)
			}
		})
	}
}

func TestWithHint(t *testing.T) {
	if withHint(nil, "h") != nil {
		t.Error("withHint(nil) should be nil")
	}
	base := errors.New("base")
	if withHint(base, "") != base {
		t.Error("empty hint should return err unchanged")
	}
	wrapped := withHint(base, "h")
	if !errors.Is(wrapped, base) {
		t.Error("hint error should unwrap to base")
	}
	if wrapped.Error() != "base" {
		t.Errorf("message changed: %q", wrapped.Error())
	}
}

func TestAPIKeyHint(t *testing.T) {
	for provider, env := range apiKeyEnvVars {
		if h := apiKeyHint(provider); !strings.Contains(h, env) {
			t.Errorf("%s hint %q missing %s", provider, h, env)
		}
	}
	if h := apiKeyHint("ollama"); h != "" {
		t.Errorf("ollama should have no API key hint, got %q", h)
	}
	if !isAPIKeyError(errors.New("Anthropic API key not configured (set ANTHROPIC_API_KEY)")) {
		t.Error("expected API key error to be detected")
	}
	if isAPIKeyError(errors.New("network down")) {
		t.Error("unexpected API key error detection")
	}
}

func TestPrintError(t *testing.T) {
	var buf bytes.Buffer
	printError(&buf, withHint(errors.New("ANTHROPIC_API_KEY not set"), apiKeyHint("anthropic")))
	out := buf.String()
	if !strings.HasPrefix(out, "오류: ANTHROPIC_API_KEY not set\n힌트: ") {
		t.Errorf("unexpected output: %q", out)
	}

	buf.Reset()
	printError(&buf, errors.New("boom"))
	if buf.String() != "오류: boom\n" {
		t.Errorf("unexpected output without hint: %q", buf.String())
	}
}

func TestProgressDisabled(t *testing.T) {
	var buf bytes.Buffer
	p := startProgress(&buf, "작업 중...", false, true)
	p.Stop()
	if buf.Len() != 0 {
		t.Errorf("disabled progress wrote %q", buf.String())
	}
}

func TestProgressNonTerminal(t *testing.T) {
	var buf bytes.Buffer
	p := startProgress(&buf, "작업 중...", true, false)
	p.Stop()
	if buf.String() != "작업 중...\n" {
		t.Errorf("got %q", buf.String())
	}
}

func TestProgressAnimated(t *testing.T) {
	var buf bytes.Buffer
	p := startProgress(&buf, "작업 중...", true, true)
	time.Sleep(250 * time.Millisecond)
	p.Stop()
	p.Stop() // second Stop must be safe

	out := buf.String()
	if !strings.Contains(out, "작업 중... (0s)") {
		t.Errorf("spinner line missing: %q", out)
	}
	// The last write clears the line and returns the cursor to column 0.
	if !strings.HasSuffix(out, " \r") {
		t.Errorf("spinner line not cleared: %q", out)
	}
}
