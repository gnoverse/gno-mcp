package write

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/keystore"
	"github.com/gnoverse/gno-mcp/internal/profiles"
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

// newServiceFaucetServer builds a Server whose testnet9999 profile points at a
// faucet service that answers every fund request with status, and generates
// the profile's agent key.
func newServiceFaucetServer(t *testing.T, status int) (*server.Server, *keystore.Keystore, string) {
	t.Helper()
	faucetSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "faucet: dispense failed", status)
	}))
	t.Cleanup(faucetSrv.Close)
	p := testnet9999Profile()
	p.FaucetServiceURL = faucetSrv.URL
	s := newTestnetServerFromProfiles(t, map[string]profiles.Profile{"testnet9999": p})
	ks := keystore.New(t.TempDir(), "", 5)
	addr, err := ks.GenerateForProfile("testnet9999", "", p)
	require.NoError(t, err)
	return s, ks, addr
}

func TestFaucetFund_serverErrorWithABalanceReportsFunded(t *testing.T) {
	s, ks, addr := newServiceFaucetServer(t, http.StatusBadGateway)
	fake := chain.NewFake()
	fake.SetBalance(addr, 10_000_000) // the grant landed before the faucet failed
	RegisterFaucetFund(s, ks, constChainResolver(fake), &http.Client{})

	res, err := s.Registry().Call(context.Background(), "gno_faucet_fund", map[string]any{"profile": "testnet9999"})
	require.NoError(t, err, "a funded key is a success whatever the faucet answered")
	assert.Contains(t, res.Text, addr+": funded")
	assert.Contains(t, res.Text, "did not confirm the grant", "the faucet's failure is noted")
	assert.NotContains(t, res.Text, "dispense failed", "the faucet's body stays out of a success result")
}

func TestFaucetFund_serverErrorWithoutABalanceSaysCheckBeforeRetrying(t *testing.T) {
	s, ks, _ := newServiceFaucetServer(t, http.StatusBadGateway)
	RegisterFaucetFund(s, ks, constChainResolver(chain.NewFake()), &http.Client{})

	// The deadline ends the balance poll early.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_, err := s.Registry().Call(ctx, "gno_faucet_fund", map[string]any{"profile": "testnet9999"})
	require.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, "may still land")
	assert.Contains(t, msg, "gno_account", "the agent reads the balance before calling the faucet again")
	require.Contains(t, msg, "[untrusted faucet response]")
	// The label marks where the faucet's text starts, not where it ends.
	assert.Less(t, strings.Index(msg, "gno_account"), strings.Index(msg, "[untrusted faucet response]"),
		"the guidance precedes the faucet's untrusted body")
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
