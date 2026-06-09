//go:build integration

package integration_test

import (
	"log/slog"
	"testing"

	gnoclient "github.com/gnolang/gno/gno.land/pkg/gnoclient"
	"github.com/gnolang/gno/gno.land/pkg/gnoland"
	"github.com/gnolang/gno/gno.land/pkg/gnoland/ugnot"
	"github.com/gnolang/gno/gno.land/pkg/integration"
	"github.com/gnolang/gno/gnovm/pkg/gnoenv"
	"github.com/gnolang/gno/tm2/pkg/std"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/keystore"
)

const nodeChainID = "tendermint_test"

// newNodeBackedRealAddr boots an in-process node, seeds testdata/ realms, and
// returns the chain.Real client together with the node's RPC address.
func newNodeBackedRealAddr(t *testing.T) (*chain.Real, string) {
	t.Helper()
	cfg := integration.TestingMinimalNodeConfig(gnoenv.RootDir())

	loader := integration.NewPkgsLoader()
	require.NoError(t, loader.LoadAllPackagesFromDir("testdata"))
	key, err := integration.GeneratePrivKeyFromMnemonic(keystore.Test1Mnemonic, "", 0, 0)
	require.NoError(t, err)
	fee := std.NewFee(500000, std.MustParseCoin(ugnot.ValueString(1_000_000)))
	txs, err := loader.GenerateTxs(key, fee, nil)
	require.NoError(t, err)

	state := cfg.Genesis.AppState.(gnoland.GnoGenesisState)
	state.Txs = append(state.Txs, txs...)
	cfg.Genesis.AppState = state

	node, remoteAddr := integration.TestingInMemoryNode(t, slog.Default(), cfg)
	t.Cleanup(func() { _ = node.Stop() })

	c, err := chain.NewReal(remoteAddr, nodeChainID)
	require.NoError(t, err)
	return c, remoteAddr
}

func newNodeBackedReal(t *testing.T) *chain.Real {
	t.Helper()
	c, _ := newNodeBackedRealAddr(t)
	return c
}

func test1Signer(t *testing.T) gnoclient.Signer {
	t.Helper()
	s, err := gnoclient.SignerFromBip39(keystore.Test1Mnemonic, nodeChainID, "", 0, 0)
	require.NoError(t, err)
	return s
}
