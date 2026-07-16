//go:build integration

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	gnoclient "github.com/gnolang/gno/gno.land/pkg/gnoclient"
	"github.com/gnolang/gno/gno.land/pkg/sdk/vm"
	"github.com/gnolang/gno/tm2/pkg/std"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/keystore"
)

// claDir is the seeded CLA realm package, relative to this test's working dir.
const claDir = "realms/gno.land/r/sys/cla"

// claHash is an arbitrary non-empty hash that activates the gate. Its only job
// is to be non-empty (enforcement on) and to be the value Sign must echo.
const claHash = "0000000000000000000000000000000000000000000000000000000000000000"

// bootCLA boots a CLA-seeded simnet on a kernel-assigned port plus a faucet,
// returning the chain client and the faucet HTTP address.
func bootCLA(t *testing.T, grantUgnot int64) (*chain.Real, string) {
	t.Helper()
	n, rpcAddr, err := Boot(slog.Default(), Config{
		RealmsDir:       "../../../test/e2e/realms",
		ChainID:         testChainID,
		RPCListen:       "tcp://127.0.0.1:0",
		CLAPkgDir:       claDir,
		CLARequiredHash: claHash,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = n.Stop() })

	c, err := chain.NewReal(rpcAddr, testChainID)
	require.NoError(t, err)

	faucetAddr, stop, err := StartFaucet(slog.Default(), FaucetConfig{
		RPCAddr: rpcAddr, ChainID: testChainID, Listen: "127.0.0.1:0", GrantUgnot: grantUgnot,
	})
	require.NoError(t, err)
	t.Cleanup(stop)
	return c, faucetAddr
}

// fundedAgent derives a fresh agent signer at the given BIP-39 index (kept at
// the call site so each test uses a distinct account — avoiding cross-test
// nonce/balance collisions on the shared mnemonic) and funds it via the faucet.
func fundedAgent(t *testing.T, c *chain.Real, faucetAddr string, index uint32, grant int64) (gnoclient.Signer, string) {
	t.Helper()
	signer, err := gnoclient.SignerFromBip39(keystore.Test1Mnemonic, testChainID, "", 0, index)
	require.NoError(t, err)
	info, err := signer.Info()
	require.NoError(t, err)
	addr := info.GetAddress().String()
	fundViaFaucet(t, c, faucetAddr, addr, grant)
	return signer, addr
}

