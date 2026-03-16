package tests

import (
	"archive/zip"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func createTempHWPXWithMultiParagraphCell(t *testing.T) string {
	t.Helper()

	tempDir := t.TempDir()
	hwpxPath := filepath.Join(tempDir, "table-cell-multi-paragraph.hwpx")

	file, err := os.Create(hwpxPath)
	if err != nil {
		t.Fatalf("failed to create temp hwpx: %v", err)
	}
	defer file.Close()

	zipWriter := zip.NewWriter(file)
	defer zipWriter.Close()

	manifest := `<?xml version="1.0" encoding="UTF-8"?>
<opf:package xmlns:opf="http://www.idpf.org/2007/opf/">
  <opf:manifest>
    <opf:item id="section0" href="Contents/section0.xml" media-type="application/xml"/>
  </opf:manifest>
  <opf:spine>
    <opf:itemref idref="section0"/>
  </opf:spine>
</opf:package>`

	section := `<?xml version="1.0" encoding="UTF-8"?>
<hs:sec xmlns:hs="http://www.hancom.co.kr/hwpml/2011/section"
        xmlns:hp="http://www.hancom.co.kr/hwpml/2011/paragraph">
  <hp:tbl>
    <hp:tr>
      <hp:tc>
        <hp:subList>
          <hp:p><hp:run><hp:t>문단1</hp:t></hp:run></hp:p>
          <hp:p><hp:run><hp:t>문단2</hp:t></hp:run></hp:p>
          <hp:p><hp:run><hp:t>문단3</hp:t></hp:run></hp:p>
        </hp:subList>
      </hp:tc>
    </hp:tr>
  </hp:tbl>
</hs:sec>`

	addZipEntry := func(name, content string) {
		entry, createErr := zipWriter.Create(name)
		if createErr != nil {
			t.Fatalf("failed to create zip entry %s: %v", name, createErr)
		}
		if _, writeErr := entry.Write([]byte(content)); writeErr != nil {
			t.Fatalf("failed to write zip entry %s: %v", name, writeErr)
		}
	}

	addZipEntry("content.hpf", manifest)
	addZipEntry("Contents/section0.xml", section)

	return hwpxPath
}

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

func TestE2EStage1_HWPXTableCellParagraphBreaks(t *testing.T) {
	inputFile := createTempHWPXWithMultiParagraphCell(t)
	binPath, cleanup := buildTestBinary(t)
	defer cleanup()

	cmd := exec.Command("./"+binPath, "convert", inputFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("convert command failed: %v\noutput: %s", err, output)
	}

	md := string(output)
	if !strings.Contains(md, "문단1<br>문단2<br>문단3") {
		t.Fatalf("expected table cell paragraph boundaries rendered with <br>, got: %s", md)
	}
	if strings.Contains(md, "문단1문단2문단3") {
		t.Fatalf("unexpected concatenated table cell text without boundaries: %s", md)
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

// E2E Test for Stage 1: HWP 5.x -> Basic Markdown
// Verifies that converting testdata/hangul5test.hwp produces valid markdown with expected content

func TestE2EStage1_HWP5ToMarkdown(t *testing.T) {
	// Find test files
	testdataDir := filepath.Join("..", "testdata")
	inputFile := filepath.Join(testdataDir, "hangul5test.hwp")

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
	if err := validateHWP5Stage1Output(t, actualMD); err != nil {
		t.Errorf("HWP5 Stage 1 validation failed: %v", err)
	}
}

// validateHWP5Stage1Output checks that Stage 1 (parser) output from HWP 5.x contains expected content
func validateHWP5Stage1Output(t *testing.T, md string) error {
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
			t.Errorf("HWP5 Stage 1 output missing required content: %s", content)
		}
	}

	// Note: HWP 5.x table parsing is still in development
	// Tables are extracted as text content rather than markdown tables
	// TODO: Enable table syntax check when table parsing is complete
	// if !strings.Contains(md, "|") {
	// 	t.Error("HWP5 Stage 1 output should contain markdown tables (| syntax)")
	// }

	// Check minimum content length (document should have substantial content)
	if len(md) < 5000 {
		t.Errorf("HWP5 Stage 1 output too short: %d chars (expected at least 5000)", len(md))
	}

	// HWP 5.x specific: Check that no garbage characters appear at the start
	// These would indicate control character parsing issues
	if len(md) > 100 && (strings.Contains(md[:100], "捤獥") || strings.Contains(md[:100], "汤捯")) {
		t.Error("HWP5 Stage 1 output contains unparsed control characters at the start")
	}

	return nil
}

// TestE2EStage1_HWP5_ControlCharacters tests that HWP 5.x control characters are properly handled
func TestE2EStage1_HWP5_ControlCharacters(t *testing.T) {
	testdataDir := filepath.Join("..", "testdata")
	inputFile := filepath.Join(testdataDir, "hangul5test.hwp")

	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		t.Skipf("input file not found: %s", inputFile)
	}

	binPath, cleanup := buildTestBinary(t)
	defer cleanup()

	cmd := exec.Command("./"+binPath, "convert", inputFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("convert command failed: %v\noutput: %s", err, output)
	}

	actualMD := string(output)

	// Check for common control character parsing issues
	// These Unicode characters appear when HWP control codes are misinterpreted as text
	badPatterns := []string{
		"捤獥", // "secd" misread as UTF-16LE
		"汤捯", // "cold" misread
		"慤桥", // "head" misread
		"潦瑯", // "foot" misread
		"扴",  // "tbl" misread
	}

	for _, pattern := range badPatterns {
		if strings.Contains(actualMD, pattern) {
			t.Errorf("HWP5 output contains misinterpreted control character: %s", pattern)
		}
	}

	// Verify the document starts with valid content (after front matter)
	lines := strings.Split(actualMD, "\n")
	contentStarted := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "title:") {
			continue
		}
		contentStarted = true
		// First non-empty, non-frontmatter line should be readable Korean or ASCII
		hasKorean := false
		for _, r := range line {
			if r >= 0xAC00 && r <= 0xD7A3 { // Korean syllables range
				hasKorean = true
				break
			}
		}
		if !hasKorean && len(line) > 0 {
			// If no Korean, should at least be ASCII
			for _, r := range line {
				if r > 0x7F && (r < 0xAC00 || r > 0xD7A3) {
					t.Errorf("First content line contains unexpected non-ASCII, non-Korean character: %q", line)
					break
				}
			}
		}
		break
	}

	if !contentStarted {
		t.Error("HWP5 output has no content after front matter")
	}
}
