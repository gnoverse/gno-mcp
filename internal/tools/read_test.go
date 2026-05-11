package tools_test

import (
	"strings"
	"testing"

	"github.com/gnolang/gno-mcp/internal/mcp/testmcp"
)

func TestRead_Source(t *testing.T) {
	h := testmcp.New(t)
	res := h.Call(t, "gno_read", map[string]any{
		"path": "gno.land/r/demo/boards",
	})
	if res.IsError {
		t.Fatalf("unexpected error: %v", res.Content)
	}
	text := testmcp.TextContent(t, res)
	if !strings.Contains(text, "fake source") {
		t.Errorf("expected source content, got: %s", text)
	}
}

func TestRead_SliceBySymbol(t *testing.T) {
	h := testmcp.New(t)
	res := h.Call(t, "gno_read", map[string]any{
		"path":   "gno.land/r/demo/boards",
		"symbol": "Board",
	})
	if res.IsError {
		t.Fatalf("unexpected error: %v", res.Content)
	}
	text := testmcp.TextContent(t, res)
	// With slice, should return untrusted envelope, not summary
	if strings.Contains(text, "summary") {
		t.Errorf("expected source, not summary, got: %s", text)
	}
}

func TestRead_LargeWithoutSlice(t *testing.T) {
	h := testmcp.New(t)
	h.Client.Sources["gno.land/r/demo/big||"] = strings.Repeat("// code\n", 600)
	res := h.Call(t, "gno_read", map[string]any{
		"path": "gno.land/r/demo/big",
	})
	if res.IsError {
		t.Fatalf("unexpected error: %v", res.Content)
	}
	text := testmcp.TextContent(t, res)
	if !strings.Contains(text, "summary") {
		t.Errorf("expected summary for large content, got: %s", text[:100])
	}
}
