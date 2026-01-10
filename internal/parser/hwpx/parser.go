// Package hwpx provides a parser for HWPX (Open HWPML) documents.
package hwpx

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/roboco-io/hwp2md/internal/ir"
	"github.com/roboco-io/hwp2md/internal/parser"
)

// Parser parses HWPX documents.
type Parser struct {
	path    string
	reader  *zip.ReadCloser
	options parser.Options

	// Parsed data
	manifest *Manifest
	sections []string
	binData  map[string]string // id -> path mapping
}

// New creates a new HWPX parser for the given file path.
func New(path string, opts parser.Options) (*Parser, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open HWPX file: %w", err)
	}

	p := &Parser{
		path:    path,
		reader:  r,
		options: opts,
		binData: make(map[string]string),
	}

	// Parse manifest
	if err := p.parseManifest(); err != nil {
		r.Close()
		return nil, err
	}

	return p, nil
}

// Parse implements the Parser interface.
func (p *Parser) Parse() (*ir.Document, error) {
	doc := ir.NewDocument()

	// Set metadata from manifest
	if p.manifest != nil {
		doc.Metadata = p.manifest.ToMetadata()
	}

	// Parse each section in order
	for _, sectionPath := range p.sections {
		if err := p.parseSection(doc, sectionPath); err != nil {
			return nil, fmt.Errorf("failed to parse section %s: %w", sectionPath, err)
		}
	}

	return doc, nil
}

// Close releases resources.
func (p *Parser) Close() error {
	if p.reader != nil {
		return p.reader.Close()
	}
	return nil
}

