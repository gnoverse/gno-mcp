// internal/server/registry_test.go
package server

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_addAndCount(t *testing.T) {
	r := NewRegistry()
	r.Add(&Tool{Name: "test", Capability: CapBaseRead})
	assert.Equal(t, 1, r.Count())
}

func TestRegistry_filterByCapability(t *testing.T) {
	r := NewRegistry()
	r.Add(&Tool{Name: "a", Capability: CapBaseRead})
	r.Add(&Tool{Name: "b", Capability: CapIndexerRead})
	r.Add(&Tool{Name: "c", Capability: CapBaseRead})
	got := r.WithCapability(CapBaseRead)
	assert.Len(t, got, 2)
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
	require.NoError(t, err)
	assert.True(t, called, "handler not invoked")
	assert.Equal(t, "ok", res.Text)
}

func TestRegistry_callUnknownTool(t *testing.T) {
	r := NewRegistry()
	_, err := r.Call(context.Background(), "missing", nil)
	require.Error(t, err)
}

func TestRegistry_callToolWithoutHandler(t *testing.T) {
	r := NewRegistry()
	r.Add(&Tool{Name: "stub", Capability: CapBaseRead})
	_, err := r.Call(context.Background(), "stub", nil)
	require.Error(t, err)
}

func TestRegistry_allReturnsSortedTools(t *testing.T) {
	r := NewRegistry()
	r.Add(&Tool{Name: "charlie", Capability: CapBaseRead})
	r.Add(&Tool{Name: "alpha", Capability: CapBaseRead})
	r.Add(&Tool{Name: "bravo", Capability: CapIndexerRead})
	all := r.All()
	require.Len(t, all, 3)
	assert.Equal(t, "alpha", all[0].Name)
	assert.Equal(t, "bravo", all[1].Name)
	assert.Equal(t, "charlie", all[2].Name)
}

// TestRegistry_callRecoversHandlerPanic pins the defense-in-depth contract: a
// panicking handler (e.g. a nil-pointer deref from an unresolved profile)
// degrades to a single tool error, never crashing the whole server process.
func TestRegistry_callRecoversHandlerPanic(t *testing.T) {
	r := NewRegistry()
	r.Add(&Tool{
		Name:       "boom",
		Capability: CapBaseRead,
		Handler: func(_ context.Context, _ map[string]any) (Result, error) {
			panic("kaboom")
		},
	})
	_, err := r.Call(context.Background(), "boom", map[string]any{})
	require.Error(t, err, "a handler panic must surface as an error, not crash the process")
	assert.Contains(t, err.Error(), "panic")
	assert.Contains(t, err.Error(), "boom", "error should name the offending tool")
}
