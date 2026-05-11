package tools_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gnolang/gno-mcp/internal/mcp/testmcp"
)

func TestCall_OnboardingRequired(t *testing.T) {
	h := testmcp.New(t)
	res := h.Call(t, "gno_call", map[string]any{
		"path": "gno.land/r/demo/boards",
		"func": "NewBoard",
	})
	if !res.IsError {
		t.Error("expected error when signer is empty")
	}
	text := testmcp.TextContent(t, res)
	if !strings.Contains(text, "onboarding_required") {
		t.Errorf("expected onboarding_required error, got: %s", text)
	}
}

func TestCall_MainnetBlockedWithoutConfirm(t *testing.T) {
	h := testmcp.New(t)
	res := h.Call(t, "gno_call", map[string]any{
		"network": "gno.land",
		"signer":  "mykey",
		"path":    "gno.land/r/demo/boards",
		"func":    "NewBoard",
	})
	if res.IsError {
		t.Fatalf("unexpected error: %v", res.Content)
	}

	var got map[string]any
	testmcp.AsJSON(t, res, &got)

	sec, ok := got["security"].(map[string]any)
	if !ok {
		t.Fatalf("missing security block, got: %v", got)
	}
	if sec["confirmation_required"] != true {
		t.Errorf("expected confirmation_required=true, got: %v", sec["confirmation_required"])
	}
	if got["broadcast"] != nil {
		t.Errorf("expected broadcast=nil without confirm, got: %v", got["broadcast"])
	}
}

func TestCall_BroadcastWithConfirm(t *testing.T) {
	h := testmcp.New(t)
	res := h.Call(t, "gno_call", map[string]any{
		"network": "gno.land",
		"signer":  "mykey",
		"path":    "gno.land/r/demo/boards",
		"func":    "NewBoard",
		"confirm": true,
	})
	if res.IsError {
		t.Fatalf("unexpected error: %v", res.Content)
	}

	text := testmcp.TextContent(t, res)
	var got map[string]any
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, text)
	}

	broadcast, ok := got["broadcast"].(map[string]any)
	if !ok || broadcast == nil {
		t.Errorf("expected broadcast result, got: %v", got["broadcast"])
	}
	if broadcast["tx_hash"] != "FAKEHASH" {
		t.Errorf("expected FAKEHASH, got: %v", broadcast["tx_hash"])
	}
}

func TestCall_Testnet_NoConfirmRequired(t *testing.T) {
	h := testmcp.New(t)
	// On testnet (not gno.land), should broadcast even without confirm
	res := h.Call(t, "gno_call", map[string]any{
		"network": "staging.gno.land",
		"signer":  "mykey",
		"path":    "gno.land/r/demo/boards",
		"func":    "NewBoard",
	})
	if res.IsError {
		t.Fatalf("unexpected error: %v", res.Content)
	}

	var got map[string]any
	testmcp.AsJSON(t, res, &got)
	if got["broadcast"] == nil {
		t.Error("expected broadcast on testnet without explicit confirm")
	}
}