// fundViaFaucet POSTs to the faucet and waits until the address shows the grant.
func fundViaFaucet(t *testing.T, c *chain.Real, faucetAddr, dest string, want int64) {
	t.Helper()
	body := fmt.Sprintf(`{"address":%q,"chain_id":%q}`, dest, testChainID)
	resp, err := http.Post("http://"+faucetAddr+"/fund", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Eventually(t, func() bool {
		bal, err := c.Balance(context.Background(), dest)
		return err == nil && bal >= want
	}, 15*time.Second, 200*time.Millisecond)
}

// tallyRealm is a trivial deployable realm (a bump-a-counter "tally board"),
// mirroring scenario 14's deploy target.
func tallyRealm() []*std.MemFile {
	return []*std.MemFile{
		{Name: "gnomod.toml", Body: "module = \"gno.land/r/simcla/tally\"\ngno = \"0.9\"\n"},
		{Name: "tally.gno", Body: "package tally\n\nvar n int\n\nfunc Bump(cur realm) { n++ }\n"},
	}
}

// TestSimnet_claGate_blocksThenAllows proves the seeded gate replicates the
// live-network CLA behaviour: an unsigned key cannot deploy; after it signs
// r/sys/cla the same deploy lands. Funding is generous so the gate — not the
// balance — is the only variable.
func TestSimnet_claGate_blocksThenAllows(t *testing.T) {
	c, faucetAddr := bootCLA(t, 1_000_000_000) // 1000 GNOT — funds are not the variable here
	ctx := context.Background()

	// The CLA realm rendered the activated gate (sanity: enforcement is ON) and
	// the seeded agreement URL — gno_cla_sign's fetch step extracts it, and the
	// cla-sign-flow scenario's consent check binds on the AUT presenting it.
	render, err := c.Render(ctx, "gno.land/r/sys/cla", "")
	require.NoError(t, err)
	require.Contains(t, render, "ENABLED", "CLA gate should render as enabled")
	require.Contains(t, render, "https://testnet.gnomcp.sim/cla/agreement-v1.txt",
		"the render must carry the seeded agreement URL")

	// A fresh agent key (index 2, distinct from test1@0 and faucet-test@1).
	signer, _ := fundedAgent(t, c, faucetAddr, 2, 1_000_000_000)

	deployPath := "gno.land/r/simcla/tally"

	// Before signing: deploy is blocked by the CLA gate.
	_, err = c.AddPackage(ctx, signer, deployPath, tallyRealm(), false)
	require.Error(t, err, "deploy must be blocked before the CLA is signed")
	if !errors.Is(err, vm.UnauthorizedUserError{}) {
		t.Logf("pre-sign deploy error (not typed as UnauthorizedUserError over the wire): %v", err)
	}

	// Sign the CLA from the agent's own key (the scenario-14 unblock path).
	_, err = c.Call(ctx, signer, "gno.land/r/sys/cla", "Sign", []string{claHash}, "", false)
	require.NoError(t, err, "signing the CLA must succeed")

	// After signing: the same deploy lands.
	_, err = c.AddPackage(ctx, signer, deployPath, tallyRealm(), false)
	require.NoError(t, err, "deploy must succeed after the CLA is signed")
}

// TestSimnet_claEconomics measures the real per-tx cost of the deploy-through-CLA
// flow: how much GNOT a faucet-funded key actually loses to gas fee + storage
// deposit on each of Sign and AddPackage. At the minimum gas fee the per-tx cost
// is dominated by the (tiny, refundable) storage deposit, not the fee.
func TestSimnet_claEconomics(t *testing.T) {
	const grant = 1_000_000_000 // 1000 GNOT, so nothing is starved; we measure deltas
	c, faucetAddr := bootCLA(t, grant)
	ctx := context.Background()

	signer, agent := fundedAgent(t, c, faucetAddr, 3, grant)

	bal := func() int64 {
		b, err := c.Balance(ctx, agent)
		require.NoError(t, err)
		return b
	}

	start := bal()
	t.Logf("ECON funded balance:           %d ugnot (%d GNOT)", start, start/1_000_000)

	signRes, err := c.Call(ctx, signer, "gno.land/r/sys/cla", "Sign", []string{claHash}, "", false)
	require.NoError(t, err)
	afterSign := bal()
	t.Logf("ECON after cla.Sign:           %d ugnot  (delta -%d ugnot, gas_used=%d)",
		afterSign, start-afterSign, signRes.GasUsed)

	deployRes, err := c.AddPackage(ctx, signer, "gno.land/r/simcla/econ", tallyRealm(), false)
	require.NoError(t, err)
	afterDeploy := bal()
	t.Logf("ECON after gno_addpkg:         %d ugnot  (delta -%d ugnot, gas_used=%d)",
		afterDeploy, afterSign-afterDeploy, deployRes.GasUsed)

	t.Logf("ECON total flow cost (Sign+addpkg): %d ugnot (%.3f GNOT) from a %d-GNOT grant",
		start-afterDeploy, float64(start-afterDeploy)/1_000_000, grant/1_000_000)
}

// TestSimnet_claFlow_10gnot proves the fee fix removed the test13 funds wall: with
// DefaultGasFeeUgnot at the chain minimum, a 10 GNOT drip — the original test13
// drip that used to fail at cla.Sign's storage deposit ("lockStorageDeposit ...
// insufficient coins") — now clears the entire Sign+deploy+bump flow with nearly
// the whole grant left. The faucet drip never needed raising; the fee did.
func TestSimnet_claFlow_10gnot(t *testing.T) {
	const grant = 10_000_000 // 10 GNOT — the original test13 drip
	c, faucetAddr := bootCLA(t, grant)
	ctx := context.Background()

	signer, agent := fundedAgent(t, c, faucetAddr, 4, grant)
	deployPath := "gno.land/r/simcla/cleared"

	_, err := c.Call(ctx, signer, "gno.land/r/sys/cla", "Sign", []string{claHash}, "", false)
	require.NoError(t, err, "Sign must clear at the minimum fee even on a 10 GNOT drip")
	_, err = c.AddPackage(ctx, signer, deployPath, tallyRealm(), false)
	require.NoError(t, err, "deploy must clear")
	_, err = c.Call(ctx, signer, deployPath, "Bump", nil, "", false)
	require.NoError(t, err, "bump must clear")

	remaining, err := c.Balance(ctx, agent)
	require.NoError(t, err)
	t.Logf("FLOW 10 GNOT drip @ min fee — Sign+deploy+bump done, %d ugnot (%.2f GNOT) left of %d",
		remaining, float64(remaining)/1_000_000, grant)
	// Three txs at the minimum fee cost well under 1 GNOT total: most of the
	// 10 GNOT grant survives, where the old 10 GNOT fee drained it on tx one.
	require.Greater(t, remaining, int64(9_000_000), "the whole flow should cost under 1 GNOT")
}
