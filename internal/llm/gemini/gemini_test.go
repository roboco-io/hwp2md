package gemini

import (
	"os"
	"testing"
)

func TestNew_NoAPIKey(t *testing.T) {
	// Temporarily unset API keys
	oldGoogleKey := os.Getenv("GOOGLE_API_KEY")
	oldGeminiKey := os.Getenv("GEMINI_API_KEY")
	os.Unsetenv("GOOGLE_API_KEY")
	os.Unsetenv("GEMINI_API_KEY")
	defer func() {
		if oldGoogleKey != "" {
			os.Setenv("GOOGLE_API_KEY", oldGoogleKey)
		}
		if oldGeminiKey != "" {
			os.Setenv("GEMINI_API_KEY", oldGeminiKey)
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
		Model:  "gemini-1.5-pro",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.model != "gemini-1.5-pro" {
		t.Errorf("expected model %q, got %q", "gemini-1.5-pro", p.model)
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

	if p.Name() != "gemini" {
		t.Errorf("expected name 'gemini', got %q", p.Name())
	}
}
