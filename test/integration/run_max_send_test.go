//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/gnolang/gno/tm2/pkg/crypto"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/session"
)

// payScript is a MsgRun script that sends 5ugnot out of the caller's account
// to payee.
func payScript(payee crypto.Address) string {
	return `package main

import (
	"chain"
	"chain/banker"
)

func main(cur realm) {
	b := banker.NewBanker(banker.BankerTypeRealmSend, cur)
	b.SendCoins(cur.Address(), "` + payee.String() + `", chain.Coins{{"ugnot", 5}})
}
`
}

// From gno v1.8.0 a MsgRun may send out of the caller's account only up to its
// MaxSend; an empty MaxSend sends nothing.
func TestIntegration_runMaxSendCapsAgentTransfers(t *testing.T) {
	c := newNodeBackedReal(t)
	ctx := context.Background()
	payee := crypto.AddressFromPreimage([]byte("agent-run-payee"))
	code := payScript(payee)

	_, err := c.Run(ctx, test1Signer(t), code, "", true)
	require.ErrorContains(t, err, "max send")

	_, err = c.Run(ctx, test1Signer(t), code, "5ugnot", false)
	require.NoError(t, err)
	got, err := c.Balance(ctx, payee.String())
	require.NoError(t, err)
	require.Equal(t, int64(5), got)
}

func TestIntegration_runMaxSendCapsSessionTransfers(t *testing.T) {
	c, remoteAddr := newNodeBackedRealAddr(t)
	ctx := context.Background()
	payee := crypto.AddressFromPreimage([]byte("session-run-payee"))
	code := payScript(payee)

	liveFee, err := c.GasFeeUgnot(ctx)
	require.NoError(t, err)
	kp, err := session.NewKeypair()
	require.NoError(t, err)
	master := createScopedSessionOnChain(t, remoteAddr, kp, liveFee*100, []string{"vm/run"})

	_, err = c.RunAsUser(ctx, kp, master.String(), code, "", true)
	require.ErrorContains(t, err, "max send")

	_, err = c.RunAsUser(ctx, kp, master.String(), code, "5ugnot", false)
	require.NoError(t, err)
	got, err := c.Balance(ctx, payee.String())
	require.NoError(t, err)
	require.Equal(t, int64(5), got)
}
