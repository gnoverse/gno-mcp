//go:build integration

package integration_test

import (
	"context"
	"net/http/httptest"
	"testing"

	gnoclient "github.com/gnolang/gno/gno.land/pkg/gnoclient"
	rpcclient "github.com/gnolang/gno/tm2/pkg/bft/rpc/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/faucet"
	"github.com/gnoverse/gno-mcp/internal/clientfaucet"
	"github.com/gnoverse/gno-mcp/internal/profiles"
)

func TestIntegration_faucetDispense(t *testing.T) {
	real, addr := newNodeBackedRealAddr(t)

	// Funding side: a gnoclient.Client (test1 on the node) drives the dispenser.
	rpc, err := rpcclient.NewHTTPClient(addr)
	require.NoError(t, err)
	signer := test1Signer(t)
	cli := &gnoclient.Client{RPCClient: rpc, Signer: signer}
	info, err := signer.Info()
	require.NoError(t, err)
	disp := faucet.NewGnoclientDispenser(cli, info.GetAddress(), "10000000ugnot", 10_000_000)

	// The faucet service. Its chain-id guard requires a test* id; the node is
	// "tendermint_test" (which the guard rejects), so use "test5" as the guard
	// label — the dispenser signs on the node's real chain regardless.
	f := faucet.New("test5", 1_000_000, disp, faucet.NewLimiter(faucet.LimiterCfg{
		PerAddrMax: 1, PerIPMax: 5, DailyCapUgnot: 1_000_000_000, GrantUgnot: 1_000_000,
	}))
	srv := httptest.NewServer(f.Handler())
	defer srv.Close()

	// Client side: ServiceFaucet via Resolve, balance-confirm via the real node.
	fac := clientfaucet.Resolve(
		profiles.Profile{FaucetServiceURL: srv.URL, ChainID: "test5"},
		real, srv.Client(),
	)

	// A fresh, valid, unfunded recipient (any valid g1 bech32 that isn't test1).
	const recipient = "g17ernafy6ctpcz6uepfsq2js8x2vz0wladh5yc3"

	out, err := fac.Fund(context.Background(), recipient, "test5")
	require.NoError(t, err, "fund via service")
	assert.Equal(t, "service", out.Backend)
	assert.NotEmpty(t, out.TxHash, "should return a dispense tx hash")

	funded, err := fac.Funded(context.Background(), recipient)
	require.NoError(t, err)
	assert.True(t, funded, "recipient balance should be positive on-chain after dispense")

	// Second request, same address → per-address cooldown → service returns an error (429).
	_, err = fac.Fund(context.Background(), recipient, "test5")
	require.Error(t, err, "second fund of same address should be rate-limited")
}
