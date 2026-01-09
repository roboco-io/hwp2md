package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// E2E Test for Stage 1: HWPX -> Basic Markdown
// Verifies that converting testdata/한글 테스트.hwpx produces valid markdown with expected content

func TestE2EStage1_HWPXToMarkdown(t *testing.T) {
	// Find test files
	testdataDir := filepath.Join("..", "testdata")
	inputFile := filepath.Join(testdataDir, "한글 테스트.hwpx")

	// Check if test files exist
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		t.Skipf("input file not found: %s", inputFile)
	}

	// Build test binary
	binPath, cleanup := buildTestBinary(t)
	defer cleanup()

	// Run convert command (Stage 1 only - no --llm flag)
	cmd := exec.Command("./"+binPath, "convert", inputFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("convert command failed: %v\noutput: %s", err, output)
	}

	actualMD := string(output)

	// Validate Stage 1 output structure and content
	if err := validateStage1Output(t, actualMD); err != nil {
		t.Errorf("Stage 1 validation failed: %v", err)
	}
}

// validateStage1Output checks that Stage 1 (parser) output contains expected content
func validateStage1Output(t *testing.T, md string) error {
	t.Helper()

	// Check for required content that must be present in the parsed document
	requiredContent := []string{
		"문화체육관광부",
		"경력경쟁채용",
		"임용예정",
		"전문임기제",
		"종무",
		"불교",
		"천주교",
		"개신교",
		"세종",
		"응시",
	}

	for _, content := range requiredContent {
		if !strings.Contains(md, content) {
			t.Errorf("Stage 1 output missing required content: %s", content)
		}
	}

	// Check that output contains markdown table syntax
	if !strings.Contains(md, "|") {
		t.Error("Stage 1 output should contain markdown tables (| syntax)")
	}

	// Check for table separator rows
	if !strings.Contains(md, "---") {
		t.Error("Stage 1 output should contain table separator rows")
	}

	// Check minimum content length (document should have substantial content)
	if len(md) < 5000 {
		t.Errorf("Stage 1 output too short: %d chars (expected at least 5000)", len(md))
	}

	return nil
}

// E2E Test for Stage 2: Validates structure of LLM-formatted markdown
// Since LLM output is non-deterministic, we validate structure rather than exact match

func TestE2EStage2_LLMFormattedMarkdown(t *testing.T) {
	// Skip if no API key is available (for CI without secrets)
	if os.Getenv("ANTHROPIC_API_KEY") == "" &&
		os.Getenv("OPENAI_API_KEY") == "" &&
		os.Getenv("GOOGLE_API_KEY") == "" {
		t.Skip("skipping Stage 2 test: no LLM API key available")
	}

	testdataDir := filepath.Join("..", "testdata")
	inputFile := filepath.Join(testdataDir, "한글 테스트.hwpx")
	expectedFormattedFile := filepath.Join(testdataDir, "expected_formatted.md")

	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		t.Skipf("input file not found: %s", inputFile)
	}
	if _, err := os.Stat(expectedFormattedFile); os.IsNotExist(err) {
		t.Skipf("expected formatted file not found: %s", expectedFormattedFile)
	}

	binPath, cleanup := buildTestBinary(t)
	defer cleanup()

	// Run convert with LLM (Stage 2)
	cmd := exec.Command("./"+binPath, "convert", inputFile, "--llm")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("convert --llm command failed: %v\noutput: %s", err, output)
	}

	// Validate structure
	actualMD := string(output)
	if err := validateFormattedMarkdownStructure(t, actualMD); err != nil {
		t.Errorf("Stage 2 structural validation failed: %v", err)
	}
}

// TestE2EStage2_StructuralValidation validates the expected_formatted.md structure
// This can run without API keys as it only checks the reference file
func TestE2EStage2_StructuralValidation(t *testing.T) {
	testdataDir := filepath.Join("..", "testdata")
	expectedFormattedFile := filepath.Join(testdataDir, "expected_formatted.md")

	if _, err := os.Stat(expectedFormattedFile); os.IsNotExist(err) {
		t.Skipf("expected formatted file not found: %s", expectedFormattedFile)
	}

	content, err := os.ReadFile(expectedFormattedFile)
	if err != nil {
		t.Fatalf("failed to read expected formatted file: %v", err)
	}

	if err := validateFormattedMarkdownStructure(t, string(content)); err != nil {
		t.Errorf("expected_formatted.md structural validation failed: %v", err)
	}
}

