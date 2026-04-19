package mcp_test

import (
	"slices"
	"testing"

	_ "github.com/gnolang/gno-mcp/internal/tools" // trigger all init() registrations
	"github.com/gnolang/gno-mcp/internal/mcp/testmcp"
)

func TestAllToolsRegistered(t *testing.T) {
	h := testmcp.New(t)
	names := h.ListTools(t)

	want := []string{
		"gno_network_info",
		"gno_get",
		"gno_eval",
		"gno_read",
		"gno_inspect",
		"gno_address_info",
		"gno_keygen",
		"gno_faucet_request",
		"gno_call",
		"gno_run",
		"gno_session_create",
		"gno_session_revoke",
		"gno_session_list",
		"gno_config_get",
		"gno_config_set",
		"gno_audit_tail",
	}

	for _, w := range want {
		if !slices.Contains(names, w) {
			t.Errorf("missing tool: %s", w)
		}
	}

	t.Logf("registered tools (%d): %v", len(names), names)
}
