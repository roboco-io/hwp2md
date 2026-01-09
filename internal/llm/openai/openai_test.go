package openai

import (
	"os"
	"testing"
)

func TestNew_NoAPIKey(t *testing.T) {
	// Temporarily unset API key
	oldKey := os.Getenv("OPENAI_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")
	defer func() {
		if oldKey != "" {
			os.Setenv("OPENAI_API_KEY", oldKey)
		}
	}()

	_, err := New(Config{})
	if err == nil {
		t.Error("expected error when API key is not set")
	}
}

func TestNew_WithAPIKey(t *testing.T) {
	p, err := New(Config{
		APIKey: "test-key",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.Name() != ProviderName {
		t.Errorf("expected provider name %q, got %q", ProviderName, p.Name())
	}
}

func TestNew_DefaultModel(t *testing.T) {
	p, err := New(Config{
		APIKey: "test-key",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.model != DefaultModel {
		t.Errorf("expected model %q, got %q", DefaultModel, p.model)
	}
}

func TestNew_CustomModel(t *testing.T) {
	p, err := New(Config{
		APIKey: "test-key",
		Model:  "gpt-4o",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.model != "gpt-4o" {
		t.Errorf("expected model %q, got %q", "gpt-4o", p.model)
	}
}

func TestProvider_Validate(t *testing.T) {
	p, err := New(Config{
		APIKey: "test-key",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := p.Validate(); err != nil {
		t.Errorf("expected no validation error, got: %v", err)
	}
}

func TestProvider_Name(t *testing.T) {
	p, _ := New(Config{APIKey: "test-key"})

	if p.Name() != "openai" {
		t.Errorf("expected name 'openai', got %q", p.Name())
	}
}
