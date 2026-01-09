// Package ollama provides a local Ollama-based LLM provider.
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/roboco-io/hwp2markdown/internal/ir"
	"github.com/roboco-io/hwp2markdown/internal/llm"
)

const (
	// DefaultModel is the default Ollama model.
	DefaultModel = "llama3.2"
	// DefaultBaseURL is the default Ollama server URL.
	DefaultBaseURL = "http://localhost:11434"
	// ProviderName is the provider identifier.
	ProviderName = "ollama"
)

// Provider implements the LLM Provider interface for Ollama.
type Provider struct {
	client  *http.Client
	baseURL string
	model   string
	timeout time.Duration
}

// Config holds the configuration for the Ollama provider.
type Config struct {
	BaseURL string
	Model   string
	Timeout time.Duration
}

// New creates a new Ollama provider.
func New(cfg Config) (*Provider, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = os.Getenv("OLLAMA_HOST")
	}
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	model := cfg.Model
	if model == "" {
		model = os.Getenv("OLLAMA_MODEL")
	}
	if model == "" {
		model = DefaultModel
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 120 * time.Second // Longer timeout for local models
	}

	client := &http.Client{
		Timeout: timeout,
	}

	return &Provider{
		client:  client,
		baseURL: baseURL,
		model:   model,
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
		return errors.New("Ollama HTTP client not initialized")
	}
	if p.baseURL == "" {
		return errors.New("Ollama base URL not set")
	}
	return nil
}

// ChatRequest represents the Ollama chat API request.
type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
	Options  ChatOptions   `json:"options,omitempty"`
}

// ChatMessage represents a message in the chat.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatOptions represents generation options.
type ChatOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

// ChatResponse represents the Ollama chat API response.
type ChatResponse struct {
	Model     string      `json:"model"`
	Message   ChatMessage `json:"message"`
	Done      bool        `json:"done"`
	TotalDuration    int64 `json:"total_duration,omitempty"`
	LoadDuration     int64 `json:"load_duration,omitempty"`
	PromptEvalCount  int   `json:"prompt_eval_count,omitempty"`
	EvalCount        int   `json:"eval_count,omitempty"`
}

// Format converts an IR document to formatted Markdown using Ollama.
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

	// Build request
	maxTokens := opts.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	temperature := opts.Temperature
	if temperature == 0 {
		temperature = 0.3
	}

	req := ChatRequest{
		Model: p.model,
		Messages: []ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Stream: false,
		Options: ChatOptions{
			Temperature: temperature,
			NumPredict:  maxTokens,
		},
	}

	// Send request
	resp, err := p.sendRequest(ctx, "/api/chat", req)
	if err != nil {
		return nil, err
	}

	return &llm.FormatResult{
		Markdown: resp.Message.Content,
		Model:    resp.Model,
		Usage: llm.TokenUsage{
			InputTokens:  resp.PromptEvalCount,
			OutputTokens: resp.EvalCount,
			TotalTokens:  resp.PromptEvalCount + resp.EvalCount,
		},
	}, nil
}

// sendRequest sends an HTTP request to the Ollama API.
func (p *Provider) sendRequest(ctx context.Context, endpoint string, payload interface{}) (*ChatResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := p.baseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Ollama API error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &chatResp, nil
}
