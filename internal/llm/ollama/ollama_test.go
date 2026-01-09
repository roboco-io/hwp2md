package ollama

import (
	"os"
	"testing"
)

func TestNew_DefaultValues(t *testing.T) {
	// Ollama doesn't require API key
	p, err := New(Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.Name() != ProviderName {
		t.Errorf("expected provider name %q, got %q", ProviderName, p.Name())
	}

	if p.baseURL != DefaultBaseURL {
		t.Errorf("expected base URL %q, got %q", DefaultBaseURL, p.baseURL)
	}

	if p.model != DefaultModel {
		t.Errorf("expected model %q, got %q", DefaultModel, p.model)
	}
}

func TestNew_CustomBaseURL(t *testing.T) {
	p, err := New(Config{
		BaseURL: "http://custom:11434",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.baseURL != "http://custom:11434" {
		t.Errorf("expected base URL %q, got %q", "http://custom:11434", p.baseURL)
	}
}

func TestNew_CustomModel(t *testing.T) {
	p, err := New(Config{
		Model: "mistral",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.model != "mistral" {
		t.Errorf("expected model %q, got %q", "mistral", p.model)
	}
}

func TestNew_FromEnvVars(t *testing.T) {
	// Set env vars
	os.Setenv("OLLAMA_HOST", "http://envhost:11434")
	os.Setenv("OLLAMA_MODEL", "codellama")
	defer func() {
		os.Unsetenv("OLLAMA_HOST")
		os.Unsetenv("OLLAMA_MODEL")
	}()

	p, err := New(Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.baseURL != "http://envhost:11434" {
		t.Errorf("expected base URL from env, got %q", p.baseURL)
	}
	if p.model != "codellama" {
		t.Errorf("expected model from env, got %q", p.model)
	}
}

func TestProvider_Validate(t *testing.T) {
	p, err := New(Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := p.Validate(); err != nil {
		t.Errorf("expected no validation error, got: %v", err)
	}
}

func TestProvider_Name(t *testing.T) {
	p, _ := New(Config{})

	if p.Name() != "ollama" {
		t.Errorf("expected name 'ollama', got %q", p.Name())
	}
}

func TestChatRequest_Serialization(t *testing.T) {
	req := ChatRequest{
		Model: "llama3.2",
		Messages: []ChatMessage{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hello"},
		},
		Stream: false,
		Options: ChatOptions{
			Temperature: 0.7,
			NumPredict:  1024,
		},
	}

	if req.Model != "llama3.2" {
		t.Errorf("expected model llama3.2, got %s", req.Model)
	}
	if len(req.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(req.Messages))
	}
}
