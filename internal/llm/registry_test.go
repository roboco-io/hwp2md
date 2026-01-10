package llm

import (
	"context"
	"testing"

	"github.com/roboco-io/hwp2md/internal/ir"
)

// mockProvider is a test implementation of Provider.
type mockProvider struct {
	name string
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) Format(ctx context.Context, doc *ir.Document, opts FormatOptions) (*FormatResult, error) {
	return &FormatResult{
		Markdown: "# Mock Output",
		Model:    "mock-model",
	}, nil
}

func (m *mockProvider) Validate() error {
	return nil
}

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()

	if r == nil {
		t.Fatal("expected non-nil registry")
	}
	if r.Count() != 0 {
		t.Errorf("expected 0 providers, got %d", r.Count())
	}
}

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()
	p := &mockProvider{name: "test"}

	err := r.Register(p)
	if err != nil {
		t.Fatalf("failed to register: %v", err)
	}

	if r.Count() != 1 {
		t.Errorf("expected 1 provider, got %d", r.Count())
	}
}

func TestRegistry_RegisterDuplicate(t *testing.T) {
	r := NewRegistry()
	p1 := &mockProvider{name: "test"}
	p2 := &mockProvider{name: "test"}

	if err := r.Register(p1); err != nil {
		t.Fatalf("failed to register first: %v", err)
	}

	err := r.Register(p2)
	if err == nil {
		t.Error("expected error for duplicate registration")
	}
}

func TestRegistry_RegisterNil(t *testing.T) {
	r := NewRegistry()

	err := r.Register(nil)
	if err == nil {
		t.Error("expected error for nil provider")
	}
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()
	p := &mockProvider{name: "test"}
	_ = r.Register(p)

	got, err := r.Get("test")
	if err != nil {
		t.Fatalf("failed to get: %v", err)
	}

	if got.Name() != "test" {
		t.Errorf("expected 'test', got %s", got.Name())
	}
}

func TestRegistry_GetNotFound(t *testing.T) {
	r := NewRegistry()

	_, err := r.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent provider")
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(&mockProvider{name: "alpha"})
	_ = r.Register(&mockProvider{name: "beta"})
	_ = r.Register(&mockProvider{name: "gamma"})

	names := r.List()

	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}

	// List should be sorted
	if names[0] != "alpha" || names[1] != "beta" || names[2] != "gamma" {
		t.Errorf("expected sorted list, got %v", names)
	}
}

func TestRegistry_Has(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(&mockProvider{name: "test"})

	if !r.Has("test") {
		t.Error("expected Has('test') to return true")
	}
	if r.Has("nonexistent") {
		t.Error("expected Has('nonexistent') to return false")
	}
}

func TestRegistry_Unregister(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(&mockProvider{name: "test"})

	err := r.Unregister("test")
	if err != nil {
		t.Fatalf("failed to unregister: %v", err)
	}

	if r.Count() != 0 {
		t.Errorf("expected 0 providers after unregister, got %d", r.Count())
	}
}

func TestRegistry_UnregisterNotFound(t *testing.T) {
	r := NewRegistry()

	err := r.Unregister("nonexistent")
	if err == nil {
		t.Error("expected error for unregistering nonexistent provider")
	}
}

func TestDefaultFormatOptions(t *testing.T) {
	opts := DefaultFormatOptions()

	if opts.Language != "ko" {
		t.Errorf("expected language 'ko', got %s", opts.Language)
	}
	if opts.MaxTokens != 4096 {
		t.Errorf("expected max_tokens 4096, got %d", opts.MaxTokens)
	}
	if opts.Temperature != 0.3 {
		t.Errorf("expected temperature 0.3, got %f", opts.Temperature)
	}
}
