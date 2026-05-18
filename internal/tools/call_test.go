package tools_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gnolang/gno-mcp/internal/client"
	"github.com/gnolang/gno-mcp/internal/mcp/testmcp"
	"github.com/gnolang/gno-mcp/internal/session"
)

// TestCall_AuthRequired covers the v0.2 "OAuth-style" gate: no signer, no
// authorized session → return authentication_required with a fund link/QR.
func TestCall_AuthRequired(t *testing.T) {
	h := testmcp.New(t)
	res := h.Call(t, "gno_call", map[string]any{
		"path": "gno.land/r/demo/boards",
		"func": "NewBoard",
	})
	if !res.IsError {
		t.Fatal("expected error when no signer and session is unfunded")
	}
	text := testmcp.TextContent(t, res)
	if !strings.Contains(text, `"code":"authentication_required"`) {
		t.Errorf("expected authentication_required code, got: %s", text)
	}
	for _, want := range []string{"session_address", "fund_url", "qr_ascii", "threshold_ugnot"} {
		if !strings.Contains(text, want) {
			t.Errorf("auth payload missing %q in response: %s", want, text)
		}
	}
}

// TestCall_AuthorizedSessionSigns covers the success path: fund the session
// address through the fake client, refresh, then a no-signer call succeeds
// using the session signer.
func TestCall_AuthorizedSessionSigns(t *testing.T) {
	h := testmcp.New(t)

	// Bootstrap a pending session and credit its address on the fake chain.
	if err := h.Session.EnsurePending(); err != nil {
		t.Fatal(err)
	}
	addr := h.Session.Address()
	h.Client.Addresses[addr] = &client.AddressInfo{Address: addr, Balance: "5000000ugnot"}

	if err := h.Session.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if h.Session.State() != session.StateAuthenticated {
		t.Fatalf("session not authenticated, state=%s", h.Session.State())
	}

	res := h.Call(t, "gno_call", map[string]any{
		"network": "staging.gno.land",
		"path":    "gno.land/r/demo/boards",
		"func":    "NewBoard",
	})
	if res.IsError {
		t.Fatalf("authorized call failed: %s", testmcp.TextContent(t, res))
	}
	var got map[string]any
	testmcp.AsJSON(t, res, &got)
	sec, _ := got["security"].(map[string]any)
	if sec["signer"] != "mcp-session" {
		t.Errorf("signer = %v, want mcp-session", sec["signer"])
	}
	if got["broadcast"] == nil {
		t.Error("authorized testnet call should broadcast")
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
