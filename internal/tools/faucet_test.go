package tools_test

import (
	"strings"
	"testing"

	"github.com/gnolang/gno-mcp/internal/mcp/testmcp"
)

func TestFaucetRequest_MainnetRejected(t *testing.T) {
	h := testmcp.New(t)
	res := h.Call(t, "gno_faucet_request", map[string]any{
		"network": "gno.land",
		"address": "g1test",
	})
	if !res.IsError {
		t.Error("expected error for mainnet faucet request")
	}
	text := testmcp.TextContent(t, res)
	if !strings.Contains(text, "mainnet") {
		t.Errorf("expected mainnet rejection message, got: %s", text)
	}
}

func TestFaucetRequest_Testnet(t *testing.T) {
	h := testmcp.New(t)
	addr := "g1testaddr"
	res := h.Call(t, "gno_faucet_request", map[string]any{
		"network": "staging.gno.land",
		"address": addr,
	})
	if res.IsError {
		t.Fatalf("unexpected error: %v", res.Content)
	}
	text := testmcp.TextContent(t, res)
	if !strings.Contains(text, "ok") {
		t.Errorf("expected ok status, got: %s", text)
	}
	if !strings.Contains(text, addr) {
		t.Errorf("expected address in response, got: %s", text)
	}
}
