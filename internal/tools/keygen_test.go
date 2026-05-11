package tools_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gnolang/gno-mcp/internal/mcp/testmcp"
)

func TestKeygen(t *testing.T) {
	h := testmcp.New(t)
	res := h.Call(t, "gno_keygen", map[string]any{"name": "mykey"})
	if res.IsError {
		t.Fatalf("unexpected error: %v", res.Content)
	}

	text := testmcp.TextContent(t, res)

	var got map[string]any
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, text)
	}

	if got["name"] != "mykey" {
		t.Errorf("expected name=mykey, got %v", got["name"])
	}
	if _, ok := got["address"]; !ok {
		t.Error("missing address field")
	}
	if _, ok := got["pubkey"]; !ok {
		t.Error("missing pubkey field")
	}
	// Critically: no mnemonic field
	if _, ok := got["mnemonic"]; ok {
		t.Error("mnemonic field must NOT be present in response")
	}
	// Verify address does not look like mnemonic (basic sanity)
	if strings.Contains(text, "mnemonic") {
		t.Error("mnemonic text must not appear in response")
	}
}
