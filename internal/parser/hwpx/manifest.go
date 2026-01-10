// Package hwpx provides a parser for HWPX (Open HWPML) documents.
package hwpx

import (
	"encoding/xml"

	"github.com/roboco-io/hwp2md/internal/ir"
)

// Manifest represents the OPF package manifest (content.hpf).
type Manifest struct {
	XMLName  xml.Name       `xml:"package"`
	Metadata ManifestMeta   `xml:"metadata"`
	Items    []ManifestItem `xml:"manifest>item"`
	Spine    []SpineItem    `xml:"spine>itemref"`
}

// ManifestMeta contains document metadata from the manifest.
type ManifestMeta struct {
	Title       string `xml:"title"`
	Creator     string `xml:"creator"`
	Subject     string `xml:"subject"`
	Description string `xml:"description"`
	Publisher   string `xml:"publisher"`
	Date        string `xml:"date"`
	Language    string `xml:"language"`
	Keywords    string `xml:"keywords"`
}

// ManifestItem represents a single item in the manifest.
type ManifestItem struct {
	ID        string `xml:"id,attr"`
	Href      string `xml:"href,attr"`
	MediaType string `xml:"media-type,attr"`
}

// SpineItem represents a spine reference for reading order.
type SpineItem struct {
	IDRef string `xml:"idref,attr"`
}

// ParseManifest parses OPF-format manifest XML data.
func ParseManifest(data []byte) (*Manifest, error) {
	var manifest Manifest
	if err := xml.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

// ToMetadata converts manifest metadata to IR metadata.
func (m *Manifest) ToMetadata() ir.Metadata {
	return ir.Metadata{
		Title:       m.Metadata.Title,
		Author:      m.Metadata.Creator,
		Subject:     m.Metadata.Subject,
		Description: m.Metadata.Description,
		Keywords:    m.Metadata.Keywords,
		Creator:     m.Metadata.Creator,
		Created:     m.Metadata.Date,
	}
}

// GetSectionPaths returns ordered section file paths based on spine.
func (m *Manifest) GetSectionPaths() []string {
	// Build id -> href map
	itemMap := make(map[string]string)
	for _, item := range m.Items {
		itemMap[item.ID] = item.Href
	}

	// Get sections in spine order
	var paths []string
	for _, ref := range m.Spine {
		if href, ok := itemMap[ref.IDRef]; ok {
			paths = append(paths, href)
		}
	}

	// If spine is empty, fall back to manifest order
	if len(paths) == 0 {
		for _, item := range m.Items {
			if isSection(item) {
				paths = append(paths, item.Href)
			}
		}
	}

	return paths
}

// isSection checks if a manifest item is a section file.
func isSection(item ManifestItem) bool {
	return item.MediaType == "application/xml" ||
		item.MediaType == "text/xml" ||
		(len(item.ID) > 0 && (item.ID[0] == 's' || item.ID[0] == 'S'))
}