// validateFormattedMarkdownStructure checks that the markdown has expected structure
func validateFormattedMarkdownStructure(t *testing.T, md string) error {
	t.Helper()

	// Core keywords that must be present (relaxed for LLM variability)
	coreKeywords := []string{
		"문화체육관광부",
		"임용",
		"전문임기제",
	}

	for _, keyword := range coreKeywords {
		if !strings.Contains(md, keyword) {
			t.Errorf("missing core keyword: %s", keyword)
		}
	}

	// Check for markdown structure elements (relaxed counts)
	checks := []struct {
		name     string
		pattern  string
		minCount int
	}{
		{"headings", `(?m)^#{1,6}\s+.+$`, 3},              // At least 3 headings
		{"tables", `\|.*\|`, 2},                           // At least 2 table rows
		{"list items", `(?m)^[-*]\s+.+$|^\d+\.\s+.+$`, 2}, // At least 2 list items
	}

	for _, check := range checks {
		re := regexp.MustCompile(check.pattern)
		matches := re.FindAllString(md, -1)
		if len(matches) < check.minCount {
			t.Errorf("%s: expected at least %d, got %d", check.name, check.minCount, len(matches))
		}
	}

	// Check minimum content length
	if len(md) < 1000 {
		t.Errorf("output too short: %d chars (expected at least 1000)", len(md))
	}

	return nil
}

// TestE2EStage2_Similarity compares actual LLM output with expected using similarity
func TestE2EStage2_Similarity(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" &&
		os.Getenv("OPENAI_API_KEY") == "" &&
		os.Getenv("GOOGLE_API_KEY") == "" {
		t.Skip("skipping similarity test: no LLM API key available")
	}

	testdataDir := filepath.Join("..", "testdata")
	inputFile := filepath.Join(testdataDir, "한글 테스트.hwpx")
	expectedFormattedFile := filepath.Join(testdataDir, "expected_formatted.md")

	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		t.Skipf("input file not found: %s", inputFile)
	}
	if _, err := os.Stat(expectedFormattedFile); os.IsNotExist(err) {
		t.Skipf("expected formatted file not found: %s", expectedFormattedFile)
	}

	binPath, cleanup := buildTestBinary(t)
	defer cleanup()

	// Run convert with LLM
	cmd := exec.Command("./"+binPath, "convert", inputFile, "--llm")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("convert --llm command failed: %v\noutput: %s", err, output)
	}

	expected, err := os.ReadFile(expectedFormattedFile)
	if err != nil {
		t.Fatalf("failed to read expected file: %v", err)
	}

	// Calculate similarity
	similarity := calculateJaccardSimilarity(string(output), string(expected))
	minSimilarity := 0.2 // 20% similarity threshold (LLM outputs can vary significantly)

	t.Logf("Jaccard similarity: %.2f%%", similarity*100)

	if similarity < minSimilarity {
		t.Errorf("similarity too low: %.2f%% (minimum: %.2f%%)", similarity*100, minSimilarity*100)
	}
}

// calculateJaccardSimilarity calculates word-based Jaccard similarity
func calculateJaccardSimilarity(a, b string) float64 {
	wordsA := extractWords(a)
	wordsB := extractWords(b)

	setA := make(map[string]bool)
	setB := make(map[string]bool)

	for _, w := range wordsA {
		setA[w] = true
	}
	for _, w := range wordsB {
		setB[w] = true
	}

	intersection := 0
	for w := range setA {
		if setB[w] {
			intersection++
		}
	}

	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0
	}

	return float64(intersection) / float64(union)
}

// extractWords extracts words from text (Korean and English)
func extractWords(text string) []string {
	// Simple word extraction - split by whitespace and punctuation
	re := regexp.MustCompile(`[\p{L}\p{N}]+`)
	return re.FindAllString(text, -1)
}
