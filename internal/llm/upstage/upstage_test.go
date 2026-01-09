package upstage

import (
	"os"
	"testing"
)

func TestNew_NoAPIKey(t *testing.T) {
	// Temporarily unset API key
	oldKey := os.Getenv("UPSTAGE_API_KEY")
	os.Unsetenv("UPSTAGE_API_KEY")
	defer func() {
		if oldKey != "" {
			os.Setenv("UPSTAGE_API_KEY", oldKey)
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
		Model:  "solar-mini",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.model != "solar-mini" {
		t.Errorf("expected model %q, got %q", "solar-mini", p.model)
	}
}

func TestNew_CustomBaseURL(t *testing.T) {
	p, err := New(Config{
		APIKey:  "test-key",
		BaseURL: "https://custom.api.upstage.ai/v1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.client == nil {
		t.Error("expected client to be initialized")
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

	if p.Name() != "upstage" {
		t.Errorf("expected name 'upstage', got %q", p.Name())
	}
}
