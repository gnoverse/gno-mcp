package tools_test

import (
	"strings"
	"testing"

	"github.com/gnolang/gno-mcp/internal/mcp/testmcp"
)

func TestEval(t *testing.T) {
	h := testmcp.New(t)
	res := h.Call(t, "gno_eval", map[string]any{
		"expr":    "gno.land/r/demo/boards.GetPost(1)",
		"network": "gno.land",
	})
	if res.IsError {
		t.Fatalf("unexpected error: %v", res.Content)
	}
	text := testmcp.TextContent(t, res)
	if !strings.Contains(text, "fake result") {
		t.Errorf("expected 'fake result' in output, got: %s", text)
	}
}

func TestEval_DefaultNetwork(t *testing.T) {
	h := testmcp.New(t)
	res := h.Call(t, "gno_eval", map[string]any{
		"expr": "gno.land/r/demo/boards.GetPost(1)",
	})
	if res.IsError {
		t.Fatalf("unexpected error: %v", res.Content)
	}
}
