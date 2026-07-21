//go:build integration

package integration_test

import (
	"context"
	"testing"

	gnoclient "github.com/gnolang/gno/gno.land/pkg/gnoclient"
	rpcclient "github.com/gnolang/gno/tm2/pkg/bft/rpc/client"
	"github.com/gnolang/gno/tm2/pkg/crypto"
	"github.com/gnolang/gno/tm2/pkg/sdk/auth"
	"github.com/gnolang/gno/tm2/pkg/std"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/session"
)

// createSessionOnChain broadcasts a master-signed MsgCreateSession for kp with
// the given ugnot spend limit, scoped to the counter realm. Returns the master
// (test1) address.
func createSessionOnChain(t *testing.T, remoteAddr string, kp *session.Keypair, spendLimitUgnot int64) crypto.Address {
	t.Helper()
	signer := test1Signer(t)
	info, err := signer.Info()
	require.NoError(t, err)
	master := info.GetAddress()

	rpc, err := rpcclient.NewHTTPClient(remoteAddr)
	require.NoError(t, err)
	cli := &gnoclient.Client{RPCClient: rpc, Signer: signer}

	msg := auth.MsgCreateSession{
		Creator:    master,
		SessionKey: kp.TMPubKey(),
		ExpiresAt:  0, // valid until revoked
		AllowPaths: []string{"vm/exec:" + counterRealm},
		SpendLimit: std.Coins{std.Coin{Denom: "ugnot", Amount: spendLimitUgnot}},
	}
	_, err = cli.CreateSession(gnoclient.BaseTxCfg{GasFee: "1000000ugnot", GasWanted: 5_000_000}, msg)
	require.NoError(t, err, "broadcast MsgCreateSession")
	return master
}

// TestIntegration_sessionSpendPreflight pins the client-side spend pre-check
// against a REAL on-chain session — the exact failure shape from test-13: a
// grant whose spend limit cannot cover one write's live gas fee. Both
// simulate and broadcast must be refused client-side with the numbers and
// recovery hint, never the chain's bare "session not allowed error".
func TestIntegration_sessionSpendPreflight(t *testing.T) {
	c, remoteAddr := newNodeBackedRealAddr(t)
	ctx := context.Background()

	liveFee, err := c.GasFeeUgnot(ctx)
	require.NoError(t, err)

	kp, err := session.NewKeypair()
	require.NoError(t, err)
	master := createSessionOnChain(t, remoteAddr, kp, liveFee-1) // one ugnot short of a single write

	for _, simulate := range []bool{true, false} {
		_, err := c.CallAsUser(ctx, kp, master.String(), counterRealm, "Increment", nil, "", simulate)
		require.Error(t, err, "simulate=%v", simulate)
		require.Contains(t, err.Error(), "session spend pre-check", "simulate=%v", simulate)
		require.Contains(t, err.Error(), "gno_session_propose", "simulate=%v: recovery hint missing", simulate)
	}
}

// TestIntegration_sessionSpendWithinLimitBroadcasts is the control: an
// identical session whose limit affords the write goes through end to end —
// proving the pre-flight only refuses what the chain itself would refuse.
func TestIntegration_sessionSpendWithinLimitBroadcasts(t *testing.T) {
	c, remoteAddr := newNodeBackedRealAddr(t)
	ctx := context.Background()

	liveFee, err := c.GasFeeUgnot(ctx)
	require.NoError(t, err)

	kp, err := session.NewKeypair()
	require.NoError(t, err)
	master := createSessionOnChain(t, remoteAddr, kp, liveFee*10)

	res, err := c.CallAsUser(ctx, kp, master.String(), counterRealm, "Increment", nil, "", false)
	require.NoError(t, err, "session-signed broadcast")
	require.Equal(t, liveFee, res.GasFeeUgnot)

	out, err := c.Eval(ctx, counterRealm, "Total()")
	require.NoError(t, err)
	require.Contains(t, out, "1")
}
