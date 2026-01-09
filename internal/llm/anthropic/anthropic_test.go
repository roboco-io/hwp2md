package anthropic

import (
	"os"
	"testing"
)

func TestNew_NoAPIKey(t *testing.T) {
	// Temporarily unset API key
	oldKey := os.Getenv("ANTHROPIC_API_KEY")
	os.Unsetenv("ANTHROPIC_API_KEY")
	defer func() {
		if oldKey != "" {
			os.Setenv("ANTHROPIC_API_KEY", oldKey)
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

	if string(p.model) != DefaultModel {
		t.Errorf("expected model %q, got %q", DefaultModel, p.model)
	}
}

func TestNew_CustomModel(t *testing.T) {
	p, err := New(Config{
		APIKey: "test-key",
		Model:  "claude-3-haiku-20240307",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(p.model) != "claude-3-haiku-20240307" {
		t.Errorf("expected model %q, got %q", "claude-3-haiku-20240307", p.model)
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

	if p.Name() != "anthropic" {
		t.Errorf("expected name 'anthropic', got %q", p.Name())
	}
}
