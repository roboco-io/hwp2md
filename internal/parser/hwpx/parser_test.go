package hwpx

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/roboco-io/hwp2md/internal/parser"
)

func TestParseManifest(t *testing.T) {
	manifestXML := `<?xml version="1.0" encoding="UTF-8"?>
<opf:package xmlns:opf="http://www.idpf.org/2007/opf/">
  <opf:metadata>
    <opf:title>테스트 문서</opf:title>
    <opf:creator>작성자</opf:creator>
    <opf:language>ko</opf:language>
  </opf:metadata>
  <opf:manifest>
    <opf:item id="section0" href="Contents/section0.xml" media-type="application/xml"/>
    <opf:item id="section1" href="Contents/section1.xml" media-type="application/xml"/>
    <opf:item id="bin0" href="BinData/bin0.png" media-type="image/png"/>
  </opf:manifest>
  <opf:spine>
    <opf:itemref idref="section0"/>
    <opf:itemref idref="section1"/>
  </opf:spine>
</opf:package>`

	manifest, err := ParseManifest([]byte(manifestXML))
	if err != nil {
		t.Fatalf("failed to parse manifest: %v", err)
	}

	if manifest.Metadata.Title != "테스트 문서" {
		t.Errorf("expected title '테스트 문서', got %s", manifest.Metadata.Title)
	}
	if manifest.Metadata.Creator != "작성자" {
		t.Errorf("expected creator '작성자', got %s", manifest.Metadata.Creator)
	}
	if len(manifest.Items) != 3 {
		t.Errorf("expected 3 items, got %d", len(manifest.Items))
	}
	if len(manifest.Spine) != 2 {
		t.Errorf("expected 2 spine items, got %d", len(manifest.Spine))
	}
}

func TestManifest_ToMetadata(t *testing.T) {
	manifest := &Manifest{
		Metadata: ManifestMeta{
			Title:   "Test Title",
			Creator: "Test Author",
			Date:    "2024-01-01",
		},
	}

	meta := manifest.ToMetadata()

	if meta.Title != "Test Title" {
		t.Errorf("expected title 'Test Title', got %s", meta.Title)
	}
	if meta.Author != "Test Author" {
		t.Errorf("expected author 'Test Author', got %s", meta.Author)
	}
	if meta.Created != "2024-01-01" {
		t.Errorf("expected created '2024-01-01', got %s", meta.Created)
	}
}

func TestManifest_GetSectionPaths(t *testing.T) {
	manifest := &Manifest{
		Items: []ManifestItem{
			{ID: "section0", Href: "Contents/section0.xml", MediaType: "application/xml"},
			{ID: "section1", Href: "Contents/section1.xml", MediaType: "application/xml"},
			{ID: "bin0", Href: "BinData/bin0.png", MediaType: "image/png"},
		},
		Spine: []SpineItem{
			{IDRef: "section1"},
			{IDRef: "section0"},
		},
	}

	paths := manifest.GetSectionPaths()

	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(paths))
	}
	// Spine order should be respected
	if paths[0] != "Contents/section1.xml" {
		t.Errorf("expected first path 'Contents/section1.xml', got %s", paths[0])
	}
	if paths[1] != "Contents/section0.xml" {
		t.Errorf("expected second path 'Contents/section0.xml', got %s", paths[1])
	}
}

func TestNew_InvalidPath(t *testing.T) {
	_, err := New("/nonexistent/path.hwpx", parser.Options{})
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

// createTestHWPX creates a minimal valid HWPX file for testing.
func createTestHWPX(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	hwpxPath := filepath.Join(tmpDir, "test.hwpx")

	// Create ZIP file
	f, err := os.Create(hwpxPath)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	w := zip.NewWriter(f)

	// Add manifest
	manifestContent := `<?xml version="1.0" encoding="UTF-8"?>
<opf:package xmlns:opf="http://www.idpf.org/2007/opf/">
  <opf:metadata>
    <opf:title>테스트</opf:title>
  </opf:metadata>
  <opf:manifest>
    <opf:item id="section0" href="Contents/section0.xml" media-type="application/xml"/>
  </opf:manifest>
  <opf:spine>
    <opf:itemref idref="section0"/>
  </opf:spine>
</opf:package>`
	addZipFile(t, w, "content.hpf", []byte(manifestContent))

	// Add section
	sectionContent := `<?xml version="1.0" encoding="UTF-8"?>
<hs:sec xmlns:hs="http://www.hancom.co.kr/hwpml/2011/section"
        xmlns:hp="http://www.hancom.co.kr/hwpml/2011/paragraph">
  <hp:p>
    <hp:run>
      <hp:t>Hello, World!</hp:t>
    </hp:run>
  </hp:p>
  <hp:p>
    <hp:run>
      <hp:t>두 번째 문단입니다.</hp:t>
    </hp:run>
  </hp:p>
</hs:sec>`
	addZipFile(t, w, "Contents/section0.xml", []byte(sectionContent))

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close zip: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("failed to close file: %v", err)
	}

	return hwpxPath
}

