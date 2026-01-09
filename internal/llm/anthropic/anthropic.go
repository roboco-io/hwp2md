// Package anthropic provides an Anthropic Claude-based LLM provider.
package anthropic

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/roboco-io/hwp2markdown/internal/ir"
	"github.com/roboco-io/hwp2markdown/internal/llm"
)

const (
	// DefaultModel is the default Anthropic model.
	DefaultModel = "claude-3-5-sonnet-20241022"
	// ProviderName is the provider identifier.
	ProviderName = "anthropic"
)

// Provider implements the LLM Provider interface for Anthropic.
type Provider struct {
	client  anthropic.Client
	model   anthropic.Model
	apiKey  string
	timeout time.Duration
}

// Config holds the configuration for the Anthropic provider.
type Config struct {
	APIKey  string
	Model   string
	BaseURL string
	Timeout time.Duration
}

// New creates a new Anthropic provider.
func New(cfg Config) (*Provider, error) {
	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		return nil, errors.New("Anthropic API key not configured (set ANTHROPIC_API_KEY or provide via config)")
	}

	model := cfg.Model
	if model == "" {
		model = DefaultModel
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}

	client := anthropic.NewClient(opts...)

	return &Provider{
		client:  client,
		model:   anthropic.Model(model),
		apiKey:  apiKey,
		timeout: timeout,
	}, nil
}

// Name returns the provider identifier.
func (p *Provider) Name() string {
	return ProviderName
}

// Validate checks if the provider is properly configured.
func (p *Provider) Validate() error {
	if p.apiKey == "" {
		return errors.New("Anthropic API key not set")
	}
	return nil
}

// Format converts an IR document to formatted Markdown using Anthropic Claude.
func (p *Provider) Format(ctx context.Context, doc *ir.Document, opts llm.FormatOptions) (*llm.FormatResult, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}

	// Build prompts
	systemPrompt := opts.Prompt
	if systemPrompt == "" {
		systemPrompt = llm.SystemPrompt
	}

	userPrompt := llm.BuildCompactPrompt(doc)

	// Set timeout
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	// Build request
	maxTokens := int64(opts.MaxTokens)
	if maxTokens == 0 {
		maxTokens = 4096
	}

	temperature := opts.Temperature
	if temperature == 0 {
		temperature = 0.3
	}

	// Call API using helper functions
	resp, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     p.model,
		MaxTokens: maxTokens,
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)),
		},
		Temperature: anthropic.Float(temperature),
	})
	if err != nil {
		return nil, fmt.Errorf("Anthropic API error: %w", err)
	}

	// Extract text from response
	var markdown string
	for _, block := range resp.Content {
		if block.Type == "text" {
			textBlock := block.AsText()
			markdown += textBlock.Text
		}
	}

	return &llm.FormatResult{
		Markdown: markdown,
		Model:    string(resp.Model),
		Usage: llm.TokenUsage{
			InputTokens:  int(resp.Usage.InputTokens),
			OutputTokens: int(resp.Usage.OutputTokens),
			TotalTokens:  int(resp.Usage.InputTokens + resp.Usage.OutputTokens),
		},
	}, nil
}
