// Package openai provides an OpenAI-based LLM provider.
package openai

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/roboco-io/hwp2md/internal/ir"
	"github.com/roboco-io/hwp2md/internal/llm"
	goopenai "github.com/sashabaranov/go-openai"
)

const (
	// DefaultModel is the default OpenAI model.
	DefaultModel = "gpt-4o-mini"
	// ProviderName is the provider identifier.
	ProviderName = "openai"
)

// Provider implements the LLM Provider interface for OpenAI.
type Provider struct {
	client  *goopenai.Client
	model   string
	apiKey  string
	timeout time.Duration
}

// Config holds the configuration for the OpenAI provider.
type Config struct {
	APIKey  string
	Model   string
	BaseURL string
	Timeout time.Duration
}

// New creates a new OpenAI provider.
func New(cfg Config) (*Provider, error) {
	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		return nil, errors.New("OpenAI API key not configured (set OPENAI_API_KEY or provide via config)")
	}

	model := cfg.Model
	if model == "" {
		model = DefaultModel
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 180 * time.Second // 3 minutes for large documents
	}

	config := goopenai.DefaultConfig(apiKey)
	if cfg.BaseURL != "" {
		config.BaseURL = cfg.BaseURL
	}

	client := goopenai.NewClientWithConfig(config)

	return &Provider{
		client:  client,
		model:   model,
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
	if p.client == nil {
		return errors.New("OpenAI client not initialized")
	}
	if p.apiKey == "" {
		return errors.New("OpenAI API key not set")
	}
	return nil
}

// Format converts an IR document to formatted Markdown using OpenAI.
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
	maxTokens := opts.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	temperature := float32(opts.Temperature)
	if temperature == 0 {
		temperature = 0.3
	}

	req := goopenai.ChatCompletionRequest{
		Model: p.model,
		Messages: []goopenai.ChatCompletionMessage{
			{
				Role:    goopenai.ChatMessageRoleSystem,
				Content: systemPrompt,
			},
			{
				Role:    goopenai.ChatMessageRoleUser,
				Content: userPrompt,
			},
		},
		MaxTokens:   maxTokens,
		Temperature: temperature,
	}

	// Call API
	resp, err := p.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("OpenAI API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, errors.New("OpenAI returned no choices")
	}

	return &llm.FormatResult{
		Markdown: resp.Choices[0].Message.Content,
		Model:    resp.Model,
		Usage: llm.TokenUsage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		},
	}, nil
}
