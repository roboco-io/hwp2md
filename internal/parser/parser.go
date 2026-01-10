// Package parser provides interfaces and implementations for parsing HWP documents.
package parser

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/roboco-io/hwp2md/internal/ir"
)

// Parser is the interface for document parsers.
type Parser interface {
	// Parse reads the document and returns an IR representation.
	Parse() (*ir.Document, error)

	// Close releases any resources held by the parser.
	Close() error
}

// Format represents a document format.
type Format int

const (
	FormatUnknown Format = iota
	FormatHWPX
	FormatHWP // HWP 5.x binary format
)

// String returns the string representation of the format.
func (f Format) String() string {
	switch f {
	case FormatHWPX:
		return "hwpx"
	case FormatHWP:
		return "hwp"
	default:
		return "unknown"
	}
}

// DetectFormat detects the document format from the file path.
func DetectFormat(path string) Format {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".hwpx":
		return FormatHWPX
	case ".hwp", ".hwp5":
		return FormatHWP
	default:
		return FormatUnknown
	}
}

// DetectFormatFromReader detects the format by reading magic bytes.
func DetectFormatFromReader(r io.ReaderAt) (Format, error) {
	// Read first 8 bytes for magic number detection
	buf := make([]byte, 8)
	n, err := r.ReadAt(buf, 0)
	if err != nil && err != io.EOF {
		return FormatUnknown, fmt.Errorf("failed to read magic bytes: %w", err)
	}
	if n < 4 {
		return FormatUnknown, fmt.Errorf("file too small to detect format")
	}

	// ZIP magic number (HWPX)
	if buf[0] == 'P' && buf[1] == 'K' {
		return FormatHWPX, nil
	}

	// OLE/CFBF magic number (HWP 5.x)
	if buf[0] == 0xD0 && buf[1] == 0xCF && buf[2] == 0x11 && buf[3] == 0xE0 {
		return FormatHWP, nil
	}

	// HWP Document File signature
	if len(buf) >= 8 && string(buf[:3]) == "HWP" {
		return FormatHWP, nil
	}

	return FormatUnknown, nil
}

// Options contains parser configuration options.
type Options struct {
	ExtractImages bool   // Whether to extract embedded images
	ImageDir      string // Directory to save extracted images
}

// DefaultOptions returns default parser options.
func DefaultOptions() Options {
	return Options{
		ExtractImages: false,
		ImageDir:      "",
	}
}
