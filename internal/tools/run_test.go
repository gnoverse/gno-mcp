package tools_test

import (
	"strings"
	"testing"

	"github.com/gnolang/gno-mcp/internal/mcp/testmcp"
)

func TestRun_OnboardingRequired(t *testing.T) {
	h := testmcp.New(t)
	res := h.Call(t, "gno_run", map[string]any{
		"code": `package main; func main() { println("hello") }`,
	})
	if !res.IsError {
		t.Error("expected error when signer is empty")
	}
	text := testmcp.TextContent(t, res)
	if !strings.Contains(text, "onboarding_required") {
		t.Errorf("expected onboarding_required error, got: %s", text)
	}
}

func TestRun_MainnetBlockedWithoutConfirm(t *testing.T) {
	h := testmcp.New(t)
	res := h.Call(t, "gno_run", map[string]any{
		"network": "gno.land",
		"signer":  "mykey",
		"code":    `package main; func main() { println("hello") }`,
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

func TestRun_BroadcastWithConfirm(t *testing.T) {
	h := testmcp.New(t)
	res := h.Call(t, "gno_run", map[string]any{
		"network": "gno.land",
		"signer":  "mykey",
		"code":    `package main; func main() { println("hello") }`,
		"confirm": true,
	})
	if res.IsError {
		t.Fatalf("unexpected error: %v", res.Content)
	}

	var got map[string]any
	testmcp.AsJSON(t, res, &got)
	broadcast, ok := got["broadcast"].(map[string]any)
	if !ok || broadcast == nil {
		t.Errorf("expected broadcast result, got: %v", got["broadcast"])
	}
	if broadcast["tx_hash"] != "FAKEHASH" {
		t.Errorf("expected FAKEHASH, got: %v", broadcast["tx_hash"])
	}
}
