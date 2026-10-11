package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync"
)

// parseLogLevel converts a level name (debug, info, warn, error) to slog.Level.
func parseLogLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("알 수 없는 로그 레벨: %q (지원: debug, info, warn, error)", s)
	}
}

// resolveLogLevel determines the log level in priority order:
// --log-level flag > -v/-q flags > HWP2MD_LOG_LEVEL env > info.
// -v maps to debug, -q maps to error.
func resolveLogLevel(flagVal string, verbose, quiet bool, envVal string) (slog.Level, error) {
	if strings.TrimSpace(flagVal) != "" {
		return parseLogLevel(flagVal)
	}
	if verbose {
		return slog.LevelDebug, nil
	}
	if quiet {
		return slog.LevelError, nil
	}
	if strings.TrimSpace(envVal) != "" {
		lvl, err := parseLogLevel(envVal)
		if err != nil {
			return 0, fmt.Errorf("HWP2MD_LOG_LEVEL: %w", err)
		}
		return lvl, nil
	}
	return slog.LevelInfo, nil
}

// newLogger returns a slog.Logger that writes human-readable lines to w.
func newLogger(w io.Writer, level slog.Leveler) *slog.Logger {
	return slog.New(&cliHandler{mu: &sync.Mutex{}, w: w, level: level})
}

// cliHandler is a slog.Handler for terminal output.
// It omits timestamps, prints info messages as-is, and prefixes other levels.
// Attributes are appended as key=value pairs.
type cliHandler struct {
	mu    *sync.Mutex
	w     io.Writer
	level slog.Leveler
	attrs []slog.Attr
}

func (h *cliHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level.Level()
}

func (h *cliHandler) Handle(_ context.Context, r slog.Record) error {
	var sb strings.Builder
	switch {
	case r.Level >= slog.LevelError:
		sb.WriteString("오류: ")
	case r.Level >= slog.LevelWarn:
		sb.WriteString("경고: ")
	case r.Level < slog.LevelInfo:
		sb.WriteString("[debug] ")
	}
	sb.WriteString(r.Message)

	writeAttr := func(a slog.Attr) bool {
		if a.Equal(slog.Attr{}) {
			return true
		}
		fmt.Fprintf(&sb, " %s=%s", a.Key, a.Value.Resolve().String())
		return true
	}
	for _, a := range h.attrs {
		writeAttr(a)
	}
	r.Attrs(writeAttr)
	sb.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.w, sb.String())
	return err
}

func (h *cliHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	n := *h
	n.attrs = append(slices.Clip(h.attrs), attrs...)
	return &n
}

// WithGroup is a no-op: the CLI does not use attribute groups.
func (h *cliHandler) WithGroup(string) slog.Handler {
	return h
}