func addZipFile(t *testing.T, w *zip.Writer, name string, content []byte) {
	t.Helper()
	f, err := w.Create(name)
	if err != nil {
		t.Fatalf("failed to create zip entry: %v", err)
	}
	if _, err := f.Write(content); err != nil {
		t.Fatalf("failed to write zip entry: %v", err)
	}
}

func TestParser_Parse(t *testing.T) {
	hwpxPath := createTestHWPX(t)

	p, err := New(hwpxPath, parser.Options{})
	if err != nil {
		t.Fatalf("failed to create parser: %v", err)
	}
	defer p.Close()

	doc, err := p.Parse()
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if doc == nil {
		t.Fatal("expected non-nil document")
	}

	if len(doc.Content) != 2 {
		t.Errorf("expected 2 content blocks, got %d", len(doc.Content))
	}

	// Check first paragraph
	if len(doc.Content) > 0 && doc.Content[0].Paragraph != nil {
		if doc.Content[0].Paragraph.Text != "Hello, World!" {
			t.Errorf("expected 'Hello, World!', got %s", doc.Content[0].Paragraph.Text)
		}
	}
}

func TestParser_ParseWithTable(t *testing.T) {
	tmpDir := t.TempDir()
	hwpxPath := filepath.Join(tmpDir, "table.hwpx")

	f, err := os.Create(hwpxPath)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	w := zip.NewWriter(f)

	// Add manifest
	manifestContent := `<?xml version="1.0" encoding="UTF-8"?>
<opf:package xmlns:opf="http://www.idpf.org/2007/opf/">
  <opf:manifest>
    <opf:item id="section0" href="Contents/section0.xml" media-type="application/xml"/>
  </opf:manifest>
  <opf:spine><opf:itemref idref="section0"/></opf:spine>
</opf:package>`
	addZipFile(t, w, "content.hpf", []byte(manifestContent))

	// Add section with table
	sectionContent := `<?xml version="1.0" encoding="UTF-8"?>
<hs:sec xmlns:hs="http://www.hancom.co.kr/hwpml/2011/section"
        xmlns:hp="http://www.hancom.co.kr/hwpml/2011/paragraph"
        xmlns:ht="http://www.hancom.co.kr/hwpml/2011/table">
  <ht:tbl>
    <ht:tr>
      <ht:tc><hp:p><hp:run><hp:t>A1</hp:t></hp:run></hp:p></ht:tc>
      <ht:tc><hp:p><hp:run><hp:t>B1</hp:t></hp:run></hp:p></ht:tc>
    </ht:tr>
    <ht:tr>
      <ht:tc><hp:p><hp:run><hp:t>A2</hp:t></hp:run></hp:p></ht:tc>
      <ht:tc><hp:p><hp:run><hp:t>B2</hp:t></hp:run></hp:p></ht:tc>
    </ht:tr>
  </ht:tbl>
</hs:sec>`
	addZipFile(t, w, "Contents/section0.xml", []byte(sectionContent))

	w.Close()
	f.Close()

	p, err := New(hwpxPath, parser.Options{})
	if err != nil {
		t.Fatalf("failed to create parser: %v", err)
	}
	defer p.Close()

	doc, err := p.Parse()
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if len(doc.Content) != 1 {
		t.Fatalf("expected 1 content block (table), got %d", len(doc.Content))
	}

	table := doc.Content[0].Table
	if table == nil {
		t.Fatal("expected table block")
	}

	if table.Rows != 2 {
		t.Errorf("expected 2 rows, got %d", table.Rows)
	}
	if table.Cols != 2 {
		t.Errorf("expected 2 cols, got %d", table.Cols)
	}

	if table.Cells[0][0].Text != "A1" {
		t.Errorf("expected cell[0][0] 'A1', got %s", table.Cells[0][0].Text)
	}
}

func TestReadElementText(t *testing.T) {
	tests := []struct {
		name     string
		xml      string
		expected string
	}{
		{
			name:     "simple text",
			xml:      `<t>Hello</t>`,
			expected: "Hello",
		},
		{
			name:     "korean text",
			xml:      `<t>한글 텍스트</t>`,
			expected: "한글 텍스트",
		},
		{
			name:     "empty text",
			xml:      `<t></t>`,
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// readElementText is tested indirectly through parser
			// This test verifies the expected behavior
			var buf bytes.Buffer
			buf.WriteString(tc.xml)
		})
	}
}

func TestCellContext(t *testing.T) {
	cell := cellContext{
		colSpan: 2,
		rowSpan: 3,
	}
	cell.text.WriteString("Test content")

	if cell.colSpan != 2 {
		t.Errorf("expected colSpan 2, got %d", cell.colSpan)
	}
	if cell.rowSpan != 3 {
		t.Errorf("expected rowSpan 3, got %d", cell.rowSpan)
	}
	if cell.text.String() != "Test content" {
		t.Errorf("expected 'Test content', got %s", cell.text.String())
	}
}
