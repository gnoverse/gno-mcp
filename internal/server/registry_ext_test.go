package server_test

import (
	"strings"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnnotations_Validate_okWhenIndependent(t *testing.T) {
	cases := []server.Annotations{
		{},
		{ReadOnly: true, Idempotent: true},
		{Destructive: true, OpenWorld: true},
		{Idempotent: true, OpenWorld: true},
	}
	for i, a := range cases {
		assert.NoError(t, a.Validate(), "case %d", i)
	}
}

func TestAnnotations_Validate_rejectsReadOnlyAndDestructive(t *testing.T) {
	a := server.Annotations{ReadOnly: true, Destructive: true}
	require.Error(t, a.Validate())
}

func TestToolError_Error(t *testing.T) {
	e := &server.ToolError{Code: "x", Message: "bad"}
	assert.Equal(t, "tool error [x]: bad", e.Error())
}

func TestToolError_Error_withExtra(t *testing.T) {
	e := &server.ToolError{
		Code:    "scope_mismatch",
		Message: "denied",
		Extra:   map[string]any{"allow_paths": []string{"a", "b"}, "wanted_path": "c"},
	}
	got := e.Error()
	assert.Contains(t, got, "scope_mismatch")
	assert.True(t, strings.Contains(got, "extra=[allow_paths wanted_path]"),
		"Error() = %q; want sorted extra key list", got)
}

func TestRegistry_Get_present(t *testing.T) {
	r := server.NewRegistry()
	r.Add(&server.Tool{Name: "x"})
	_, ok := r.Get("x")
	assert.True(t, ok, "expected Get to find registered tool")
}

func TestRegistry_Get_absent(t *testing.T) {
	r := server.NewRegistry()
	_, ok := r.Get("ghost")
	assert.False(t, ok, "expected Get to return ok=false for missing tool")
}

func TestResult_roundtripThroughRegistry(t *testing.T) {
	r := server.NewRegistry()
	r.Add(&server.Tool{
		Name:        "annotated_tool",
		Annotations: server.Annotations{ReadOnly: true, Idempotent: true},
	})
	tool, ok := r.Get("annotated_tool")
	require.True(t, ok, "tool not found after Add")
	assert.True(t, tool.Annotations.ReadOnly, "Annotations.ReadOnly not preserved through registry")
}
