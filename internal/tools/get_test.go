package tools_test

import (
	"strings"
	"testing"

	"github.com/gnolang/gno-mcp/internal/mcp/testmcp"
)

func TestGet_Render(t *testing.T) {
	h := testmcp.New(t)
	h.Client.Renders["gno.land/r/demo/boards"] = "# Boards"
	res := h.Call(t, "gno_get", map[string]any{"path": "gno.land/r/demo/boards"})
	if res.IsError {
		t.Fatalf("unexpected error: %v", res.Content)
	}
	text := testmcp.TextContent(t, res)
	if !strings.Contains(text, "Boards") {
		t.Errorf("expected 'Boards' in result, got: %s", text)
	}
}

func TestGet_Eval(t *testing.T) {
	h := testmcp.New(t)
	res := h.Call(t, "gno_get", map[string]any{"path": "gno.land/r/foo.Bar()"})
	if res.IsError {
		t.Fatalf("unexpected error: %v", res.Content)
	}
	text := testmcp.TextContent(t, res)
	if !strings.Contains(text, "fake result") {
		t.Errorf("expected 'fake result' in result, got: %s", text)
	}
}

func TestGet_Truncation(t *testing.T) {
	h := testmcp.New(t)
	longContent := strings.Repeat("x", 5000)
	h.Client.Renders["gno.land/r/demo/big"] = longContent
	res := h.Call(t, "gno_get", map[string]any{"path": "gno.land/r/demo/big"})
	if res.IsError {
		t.Fatalf("unexpected error: %v", res.Content)
	}
	text := testmcp.TextContent(t, res)
	if !strings.Contains(text, "output truncated") {
		t.Errorf("expected truncation notice, got: %s", text[:100])
	}
}
