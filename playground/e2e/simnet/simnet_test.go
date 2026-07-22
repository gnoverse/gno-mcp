//go:build integration

package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gnolang/gno/gno.land/pkg/integration"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/keystore"
)

const testChainID = "test-9999"

// bootForTestAddr boots a simnet node on a kernel-assigned port and returns a
// chain client and the raw RPC address. Both bootForTest and the faucet test
// delegate here so the node config is defined exactly once.
func bootForTestAddr(t *testing.T) (*chain.Real, string) {
	t.Helper()
	n, addr, err := Boot(slog.Default(), Config{
		RealmsDir: "../../../test/e2e/realms",
		ChainID:   testChainID,
		RPCListen: "tcp://127.0.0.1:0",
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = n.Stop() })

	c, err := chain.NewReal(addr, testChainID)
	require.NoError(t, err)
	return c, addr
}

// bootForTest boots a simnet node and returns only the chain client.
func bootForTest(t *testing.T) *chain.Real {
	c, _ := bootForTestAddr(t)
	return c
}

func TestSimnet_bootRendersGenesisRealms(t *testing.T) {
	c := bootForTest(t)
	out, err := c.Render(context.Background(), "gno.land/r/test/counter", "")
	require.NoError(t, err)
	require.Equal(t, "# Counter\n\nTotal: 0", out)
}

func TestSimnet_faucetFundsAddress(t *testing.T) {
	c, rpcAddr := bootForTestAddr(t)

	faucetAddr, stop, err := StartFaucet(slog.Default(), FaucetConfig{
		RPCAddr:    rpcAddr,
		ChainID:    testChainID,
		Listen:     "127.0.0.1:0",
		GrantUgnot: 1_000_000,
	})
	require.NoError(t, err)
	t.Cleanup(stop)

	// A fresh address derived from the test mnemonic at index 1 (not test1).
	key, err := integration.GeneratePrivKeyFromMnemonic(keystore.Test1Mnemonic, "", 0, 1)
	require.NoError(t, err)
	dest := key.PubKey().Address().String()

	body := fmt.Sprintf(`{"address":%q,"chain_id":%q}`, dest, testChainID)
	resp, err := http.Post("http://"+faucetAddr+"/fund", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	require.Eventually(t, func() bool {
		bal, err := c.Balance(context.Background(), dest)
		return err == nil && bal >= 1_000_000
	}, 15*time.Second, 200*time.Millisecond)
}

func TestSimnet_gnowebAdvertisesGnoconnect(t *testing.T) {
	_, rpcAddr := bootForTestAddr(t)

	const advertised = "http://testnet.gnomcp.sim:26687"
	webAddr, stop, err := StartGnoweb(slog.Default(), GnowebConfig{
		NodeRPC: rpcAddr, AdvertisedRPC: advertised,
		ChainID: testChainID, Listen: "127.0.0.1:0",
	})
	require.NoError(t, err)
	t.Cleanup(stop)

	resp, err := http.Get("http://" + webAddr + "/")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), `name="gnoconnect:rpc" content="`+advertised+`"`)
	require.Contains(t, string(body), `name="gnoconnect:chainid" content="`+testChainID+`"`)
}
