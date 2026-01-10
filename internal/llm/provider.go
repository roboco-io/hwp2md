// Package llm provides the LLM provider interface and registry for Stage 2 formatting.
package llm

import (
	"context"

	"github.com/roboco-io/hwp2md/internal/ir"
)

// Provider is the interface that all LLM providers must implement.
type Provider interface {
	// Name returns the provider identifier (e.g., "openai", "anthropic").
	Name() string

	// Format takes an IR document and returns formatted markdown.
	Format(ctx context.Context, doc *ir.Document, opts FormatOptions) (*FormatResult, error)

	// Validate checks if the provider is properly configured.
	Validate() error
}

// FormatOptions contains options for LLM formatting.
type FormatOptions struct {
	Language    string  `json:"language,omitempty"`    // output language (e.g., "ko", "en")
	MaxTokens   int     `json:"max_tokens,omitempty"`  // maximum tokens for response
	Temperature float64 `json:"temperature,omitempty"` // creativity level (0.0 - 1.0)
	Prompt      string  `json:"prompt,omitempty"`      // custom system prompt
}

// FormatResult contains the result of LLM formatting.
type FormatResult struct {
	Markdown string     `json:"markdown"`
	Usage    TokenUsage `json:"usage"`
	Model    string     `json:"model"`
}

// TokenUsage contains token usage statistics.
type TokenUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// DefaultFormatOptions returns the default formatting options.
func DefaultFormatOptions() FormatOptions {
	return FormatOptions{
		Language:    "ko",
		MaxTokens:   4096,
		Temperature: 0.3,
	}
}