// parseManifest reads and parses the content.hpf manifest file.
func (p *Parser) parseManifest() error {
	// Try different manifest locations
	manifestPaths := []string{
		"Contents/content.hpf",
		"content.hpf",
	}

	var manifestFile *zip.File
	for _, path := range manifestPaths {
		for _, f := range p.reader.File {
			if strings.EqualFold(f.Name, path) {
				manifestFile = f
				break
			}
		}
		if manifestFile != nil {
			break
		}
	}

	if manifestFile == nil {
		// No manifest found, try to find sections directly
		return p.findSectionsWithoutManifest()
	}

	rc, err := manifestFile.Open()
	if err != nil {
		return fmt.Errorf("failed to open manifest: %w", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	manifest, err := ParseManifest(data)
	if err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	p.manifest = manifest

	// Extract section paths and bindata mappings
	for _, item := range manifest.Items {
		href := item.Href
		// Normalize path
		if !strings.HasPrefix(href, "/") && !strings.HasPrefix(href, "Contents/") {
			if strings.HasSuffix(item.MediaType, "xml") && strings.Contains(item.ID, "section") {
				href = "Contents/" + href
			}
		}

		if strings.Contains(strings.ToLower(item.ID), "section") {
			p.sections = append(p.sections, href)
		}
		if strings.HasPrefix(item.Href, "BinData/") {
			p.binData[item.ID] = item.Href
		}
	}

	// Sort sections by name
	sort.Strings(p.sections)

	return nil
}

// findSectionsWithoutManifest finds section files when manifest is missing.
func (p *Parser) findSectionsWithoutManifest() error {
	for _, f := range p.reader.File {
		name := f.Name
		if strings.Contains(name, "section") && strings.HasSuffix(name, ".xml") {
			p.sections = append(p.sections, name)
		}
		if strings.HasPrefix(name, "BinData/") {
			// Use filename without extension as ID
			base := filepath.Base(name)
			id := strings.TrimSuffix(base, filepath.Ext(base))
			p.binData[id] = name
		}
	}
	sort.Strings(p.sections)
	return nil
}

// parseSection parses a single section XML file.
func (p *Parser) parseSection(doc *ir.Document, sectionPath string) error {
	var sectionFile *zip.File
	for _, f := range p.reader.File {
		if strings.EqualFold(f.Name, sectionPath) || strings.HasSuffix(f.Name, sectionPath) {
			sectionFile = f
			break
		}
	}

	if sectionFile == nil {
		return fmt.Errorf("section file not found: %s", sectionPath)
	}

	rc, err := sectionFile.Open()
	if err != nil {
		return fmt.Errorf("failed to open section: %w", err)
	}
	defer rc.Close()

	decoder := xml.NewDecoder(rc)
	return p.parseSectionXML(doc, decoder)
}

// tableState holds the state for a table being parsed.
type tableState struct {
	rows       [][]cellContext
	currentRow []cellContext
	cell       *cellContext
}

// parseSectionXML parses the section XML content.
func (p *Parser) parseSectionXML(doc *ir.Document, decoder *xml.Decoder) error {
	var currentParagraph *ir.Paragraph

	// Stack-based table handling for nested tables
	var tableStack []*tableState
	var currentTable *tableState

	// Helper to get current cell
	getCurrentCell := func() *cellContext {
		if currentTable != nil {
			return currentTable.cell
		}
		return nil
	}

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("XML parse error: %w", err)
		}

		switch t := token.(type) {
		case xml.StartElement:
			localName := t.Name.Local

			switch localName {
			case "p":
				currentParagraph = ir.NewParagraph("")
				// styleIDRef attribute is available for future style processing

			case "t":
				// Text element - read content
				if currentParagraph != nil {
					text, _ := readElementText(decoder)
					cell := getCurrentCell()
					if cell != nil {
						cell.text.WriteString(text)
					} else {
						currentParagraph.Text += text
					}
				}

			case "tab":
				if currentParagraph != nil {
					cell := getCurrentCell()
					if cell != nil {
						cell.text.WriteString("\t")
					} else {
						currentParagraph.Text += "\t"
					}
				}

			case "br":
				if currentParagraph != nil {
					brType := "line"
					for _, attr := range t.Attr {
						if attr.Name.Local == "type" {
							brType = attr.Value
						}
					}
					if brType == "line" {
						cell := getCurrentCell()
						if cell != nil {
							cell.text.WriteString("\n")
						} else {
							currentParagraph.Text += "\n"
						}
					}
				}

			case "tbl":
				// Push current table state to stack (if any)
				if currentTable != nil {
					tableStack = append(tableStack, currentTable)
				}
				// Start new table
				currentTable = &tableState{}

			case "tr":
				if currentTable != nil {
					currentTable.currentRow = nil
				}

			case "tc":
				if currentTable != nil {
					cell := cellContext{colSpan: 1, rowSpan: 1}
					// Note: colSpan and rowSpan are parsed from child cellSpan element
					currentTable.cell = &cell
				}

			case "cellSpan":
				// Parse cell span information (colSpan and rowSpan)
				cell := getCurrentCell()
				if cell != nil {
					for _, attr := range t.Attr {
						switch attr.Name.Local {
						case "colSpan":
							if _, err := fmt.Sscanf(attr.Value, "%d", &cell.colSpan); err != nil {
								cell.colSpan = 1
							}
						case "rowSpan":
							if _, err := fmt.Sscanf(attr.Value, "%d", &cell.rowSpan); err != nil {
								cell.rowSpan = 1
							}
						}
					}
				}

			case "pic", "img":
				// Image element
				if p.options.ExtractImages {
					img := p.parseImage(t)
					if img != nil {
						doc.AddImage(img)
					}
				}
			}

		case xml.EndElement:
			localName := t.Name.Local

			switch localName {
			case "p":
				if currentParagraph != nil && !currentParagraph.IsEmpty() {
					cell := getCurrentCell()
					if cell != nil {
						// Inside table cell - accumulate text
						if cell.text.Len() > 0 {
							cell.text.WriteString("\n")
						}
						cell.text.WriteString(currentParagraph.Text)
					} else if currentTable == nil {
						// Outside table - add to document
						doc.AddParagraph(currentParagraph)
					}
				}
				currentParagraph = nil

			case "tc":
				if currentTable != nil && currentTable.cell != nil {
					currentTable.currentRow = append(currentTable.currentRow, *currentTable.cell)
					currentTable.cell = nil
				}

			case "tr":
				if currentTable != nil && len(currentTable.currentRow) > 0 {
					currentTable.rows = append(currentTable.rows, currentTable.currentRow)
					currentTable.currentRow = nil
				}

			case "tbl":
				if currentTable != nil && len(currentTable.rows) > 0 {
					// Check if this is a nested table
					if len(tableStack) > 0 {
						// Convert nested table to text and add to parent cell
						nestedText := p.convertTableToText(currentTable.rows)
						// Pop parent table from stack
						parentTable := tableStack[len(tableStack)-1]
						tableStack = tableStack[:len(tableStack)-1]
						// Add nested table text to parent cell
						if parentTable.cell != nil {
							if parentTable.cell.text.Len() > 0 {
								parentTable.cell.text.WriteString("\n")
							}
							parentTable.cell.text.WriteString(nestedText)
						}
						currentTable = parentTable
					} else {
						// Top-level table - add to document
						table := p.buildTable(currentTable.rows)
						doc.AddTable(table)
						currentTable = nil
					}
				} else {
					// Empty table or no rows - restore parent if any
					if len(tableStack) > 0 {
						currentTable = tableStack[len(tableStack)-1]
						tableStack = tableStack[:len(tableStack)-1]
					} else {
						currentTable = nil
					}
				}
			}
		}
	}

	return nil
}

