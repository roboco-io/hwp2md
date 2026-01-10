package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// binaryName returns the appropriate binary name for the current OS
func binaryName() string {
	if runtime.GOOS == "windows" {
		return "hwp2md_test.exe"
	}
	return "hwp2md_test"
}

// buildTestBinary builds the test binary and returns a cleanup function
func buildTestBinary(t *testing.T) (string, func()) {
	t.Helper()
	binName := binaryName()
	buildCmd := exec.Command("go", "build", "-o", binName, "../cmd/hwp2md")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("failed to build binary: %v", err)
	}
	return binName, func() { os.Remove(binName) }
}

func TestConvertCommand(t *testing.T) {
	// Find the sample HWPX file
	fixtureDir := "fixtures"
	sampleFile := filepath.Join(fixtureDir, "sample.hwpx")

	if _, err := os.Stat(sampleFile); os.IsNotExist(err) {
		t.Skipf("sample file not found: %s", sampleFile)
	}

	binPath, cleanup := buildTestBinary(t)
	defer cleanup()

	tests := []struct {
		name       string
		args       []string
		wantErr    bool
		wantOutput []string
	}{
		{
			name:    "basic convert",
			args:    []string{"convert", sampleFile},
			wantErr: false,
		},
		{
			name:    "convert with verbose",
			args:    []string{"convert", sampleFile, "-v"},
			wantErr: false,
		},
		{
			name:    "convert non-existent file",
			args:    []string{"convert", "nonexistent.hwpx"},
			wantErr: true,
		},
		{
			name:    "convert unsupported format",
			args:    []string{"convert", "test.txt"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command("./"+binPath, tc.args...)
			output, err := cmd.CombinedOutput()

			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v\noutput: %s", err, output)
				}
			}

			for _, want := range tc.wantOutput {
				if !strings.Contains(string(output), want) {
					t.Errorf("output should contain %q, got: %s", want, output)
				}
			}
		})
	}
}

func TestExtractCommand(t *testing.T) {
	fixtureDir := "fixtures"
	sampleFile := filepath.Join(fixtureDir, "sample.hwpx")

	if _, err := os.Stat(sampleFile); os.IsNotExist(err) {
		t.Skipf("sample file not found: %s", sampleFile)
	}

	binPath, cleanup := buildTestBinary(t)
	defer cleanup()

	tests := []struct {
		name       string
		args       []string
		wantErr    bool
		wantFormat string
	}{
		{
			name:       "extract as json",
			args:       []string{"extract", sampleFile},
			wantErr:    false,
			wantFormat: "json",
		},
		{
			name:       "extract as text",
			args:       []string{"extract", sampleFile, "--format", "text"},
			wantErr:    false,
			wantFormat: "text",
		},
		{
			name:    "extract non-existent file",
			args:    []string{"extract", "nonexistent.hwpx"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command("./"+binPath, tc.args...)
			output, err := cmd.CombinedOutput()

			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v\noutput: %s", err, output)
				}

				if tc.wantFormat == "json" && !strings.Contains(string(output), "{") {
					t.Errorf("expected JSON output, got: %s", output)
				}
			}
		})
	}
}

func TestProvidersCommand(t *testing.T) {
	binPath, cleanup := buildTestBinary(t)
	defer cleanup()

	cmd := exec.Command("./"+binPath, "providers")
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Errorf("unexpected error: %v\noutput: %s", err, output)
	}

	// Check that all providers are listed
	providers := []string{"anthropic", "openai", "gemini", "ollama"}
	for _, p := range providers {
		if !strings.Contains(string(output), p) {
			t.Errorf("output should contain provider %q, got: %s", p, output)
		}
	}
}

func TestVersionCommand(t *testing.T) {
	binPath, cleanup := buildTestBinary(t)
	defer cleanup()

	cmd := exec.Command("./"+binPath, "version")
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Errorf("unexpected error: %v\noutput: %s", err, output)
	}

	if !strings.Contains(string(output), "hwp2md") {
		t.Errorf("output should contain 'hwp2md', got: %s", output)
	}
}

func TestConfigCommand(t *testing.T) {
	binPath, cleanup := buildTestBinary(t)
	defer cleanup()

	t.Run("config show", func(t *testing.T) {
		cmd := exec.Command("./"+binPath, "config", "show")
		output, err := cmd.CombinedOutput()

		if err != nil {
			t.Errorf("unexpected error: %v\noutput: %s", err, output)
		}

		if !strings.Contains(string(output), "default_provider") {
			t.Errorf("output should contain 'default_provider', got: %s", output)
		}
	})

	t.Run("config path", func(t *testing.T) {
		cmd := exec.Command("./"+binPath, "config", "path")
		output, err := cmd.CombinedOutput()

		if err != nil {
			t.Errorf("unexpected error: %v\noutput: %s", err, output)
		}

		if !strings.Contains(string(output), "config.yaml") {
			t.Errorf("output should contain 'config.yaml', got: %s", output)
		}
	})
}

func TestHelpCommand(t *testing.T) {
	binPath, cleanup := buildTestBinary(t)
	defer cleanup()

	cmd := exec.Command("./"+binPath, "--help")
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Errorf("unexpected error: %v\noutput: %s", err, output)
	}

	expectedStrings := []string{"hwp2md", "convert", "extract", "providers", "config"}
	for _, s := range expectedStrings {
		if !strings.Contains(string(output), s) {
			t.Errorf("output should contain %q, got: %s", s, output)
		}
	}
}
