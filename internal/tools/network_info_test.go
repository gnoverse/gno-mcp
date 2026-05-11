package tools_test

import (
	"testing"

	"github.com/gnolang/gno-mcp/internal/client"
	"github.com/gnolang/gno-mcp/internal/mcp/testmcp"
)

func TestNetworkInfo(t *testing.T) {
	h := testmcp.New(t)
	h.Client.Network = client.NetworkInfo{Chain: "portal-loop", Domain: "gno.land", RPC: "http://x:26657", Height: 123}
	res := h.Call(t, "gno_network_info", map[string]any{"domain": "gno.land"})
	var got client.NetworkInfo
	testmcp.AsJSON(t, res, &got)
	if got.Chain != "portal-loop" || got.Height != 123 {
		t.Errorf("bad info: %+v", got)
	}
}
