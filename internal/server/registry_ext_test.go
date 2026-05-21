package server_test

import (
	"testing"

	"github.com/gnoverse/gno-mcp/internal/server"
)

func TestAnnotations_zeroValue(t *testing.T) {
	var a server.Annotations
	if a.ReadOnly || a.Destructive || a.Idempotent || a.OpenWorld {
		t.Error("zero-value Annotations must have all false fields")
	}
}

func TestTool_withAnnotations(t *testing.T) {
	tool := &server.Tool{
		Name: "test_tool",
		Annotations: server.Annotations{
			ReadOnly:    true,
			Destructive: false,
			Idempotent:  true,
			OpenWorld:   false,
		},
	}
	if !tool.Annotations.ReadOnly {
		t.Error("ReadOnly should be true")
	}
	if tool.Annotations.Destructive {
		t.Error("Destructive should be false")
	}
}

func TestResult_withStructuredContent(t *testing.T) {
	r := server.Result{
		Text: "hello",
		StructuredContent: map[string]any{
			"key": "value",
		},
	}
	if r.Text != "hello" {
		t.Errorf("Text = %q, want \"hello\"", r.Text)
	}
	if r.StructuredContent["key"] != "value" {
		t.Errorf("StructuredContent[key] = %v, want \"value\"", r.StructuredContent["key"])
	}
}

func TestResult_isError(t *testing.T) {
	r := server.Result{
		Text:    "something went wrong",
		IsError: true,
	}
	if !r.IsError {
		t.Error("IsError should be true")
	}
}

func TestToolError_Error(t *testing.T) {
	e := &server.ToolError{Code: "x", Message: "bad"}
	if e.Error() != "tool error [x]: bad" {
		t.Errorf("Error() = %q", e.Error())
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
