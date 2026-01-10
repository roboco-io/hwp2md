// Package upstage provides a document parser using Upstage Document Parse API.
package upstage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/roboco-io/hwp2md/internal/ir"
)

const (
	// DefaultBaseURL is the default Upstage API endpoint.
	DefaultBaseURL = "https://api.upstage.ai/v1/document-ai/document-parse"
	// DefaultModel is the default document parse model.
	DefaultModel = "document-parse"
	// ProviderName is the parser identifier.
	ProviderName = "upstage"
)

// Config holds the configuration for the Upstage parser.
type Config struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration
}

// Parser implements the document parser using Upstage Document Parse API.
type Parser struct {
	apiKey  string
	baseURL string
	model   string
	timeout time.Duration
	client  *http.Client
}

// APIResponse represents the response from Upstage Document Parse API.
type APIResponse struct {
	API     string `json:"api"`
	Model   string `json:"model"`
	Content struct {
		HTML     string `json:"html"`
		Markdown string `json:"markdown"`
		Text     string `json:"text"`
	} `json:"content"`
	Elements []Element `json:"elements"`
	Usage    struct {
		Pages int `json:"pages"`
	} `json:"usage"`
}

// Coordinate represents a point in the document.
type Coordinate struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Element represents a parsed document element.
type Element struct {
	ID          int          `json:"id"`
	Category    string       `json:"category"`
	Page        int          `json:"page"`
	Coordinates []Coordinate `json:"coordinates"`
	Content     struct {
		HTML     string `json:"html"`
		Markdown string `json:"markdown"`
		Text     string `json:"text"`
	} `json:"content"`
}

// New creates a new Upstage document parser.
func New(cfg Config) (*Parser, error) {
	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("UPSTAGE_API_KEY")
	}
	if apiKey == "" {
		return nil, errors.New("Upstage API key not configured (set UPSTAGE_API_KEY or provide via config)")
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	model := cfg.Model
	if model == "" {
		model = DefaultModel
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 180 * time.Second // 3 minutes for large documents
	}

	return &Parser{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		timeout: timeout,
		client:  &http.Client{Timeout: timeout},
	}, nil
}

// Name returns the parser identifier.
func (p *Parser) Name() string {
	return ProviderName
}

// Parse parses the document at the given path using Upstage Document Parse API.
func (p *Parser) Parse(ctx context.Context, filePath string) (*ir.Document, error) {
	// Read the file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add model field
	if err := writer.WriteField("model", p.model); err != nil {
		return nil, fmt.Errorf("failed to write model field: %w", err)
	}

	// Request markdown output format
	if err := writer.WriteField("output_formats", "[\"html\", \"markdown\", \"text\"]"); err != nil {
		return nil, fmt.Errorf("failed to write output_formats field: %w", err)
	}

	// Add file field
	part, err := writer.CreateFormFile("document", filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("failed to copy file content: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var apiResp APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %w", err)
	}

	// Create IR document with raw markdown from API
	doc := ir.NewDocument()
	doc.Metadata.Title = filepath.Base(filePath)
	doc.RawMarkdown = apiResp.Content.Markdown

	return doc, nil
}

// convertToIR converts Upstage API response to IR document.
func (p *Parser) convertToIR(resp *APIResponse, filePath string) *ir.Document {
	doc := ir.NewDocument()
	doc.Metadata.Title = filepath.Base(filePath)

	// Process elements in order
	for _, elem := range resp.Elements {
		switch elem.Category {
		case "heading1":
			para := ir.NewParagraph(elem.Content.Text)
			para.SetHeading(1)
			doc.AddParagraph(para)

		case "heading2":
			para := ir.NewParagraph(elem.Content.Text)
			para.SetHeading(2)
			doc.AddParagraph(para)

		case "heading3":
			para := ir.NewParagraph(elem.Content.Text)
			para.SetHeading(3)
			doc.AddParagraph(para)

		case "paragraph", "text":
			if strings.TrimSpace(elem.Content.Text) != "" {
				para := ir.NewParagraph(elem.Content.Text)
				doc.AddParagraph(para)
			}

		case "table":
			// Parse HTML table to IR table
			table := p.parseHTMLTable(elem.Content.HTML)
			if table != nil {
				doc.AddTable(table)
			}

		case "list":
			// Parse list from markdown or text
			list := p.parseList(elem.Content.Markdown, elem.Content.Text)
			if list != nil {
				doc.AddList(list)
			}

		case "figure", "image":
			img := ir.NewImage(elem.Content.Text)
			img.Alt = elem.Content.Text
			doc.AddImage(img)

		case "chart":
			// Treat chart as paragraph with description
			para := ir.NewParagraph("[차트] " + elem.Content.Text)
			doc.AddParagraph(para)

		case "equation":
			para := ir.NewParagraph(elem.Content.Text)
			doc.AddParagraph(para)

		default:
			// Handle unknown categories as paragraphs
			if strings.TrimSpace(elem.Content.Text) != "" {
				para := ir.NewParagraph(elem.Content.Text)
				doc.AddParagraph(para)
			}
		}
	}

	return doc
}

// parseHTMLTable parses HTML table to IR table.
func (p *Parser) parseHTMLTable(html string) *ir.TableBlock {
	// Simple HTML table parser
	// For complex cases, the raw HTML can be stored
	if html == "" {
		return nil
	}

	// Create table from raw HTML text extraction
	// This is a simplified approach - extract text content
	table := ir.NewTableFromRawText(html, 0, 0)
	return table
}

// parseList parses list content to IR list.
func (p *Parser) parseList(markdown, text string) *ir.ListBlock {
	content := markdown
	if content == "" {
		content = text
	}
	if content == "" {
		return nil
	}

	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return nil
	}

	// Determine if ordered or unordered
	firstLine := strings.TrimSpace(lines[0])
	isOrdered := len(firstLine) > 0 && (firstLine[0] >= '0' && firstLine[0] <= '9')

	var list *ir.ListBlock
	if isOrdered {
		list = ir.NewOrderedList()
	} else {
		list = ir.NewUnorderedList()
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Remove list markers
		line = strings.TrimLeft(line, "0123456789.-*+) ")
		if line != "" {
			list.AddItem(line)
		}
	}

	if list.IsEmpty() {
		return nil
	}

	return list
}

// ParseResult holds the raw API response for advanced use cases.
type ParseResult struct {
	Document *ir.Document
	Raw      *APIResponse
}

// ParseWithRaw parses the document and returns both IR and raw API response.
func (p *Parser) ParseWithRaw(ctx context.Context, filePath string) (*ParseResult, error) {
	// Read the file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	if err := writer.WriteField("model", p.model); err != nil {
		return nil, fmt.Errorf("failed to write model field: %w", err)
	}

	// Request markdown output format
	if err := writer.WriteField("output_formats", "[\"html\", \"markdown\", \"text\"]"); err != nil {
		return nil, fmt.Errorf("failed to write output_formats field: %w", err)
	}

	part, err := writer.CreateFormFile("document", filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("failed to copy file content: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var apiResp APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %w", err)
	}

	doc := p.convertToIR(&apiResp, filePath)

	return &ParseResult{
		Document: doc,
		Raw:      &apiResp,
	}, nil
}

// GetMarkdown returns the markdown content directly from API response.
func (p *Parser) GetMarkdown(ctx context.Context, filePath string) (string, error) {
	result, err := p.ParseWithRaw(ctx, filePath)
	if err != nil {
		return "", err
	}
	return result.Raw.Content.Markdown, nil
}
