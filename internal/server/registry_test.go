// internal/server/registry_test.go
package server

import (
	"context"
	"testing"
)

func TestRegistry_addAndCount(t *testing.T) {
	r := NewRegistry()
	r.Add(&Tool{Name: "test", Capability: CapBaseRead})
	if got := r.Count(); got != 1 {
		t.Errorf("Count = %d, want 1", got)
	}
}

func TestRegistry_filterByCapability(t *testing.T) {
	r := NewRegistry()
	r.Add(&Tool{Name: "a", Capability: CapBaseRead})
	r.Add(&Tool{Name: "b", Capability: CapIndexerRead})
	r.Add(&Tool{Name: "c", Capability: CapBaseRead})
	got := r.WithCapability(CapBaseRead)
	if len(got) != 2 {
		t.Errorf("WithCapability(CapBaseRead) = %d, want 2", len(got))
	}
}

func TestRegistry_dispatch(t *testing.T) {
	called := false
	r := NewRegistry()
	r.Add(&Tool{
		Name:       "test_tool",
		Capability: CapBaseRead,
		Handler: func(ctx context.Context, args map[string]any) (Result, error) {
			called = true
			return Result{Text: "ok"}, nil
		},
	})
	res, err := r.Call(context.Background(), "test_tool", nil)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if !called {
		t.Error("handler not invoked")
	}
	if res.Text != "ok" {
		t.Errorf("res.Text = %q", res.Text)
	}
}

func TestRegistry_callUnknownTool(t *testing.T) {
	r := NewRegistry()
	_, err := r.Call(context.Background(), "missing", nil)
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestRegistry_callToolWithoutHandler(t *testing.T) {
	r := NewRegistry()
	r.Add(&Tool{Name: "stub", Capability: CapBaseRead})
	_, err := r.Call(context.Background(), "stub", nil)
	if err == nil {
		t.Fatal("expected error when tool has no handler")
	}
}

func TestRegistry_allReturnsSortedTools(t *testing.T) {
	r := NewRegistry()
	r.Add(&Tool{Name: "charlie", Capability: CapBaseRead})
	r.Add(&Tool{Name: "alpha", Capability: CapBaseRead})
	r.Add(&Tool{Name: "bravo", Capability: CapIndexerRead})
	all := r.All()
	if len(all) != 3 {
		t.Fatalf("All() = %d tools, want 3", len(all))
	}
	if all[0].Name != "alpha" || all[1].Name != "bravo" || all[2].Name != "charlie" {
		t.Errorf("All() not sorted by Name: %v", []string{all[0].Name, all[1].Name, all[2].Name})
	}
}
