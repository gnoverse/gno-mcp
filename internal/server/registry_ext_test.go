package server_test

import (
	"strings"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/server"
)

func TestAnnotations_Validate_okWhenIndependent(t *testing.T) {
	cases := []server.Annotations{
		{},
		{ReadOnly: true, Idempotent: true},
		{Destructive: true, OpenWorld: true},
		{Idempotent: true, OpenWorld: true},
	}
	for i, a := range cases {
		if err := a.Validate(); err != nil {
			t.Errorf("case %d: unexpected error: %v", i, err)
		}
	}
}

func TestAnnotations_Validate_rejectsReadOnlyAndDestructive(t *testing.T) {
	a := server.Annotations{ReadOnly: true, Destructive: true}
	if err := a.Validate(); err == nil {
		t.Fatal("expected error when ReadOnly and Destructive are both true")
	}
}

func TestToolError_Error(t *testing.T) {
	e := &server.ToolError{Code: "x", Message: "bad"}
	if e.Error() != "tool error [x]: bad" {
		t.Errorf("Error() = %q", e.Error())
	}
}

func TestToolError_Error_withExtra(t *testing.T) {
	e := &server.ToolError{
		Code:    "scope_mismatch",
		Message: "denied",
		Extra:   map[string]any{"allow_paths": []string{"a", "b"}, "wanted_path": "c"},
	}
	got := e.Error()
	if !strings.Contains(got, "scope_mismatch") {
		t.Errorf("Error() = %q; want it to contain code", got)
	}
	if !strings.Contains(got, "extra=[allow_paths wanted_path]") {
		t.Errorf("Error() = %q; want sorted extra key list", got)
	}
}

func TestRegistry_Get_present(t *testing.T) {
	r := server.NewRegistry()
	r.Add(&server.Tool{Name: "x"})
	if _, ok := r.Get("x"); !ok {
		t.Error("expected Get to find registered tool")
	}
}

func TestRegistry_Get_absent(t *testing.T) {
	r := server.NewRegistry()
	if _, ok := r.Get("ghost"); ok {
		t.Error("expected Get to return ok=false for missing tool")
	}
}

func TestResult_roundtripThroughRegistry(t *testing.T) {
	r := server.NewRegistry()
	r.Add(&server.Tool{
		Name:        "annotated_tool",
		Annotations: server.Annotations{ReadOnly: true, Idempotent: true},
	})
	tool, ok := r.Get("annotated_tool")
	if !ok {
		t.Fatal("tool not found after Add")
	}
	if !tool.Annotations.ReadOnly {
		t.Error("Annotations.ReadOnly not preserved through registry")
	}
}
