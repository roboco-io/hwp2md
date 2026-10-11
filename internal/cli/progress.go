package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// spinnerFrames uses ASCII only so legacy Windows consoles render it correctly.
var spinnerFrames = []string{"|", "/", "-", "\\"}

// progress shows a waiting indicator for long-running steps (LLM, API calls).
// On a terminal it animates a spinner with elapsed time on one line.
// On a non-terminal (pipe, file) it prints the message once.
type progress struct {
	w     io.Writer
	msg   string
	start time.Time
	stop  chan struct{}
	done  chan struct{}
}

// startProgress starts the indicator. When enabled is false it prints nothing.
func startProgress(w io.Writer, msg string, enabled, animate bool) *progress {
	p := &progress{w: w, msg: msg, start: time.Now()}
	if !enabled {
		return p
	}
	if !animate {
		fmt.Fprintln(w, msg)
		return p
	}
	p.stop = make(chan struct{})
	p.done = make(chan struct{})
	go p.run(100 * time.Millisecond)
	return p
}

func (p *progress) run(interval time.Duration) {
	defer close(p.done)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	lastLen := 0
	for i := 0; ; i++ {
		elapsed := time.Since(p.start).Truncate(time.Second)
		line := fmt.Sprintf("%s %s (%s)", spinnerFrames[i%len(spinnerFrames)], p.msg, elapsed)
		// Pad with spaces instead of ANSI erase codes so the previous line is
		// fully overwritten on consoles without VT support.
		pad := max(lastLen-len(line), 0)
		fmt.Fprintf(p.w, "\r%s%s", line, strings.Repeat(" ", pad))
		lastLen = len(line)

		select {
		case <-p.stop:
			fmt.Fprintf(p.w, "\r%s\r", strings.Repeat(" ", lastLen))
			return
		case <-ticker.C:
		}
	}
}

// Stop ends the indicator, clears the spinner line, and returns the elapsed time.
func (p *progress) Stop() time.Duration {
	if p.stop != nil {
		close(p.stop)
		<-p.done
		p.stop = nil
	}
	return time.Since(p.start)
}

// isTerminal reports whether w is an interactive terminal.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok || os.Getenv("TERM") == "dumb" {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
