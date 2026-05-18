package tools_test

import (
	"context"
	"strings"
	"testing"

	"github.com/gnolang/gno-mcp/internal/client"
	"github.com/gnolang/gno-mcp/internal/mcp/testmcp"
)

func TestAuthStatus_FreshReturnsPendingWithQR(t *testing.T) {
	h := testmcp.New(t)
	res := h.Call(t, "gno_auth_status", map[string]any{})
	if res.IsError {
		t.Fatalf("unexpected error: %s", testmcp.TextContent(t, res))
	}
	var got map[string]any
	testmcp.AsJSON(t, res, &got)

	if got["state"] != "pending" {
		t.Errorf("state = %v, want pending", got["state"])
	}
	for _, want := range []string{"session_address", "fund_url", "web_fund_url", "qr_ascii", "human_guidance"} {
		if got[want] == "" || got[want] == nil {
			t.Errorf("missing %q in auth_status response: %+v", want, got)
		}
	}
	addr, _ := got["session_address"].(string)
	if !strings.HasPrefix(addr, "gmcp1") {
		t.Errorf("session_address must be a gmcp bech32 address, got %q", addr)
	}
}

func TestAuthStatus_AuthenticatedHidesAuthPayload(t *testing.T) {
	h := testmcp.New(t)
	if err := h.Session.EnsurePending(); err != nil {
		t.Fatal(err)
	}
	addr := h.Session.Address()
	h.Client.Addresses[addr] = &client.AddressInfo{Address: addr, Balance: "10000000ugnot"}
	if err := h.Session.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}

	res := h.Call(t, "gno_auth_status", map[string]any{})
	var got map[string]any
	testmcp.AsJSON(t, res, &got)
	if got["state"] != "authenticated" {
		t.Errorf("state = %v, want authenticated", got["state"])
	}
	// fund_url etc. should not be present once authenticated — keeps responses tight.
	if _, ok := got["fund_url"]; ok {
		t.Error("fund_url must not appear on an authenticated session")
	}
}

func TestAuthStatus_DoesNotForcePendingWhenOptOut(t *testing.T) {
	h := testmcp.New(t)
	res := h.Call(t, "gno_auth_status", map[string]any{"ensure_pending": false})
	var got map[string]any
	testmcp.AsJSON(t, res, &got)
	if got["state"] != "unauthenticated" {
		t.Errorf("state = %v, want unauthenticated (ensure_pending=false)", got["state"])
	}
}
