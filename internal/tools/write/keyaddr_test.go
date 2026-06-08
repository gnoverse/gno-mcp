package write

import (
	"context"
	"errors"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/keystore"
	"github.com/gnoverse/gno-mcp/internal/server"
)

func TestKeyAddress_localProfile_returnsAddress(t *testing.T) {
	s := newLocalTestServer(t)
	ks := keystore.New(t.TempDir(), "")
	RegisterKeyAddress(s, ks)

	res, err := s.Registry().Call(context.Background(), "gno_key_address", map[string]any{
		"profile": "local",
	})
	if err != nil {
		t.Fatalf("gno_key_address: %v", err)
	}

	const wantAddr = "g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5"
	if res.Text != wantAddr {
		t.Errorf("Text = %q, want %q", res.Text, wantAddr)
	}
	sc := res.StructuredContent
	if sc == nil {
		t.Fatal("StructuredContent is nil")
	}
	if addr, _ := sc["address"].(string); addr != wantAddr {
		t.Errorf("StructuredContent[address] = %q, want %q", addr, wantAddr)
	}
}

func TestKeyAddress_testnetProfile_agentIdentityUnavailable(t *testing.T) {
	s := newBaseTestServer(t)
	ks := keystore.New(t.TempDir(), "")
	RegisterKeyAddress(s, ks)

	_, err := s.Registry().Call(context.Background(), "gno_key_address", map[string]any{
		"profile": "testnet5",
	})
	if err == nil {
		t.Fatal("expected agent_identity_unavailable error, got nil")
	}
	te, ok := errors.AsType[*server.ToolError](err)
	if !ok {
		t.Fatalf("expected *server.ToolError, got %T: %v", err, err)
	}
	if te.Code != "agent_identity_unavailable" {
		t.Errorf("Code = %q, want %q", te.Code, "agent_identity_unavailable")
	}
}
