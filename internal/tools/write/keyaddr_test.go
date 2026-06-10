package write

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	require.NoError(t, err, "gno_key_address")

	const wantAddr = "g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5"
	assert.Equal(t, wantAddr, res.Text)
	require.NotNil(t, res.StructuredContent)
	addr, _ := res.StructuredContent["address"].(string)
	assert.Equal(t, wantAddr, addr)
}

func TestKeyAddress_keystoreUnconfigured(t *testing.T) {
	s := newBaseTestServer(t)
	ks := keystore.New("", "") // no agent-keys directory configured
	RegisterKeyAddress(s, ks)

	_, err := s.Registry().Call(context.Background(), "gno_key_address", map[string]any{
		"profile": "testnet5",
	})
	require.Error(t, err, "expected key_storage_unconfigured error, got nil")
	var te *server.ToolError
	require.ErrorAs(t, err, &te)
	assert.Equal(t, "key_storage_unconfigured", te.Code)
}

func TestKeyAddress_testnetProfile_agentIdentityUnavailable(t *testing.T) {
	s := newBaseTestServer(t)
	ks := keystore.New(t.TempDir(), "")
	RegisterKeyAddress(s, ks)

	_, err := s.Registry().Call(context.Background(), "gno_key_address", map[string]any{
		"profile": "testnet5",
	})
	require.Error(t, err, "expected agent_identity_unavailable error, got nil")
	var te *server.ToolError
	require.ErrorAs(t, err, &te)
	assert.Equal(t, "agent_identity_unavailable", te.Code)
}
