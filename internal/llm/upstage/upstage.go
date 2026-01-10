// Package upstage provides an Upstage Solar-based LLM provider.
package upstage

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
	// DefaultModel is the default Upstage model.
	DefaultModel = "solar-pro"
	// DefaultBaseURL is the default Upstage API endpoint.
	DefaultBaseURL = "https://api.upstage.ai/v1/solar"
	// ProviderName is the provider identifier.
	ProviderName = "upstage"
)

// Provider implements the LLM Provider interface for Upstage.
type Provider struct {
	client  *goopenai.Client
	model   string
	apiKey  string
	timeout time.Duration
}

// Config holds the configuration for the Upstage provider.
type Config struct {
	APIKey  string
	Model   string
	BaseURL string
	Timeout time.Duration
}

// New creates a new Upstage provider.
func New(cfg Config) (*Provider, error) {
	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("UPSTAGE_API_KEY")
	}
	if apiKey == "" {
		return nil, errors.New("Upstage API key not configured (set UPSTAGE_API_KEY or provide via config)")
	}

	model := cfg.Model
	if model == "" {
		model = DefaultModel
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 120 * time.Second
	}

	// Upstage uses OpenAI-compatible API
	config := goopenai.DefaultConfig(apiKey)
	config.BaseURL = baseURL

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
		return errors.New("Upstage client not initialized")
	}
	if p.apiKey == "" {
		return errors.New("Upstage API key not set")
	}
	return nil
}

// Format converts an IR document to formatted Markdown using Upstage Solar.
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
		return nil, fmt.Errorf("Upstage API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, errors.New("Upstage returned no choices")
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
