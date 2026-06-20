//go:build integration

package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	gnoclient "github.com/gnolang/gno/gno.land/pkg/gnoclient"
	"github.com/gnolang/gno/gno.land/pkg/gnoland/ugnot"
	rpcclient "github.com/gnolang/gno/tm2/pkg/bft/rpc/client"
	"github.com/gnolang/gno/tm2/pkg/crypto"
	"github.com/gnolang/gno/tm2/pkg/sdk/bank"
	"github.com/gnolang/gno/tm2/pkg/std"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/keystore"
)

// TestSimnet_feeIsWhatWeOffer answers "is the 10 GNOT/tx our fault?" empirically:
// it sends the SAME bank.MsgSend at three different offered GasFees and measures
// the sender's balance delta each time. A bank send locks no storage deposit, so
// the delta is exactly (amount sent) + (fee the chain actually charged). If the
// delta tracks the offered fee, the chain takes the full fee we offer — i.e. the
// 10 GNOT is purely gno-mcp's defaultBaseTxCfg, not a chain cost.
func TestSimnet_feeIsWhatWeOffer(t *testing.T) {
	n, rpcAddr, err := Boot(slog.Default(), Config{
		RealmsDir: "../../../test/e2e/realms",
		ChainID:   testChainID,
		RPCListen: "tcp://127.0.0.1:0",
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = n.Stop() })

	c, err := chain.NewReal(rpcAddr, testChainID)
	require.NoError(t, err)

	faucetAddr, stop, err := StartFaucet(slog.Default(), FaucetConfig{
		RPCAddr: rpcAddr, ChainID: testChainID, Listen: "127.0.0.1:0", GrantUgnot: 1_000_000_000,
	})
	require.NoError(t, err)
	t.Cleanup(stop)

	rpc, err := rpcclient.NewHTTPClient(rpcAddr)
	require.NoError(t, err)

	// A throwaway recipient (index 9) so the send has somewhere to go.
	dstSigner, err := gnoclient.SignerFromBip39(keystore.Test1Mnemonic, testChainID, "", 0, 9)
	require.NoError(t, err)
	dstInfo, err := dstSigner.Info()
	require.NoError(t, err)
	dst := dstInfo.GetAddress()

	const sendAmount = 1_000_000 // 1 GNOT moved per send (constant across runs)

	// Each sub-run uses a distinct sender index so balances don't interfere.
	cases := []struct {
		name     string
		index    uint32
		feeUgnot int64
	}{
		{"fee_min_our_default", 10, chain.DefaultGasFeeUgnot}, // 10_000 ugnot
		{"fee_1_GNOT", 11, 1_000_000},
		{"fee_10_GNOT_old_default", 12, 10_000_000},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			signer, err := gnoclient.SignerFromBip39(keystore.Test1Mnemonic, testChainID, "", 0, tc.index)
			require.NoError(t, err)
			info, err := signer.Info()
			require.NoError(t, err)
			from := info.GetAddress().String()

			// Fund the sender via the faucet.
			body := fmt.Sprintf(`{"address":%q,"chain_id":%q}`, from, testChainID)
			resp, err := http.Post("http://"+faucetAddr+"/fund", "application/json", strings.NewReader(body))
			require.NoError(t, err)
			resp.Body.Close()
			require.Eventually(t, func() bool {
				b, err := c.Balance(context.Background(), from)
				return err == nil && b >= 1_000_000_000
			}, 15*time.Second, 200*time.Millisecond)

			before, err := c.Balance(context.Background(), from)
			require.NoError(t, err)

			cli := &gnoclient.Client{RPCClient: rpc, Signer: signer}
			cfg := gnoclient.BaseTxCfg{
				GasFee:    fmt.Sprintf("%dugnot", tc.feeUgnot),
				GasWanted: chain.DefaultGasWanted,
			}
			msg := bankMsgSend(info.GetAddress(), dst, sendAmount)
			res, err := cli.Send(cfg, msg)
			require.NoError(t, err, "send must succeed at offered fee %d ugnot", tc.feeUgnot)

			after, err := c.Balance(context.Background(), from)
			require.NoError(t, err)

			delta := before - after
			feeCharged := delta - sendAmount
			t.Logf("FEE offered=%d ugnot  -> balance delta=%d  (sent %d, fee charged=%d, gas_used=%d)",
				tc.feeUgnot, delta, sendAmount, feeCharged, res.DeliverTx.GasUsed)
		})
	}
}

func bankMsgSend(from, to crypto.Address, amount int64) bank.MsgSend {
	return bank.MsgSend{
		FromAddress: from,
		ToAddress:   to,
		Amount:      std.Coins{{Denom: ugnot.Denom, Amount: amount}},
	}
}