// convertTableToText converts a table to text format for nested table handling.
// This is used when a table is found inside another table cell.
func (p *Parser) convertTableToText(rows [][]cellContext) string {
	var sb strings.Builder

	for _, row := range rows {
		for i, cell := range row {
			text := strings.TrimSpace(cell.text.String())
			if text == "" {
				continue
			}
			if i > 0 {
				sb.WriteString(" | ")
			}
			// Replace newlines with spaces for inline display
			text = strings.ReplaceAll(text, "\n", " ")
			sb.WriteString(text)
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// cellContext holds temporary cell data during parsing.
type cellContext struct {
	text    strings.Builder
	colSpan int
	rowSpan int
}

// buildTable constructs an IR table from parsed rows.
// It properly handles rowSpan and colSpan to create a normalized table grid.
func (p *Parser) buildTable(rows [][]cellContext) *ir.TableBlock {
	if len(rows) == 0 {
		return nil
	}

	numRows := len(rows)

	// Two-pass approach:
	// Pass 1: Calculate actual column count by simulating cell placement
	// Pass 2: Place cells into the table

	// Pass 1: Calculate maxCols by simulating placement with rowSpan tracking
	tempOccupied := make([][]bool, numRows)
	for i := range tempOccupied {
		tempOccupied[i] = make([]bool, 100) // Use large initial size
	}

	maxCols := 0
	for rowIdx, row := range rows {
		colIdx := 0
		for _, cell := range row {
			// Skip occupied columns
			for colIdx < 100 && tempOccupied[rowIdx][colIdx] {
				colIdx++
			}
			if colIdx >= 100 {
				break
			}

			// Mark cells occupied by this cell's rowSpan and colSpan
			for r := rowIdx; r < rowIdx+cell.rowSpan && r < numRows; r++ {
				for c := colIdx; c < colIdx+cell.colSpan && c < 100; c++ {
					tempOccupied[r][c] = true
				}
			}

			colIdx += cell.colSpan
			if colIdx > maxCols {
				maxCols = colIdx
			}
		}
	}

	if maxCols == 0 {
		return nil
	}

	// Create the properly sized occupied grid
	occupiedGrid := make([][]bool, numRows)
	for i := range occupiedGrid {
		occupiedGrid[i] = make([]bool, maxCols)
	}

	table := ir.NewTable(numRows, maxCols)

	// Pass 2: Place cells and mark occupied cells from rowSpan
	for rowIdx, row := range rows {
		colIdx := 0
		cellIdx := 0

		for colIdx < maxCols && cellIdx < len(row) {
			// Skip columns occupied by rowSpan from previous rows
			for colIdx < maxCols && occupiedGrid[rowIdx][colIdx] {
				colIdx++
			}

			if colIdx >= maxCols || cellIdx >= len(row) {
				break
			}

			cell := row[cellIdx]
			table.Cells[rowIdx][colIdx].Text = strings.TrimSpace(cell.text.String())
			table.Cells[rowIdx][colIdx].ColSpan = cell.colSpan
			table.Cells[rowIdx][colIdx].RowSpan = cell.rowSpan

			// Mark cells occupied by this cell's rowSpan and colSpan
			for r := rowIdx; r < rowIdx+cell.rowSpan && r < numRows; r++ {
				for c := colIdx; c < colIdx+cell.colSpan && c < maxCols; c++ {
					occupiedGrid[r][c] = true
				}
			}

			colIdx += cell.colSpan
			cellIdx++
		}
	}

	// Check if first row might be header
	if len(rows) > 1 {
		table.SetHeaderRow()
	}

	return table
}

// parseImage extracts image information from XML element.
func (p *Parser) parseImage(elem xml.StartElement) *ir.ImageBlock {
	img := ir.NewImage("")

	for _, attr := range elem.Attr {
		switch attr.Name.Local {
		case "binItemIDRef", "binItemId":
			img.ID = attr.Value
			if path, ok := p.binData[attr.Value]; ok {
				img.Path = path
			}
		case "alt", "descr":
			img.Alt = attr.Value
		case "width":
			_, _ = fmt.Sscanf(attr.Value, "%d", &img.Width)
		case "height":
			_, _ = fmt.Sscanf(attr.Value, "%d", &img.Height)
		}
	}

	// Extract image data if requested
	if p.options.ExtractImages && img.Path != "" {
		data, err := p.extractBinData(img.Path)
		if err == nil {
			img.Data = data
			img.Format = strings.TrimPrefix(filepath.Ext(img.Path), ".")
		}
	}

	if img.ID == "" {
		return nil
	}

	return img
}

// extractBinData reads binary data from the HWPX archive.
func (p *Parser) extractBinData(path string) ([]byte, error) {
	for _, f := range p.reader.File {
		if strings.EqualFold(f.Name, path) || strings.HasSuffix(f.Name, path) {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("binary data not found: %s", path)
}

// ExtractImages extracts all images to the specified directory.
func (p *Parser) ExtractImages(dir string) ([]ir.ImageBlock, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create image directory: %w", err)
	}

	var images []ir.ImageBlock

	for id, path := range p.binData {
		data, err := p.extractBinData(path)
		if err != nil {
			continue
		}

		filename := filepath.Base(path)
		outPath := filepath.Join(dir, filename)

		if err := os.WriteFile(outPath, data, 0644); err != nil {
			continue
		}

		img := ir.ImageBlock{
			ID:     id,
			Path:   outPath,
			Format: strings.TrimPrefix(filepath.Ext(path), "."),
		}
		images = append(images, img)
	}

	return images, nil
}

// readElementText reads text content until the current element ends.
// It handles nested elements like <hp:fwSpace/> (full-width space) and <hp:hwSpace/> (half-width space).
func readElementText(decoder *xml.Decoder) (string, error) {
	var text strings.Builder
	depth := 1 // Track element nesting depth

	for {
		token, err := decoder.Token()
		if err != nil {
			return text.String(), err
		}

		switch t := token.(type) {
		case xml.CharData:
			text.Write(t)
		case xml.StartElement:
			// Handle special whitespace elements
			switch t.Name.Local {
			case "fwSpace": // Full-width space
				text.WriteString(" ")
			case "hwSpace": // Half-width space
				text.WriteString(" ")
			}
			depth++
		case xml.EndElement:
			depth--
			if depth == 0 {
				return text.String(), nil
			}
		}
	}
}
