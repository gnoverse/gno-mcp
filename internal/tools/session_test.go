package tools_test

import (
	"strings"
	"testing"

	"github.com/gnolang/gno-mcp/internal/mcp/testmcp"
)

func TestSessionCreate_NotImplemented(t *testing.T) {
	h := testmcp.New(t)
	res := h.Call(t, "gno_session_create", map[string]any{})
	if !res.IsError {
		t.Error("expected not_implemented error")
	}
	text := testmcp.TextContent(t, res)
	if !strings.Contains(text, "not_implemented") {
		t.Errorf("expected not_implemented in response, got: %s", text)
	}
}

func TestSessionRevoke_NotImplemented(t *testing.T) {
	h := testmcp.New(t)
	res := h.Call(t, "gno_session_revoke", map[string]any{})
	if !res.IsError {
		t.Error("expected not_implemented error")
	}
	text := testmcp.TextContent(t, res)
	if !strings.Contains(text, "not_implemented") {
		t.Errorf("expected not_implemented in response, got: %s", text)
	}
}

func TestSessionList_NotImplemented(t *testing.T) {
	h := testmcp.New(t)
	res := h.Call(t, "gno_session_list", map[string]any{})
	if !res.IsError {
		t.Error("expected not_implemented error")
	}
	text := testmcp.TextContent(t, res)
	if !strings.Contains(text, "not_implemented") {
		t.Errorf("expected not_implemented in response, got: %s", text)
	}
}
