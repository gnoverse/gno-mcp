package tools_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gnolang/gno-mcp/internal/mcp/testmcp"
)

func TestConfigGet(t *testing.T) {
	// Use a temp file to avoid touching real config
	cfgFile := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("GNO_MCP_CONFIG", cfgFile)

	h := testmcp.New(t)
	res := h.Call(t, "gno_config_get", map[string]any{})
	if res.IsError {
		t.Fatalf("unexpected error: %v", res.Content)
	}
	text := testmcp.TextContent(t, res)
	// Empty config should marshal to {} or {"default_key":"","default_network":"","gas_buffer":0}
	if !strings.Contains(text, "{") {
		t.Errorf("expected JSON object, got: %s", text)
	}
}

func TestConfigSet_DefaultKey(t *testing.T) {
	cfgFile := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("GNO_MCP_CONFIG", cfgFile)

	h := testmcp.New(t)
	res := h.Call(t, "gno_config_set", map[string]any{
		"key":   "default_key",
		"value": "mykey",
	})
	if res.IsError {
		t.Fatalf("unexpected error: %v", res.Content)
	}
	text := testmcp.TextContent(t, res)
	if !strings.Contains(text, "ok") {
		t.Errorf("expected ok status, got: %s", text)
	}

	// Verify the change persists via get
	res2 := h.Call(t, "gno_config_get", map[string]any{})
	text2 := testmcp.TextContent(t, res2)
	if !strings.Contains(text2, "mykey") {
		t.Errorf("expected mykey in config, got: %s", text2)
	}
}

func TestConfigSet_GasBuffer(t *testing.T) {
	cfgFile := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("GNO_MCP_CONFIG", cfgFile)

	h := testmcp.New(t)
	res := h.Call(t, "gno_config_set", map[string]any{
		"key":   "gas_buffer",
		"value": "150",
	})
	if res.IsError {
		t.Fatalf("unexpected error: %v", res.Content)
	}

	res2 := h.Call(t, "gno_config_get", map[string]any{})
	text := testmcp.TextContent(t, res2)
	if !strings.Contains(text, "150") {
		t.Errorf("expected gas_buffer=150, got: %s", text)
	}
}

func TestConfigSet_UnknownKey(t *testing.T) {
	cfgFile := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("GNO_MCP_CONFIG", cfgFile)

	h := testmcp.New(t)
	res := h.Call(t, "gno_config_set", map[string]any{
		"key":   "unknown_key",
		"value": "val",
	})
	if !res.IsError {
		t.Error("expected error for unknown config key")
	}

	// Verify GNO_MCP_CONFIG is used (ensure test isolation)
	_ = os.Remove(cfgFile)
}
