// Package gemini provides a Google Gemini-based LLM provider.
package gemini

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/roboco-io/hwp2md/internal/ir"
	"github.com/roboco-io/hwp2md/internal/llm"
	"google.golang.org/genai"
)

const (
	// DefaultModel is the default Gemini model.
	DefaultModel = "gemini-1.5-flash"
	// ProviderName is the provider identifier.
	ProviderName = "gemini"
)

// Provider implements the LLM Provider interface for Google Gemini.
type Provider struct {
	client  *genai.Client
	model   string
	apiKey  string
	timeout time.Duration
}

// Config holds the configuration for the Gemini provider.
type Config struct {
	APIKey  string
	Model   string
	Timeout time.Duration
}

// New creates a new Gemini provider.
func New(cfg Config) (*Provider, error) {
	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
	}
	if apiKey == "" {
		return nil, errors.New("Gemini API key not configured (set GOOGLE_API_KEY or GEMINI_API_KEY)")
	}

	model := cfg.Model
	if model == "" {
		model = DefaultModel
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

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
		return errors.New("Gemini client not initialized")
	}
	if p.apiKey == "" {
		return errors.New("Gemini API key not set")
	}
	return nil
}

// Format converts an IR document to formatted Markdown using Google Gemini.
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

	// Combine system and user prompts
	fullPrompt := fmt.Sprintf("%s\n\n---\n\n%s", systemPrompt, userPrompt)

	// Set timeout
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	// Build request config
	maxTokens := int32(opts.MaxTokens)
	if maxTokens == 0 {
		maxTokens = 4096
	}

	temperature := float32(opts.Temperature)
	if temperature == 0 {
		temperature = 0.3
	}

	config := &genai.GenerateContentConfig{
		Temperature:     &temperature,
		MaxOutputTokens: maxTokens,
	}

	// Call API
	result, err := p.client.Models.GenerateContent(
		ctx,
		p.model,
		genai.Text(fullPrompt),
		config,
	)
	if err != nil {
		return nil, fmt.Errorf("Gemini API error: %w", err)
	}

	// Extract text from response
	markdown := result.Text()

	// Calculate token usage (Gemini provides usage metadata)
	usage := llm.TokenUsage{}
	if result.UsageMetadata != nil {
		usage.InputTokens = int(result.UsageMetadata.PromptTokenCount)
		usage.OutputTokens = int(result.UsageMetadata.CandidatesTokenCount)
		usage.TotalTokens = int(result.UsageMetadata.TotalTokenCount)
	}

	return &llm.FormatResult{
		Markdown: markdown,
		Model:    p.model,
		Usage:    usage,
	}, nil
}
