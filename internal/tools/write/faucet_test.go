package write

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/keystore"
	"github.com/gnoverse/gno-mcp/internal/server"
)

func TestFaucetFund_linkBackend_reportsFunded(t *testing.T) {
	s := newTestnetTestServer(t)
	ks := keystore.New(t.TempDir(), "", 5)
	addr, err := ks.GenerateForProfile("testnet9999", "", testnet9999Profile())
	require.NoError(t, err)

	fake := chain.NewFake()
	fake.SetBalance(addr, 1_000_000) // already funded -> poll returns immediately
	RegisterFaucetFund(s, ks, constChainResolver(fake), &http.Client{})

	res, err := s.Registry().Call(context.Background(), "gno_faucet_fund", map[string]any{"profile": "testnet9999"})
	require.NoError(t, err)
	assert.Contains(t, res.Text, addr)
	assert.Contains(t, res.Text, "funded")
}

func TestFaucetFund_missingProfileHint(t *testing.T) {
	s := newTestnetTestServer(t)
	ks := keystore.New(t.TempDir(), "", 5)
	RegisterFaucetFund(s, ks, constChainResolver(chain.NewFake()), &http.Client{})

	_, err := s.Registry().Call(context.Background(), "gno_faucet_fund", map[string]any{"profile": ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pick one of the configured profiles",
		"the missing-profile hint must match the other write tools")
}

func TestFaucetFund_keystoreUnconfigured(t *testing.T) {
	s := newTestnetTestServer(t)
	ks := keystore.New("", "", 5) // no agent-keys directory configured
	RegisterFaucetFund(s, ks, constChainResolver(chain.NewFake()), &http.Client{})

	_, err := s.Registry().Call(context.Background(), "gno_faucet_fund", map[string]any{"profile": "testnet9999"})
	require.Error(t, err)
	var te *server.ToolError
	require.ErrorAs(t, err, &te)
	assert.Equal(t, "key_storage_unconfigured", te.Code)
}

func TestFaucetFund_noAgentKey(t *testing.T) {
	s := newTestnetTestServer(t)
	ks := keystore.New(t.TempDir(), "", 5) // no key generated
	RegisterFaucetFund(s, ks, constChainResolver(chain.NewFake()), &http.Client{})

	_, err := s.Registry().Call(context.Background(), "gno_faucet_fund", map[string]any{"profile": "testnet9999"})
	require.Error(t, err)
	var te *server.ToolError
	require.ErrorAs(t, err, &te)
	assert.Equal(t, "agent_identity_unavailable", te.Code)
	assert.Contains(t, te.Message, "gno_key_generate")
}
