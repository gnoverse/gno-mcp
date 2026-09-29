package faucet

import (
	"context"
	"errors"
	"testing"

	gnoclient "github.com/gnolang/gno/gno.land/pkg/gnoclient"
	"github.com/gnolang/gno/gno.land/pkg/gnoland"
	"github.com/gnolang/gno/tm2/pkg/amino"
	abci "github.com/gnolang/gno/tm2/pkg/bft/abci/types"
	rpcclient "github.com/gnolang/gno/tm2/pkg/bft/rpc/client"
	ctypes "github.com/gnolang/gno/tm2/pkg/bft/rpc/core/types"
	"github.com/gnolang/gno/tm2/pkg/bft/types"
	"github.com/gnolang/gno/tm2/pkg/std"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// test1Mnemonic is the well-known gno test1 seed.
const test1Mnemonic = "source bonus chronic canvas draft south burst lottery vacant surface solve popular case indicate oppose farm nothing bullet exhibit title speed wink action roast"

// fakeRPC answers the two queries a dispense makes (gas price, funding account)
// and replies to broadcast_tx_commit with a scripted result. Any other RPC
// method panics on the nil embedded client.
type fakeRPC struct {
	rpcclient.Client
	accountErr  error
	broadcast   func() (*ctypes.ResultBroadcastTxCommit, error)
	broadcasted int
}

func (f *fakeRPC) ABCIQueryWithOptions(context.Context, string, []byte, rpcclient.ABCIQueryOptions) (*ctypes.ResultABCIQuery, error) {
	return &ctypes.ResultABCIQuery{}, nil // no live gas price: the fee falls back to the floor
}

func (f *fakeRPC) ABCIQuery(context.Context, string, []byte) (*ctypes.ResultABCIQuery, error) {
	if f.accountErr != nil {
		return nil, f.accountErr
	}
	acc := gnoland.GnoAccount{BaseAccount: std.BaseAccount{AccountNumber: 1, Sequence: 7}}
	return &ctypes.ResultABCIQuery{Response: abci.ResponseQuery{
		ResponseBase: abci.ResponseBase{Data: amino.MustMarshalJSON(acc)},
	}}, nil
}

func (f *fakeRPC) BroadcastTxCommit(context.Context, types.Tx) (*ctypes.ResultBroadcastTxCommit, error) {
	f.broadcasted++
	return f.broadcast()
}

func newFakeRPCDispenser(t *testing.T, rpc *fakeRPC) Dispenser {
	t.Helper()
	signer, err := gnoclient.SignerFromBip39(test1Mnemonic, "test5", "", 0, 0)
	require.NoError(t, err)
	info, err := signer.Info()
	require.NoError(t, err)
	return NewGnoclientDispenser(&gnoclient.Client{Signer: signer, RPCClient: rpc}, info.GetAddress(), 10_000_000)
}

func TestGnoclientDispenser_badRecipient(t *testing.T) {
	d := &gnoclientDispenser{} // cli nil is fine — recipient parse fails first
	_, err := d.Send(context.Background(), "not-bech32", 1_000_000)
	require.ErrorIs(t, err, ErrNotGranted)
	assert.Contains(t, err.Error(), "recipient")
}

func TestGnoclientDispenser_accountQueryFailureIsNotGranted(t *testing.T) {
	rpc := &fakeRPC{accountErr: errors.New("connection refused")}
	_, err := newFakeRPCDispenser(t, rpc).Send(context.Background(), validAddr, 1_000_000)
	require.ErrorIs(t, err, ErrNotGranted, "a failure before the broadcast leaves the recipient unpaid")
	assert.Zero(t, rpc.broadcasted)
}

func TestGnoclientDispenser_broadcastOutcomes(t *testing.T) {
	cases := map[string]struct {
		result     *ctypes.ResultBroadcastTxCommit
		err        error
		notGranted bool
	}{
		"CheckTx rejection": {
			result: &ctypes.ResultBroadcastTxCommit{CheckTx: abci.ResponseCheckTx{
				ResponseBase: abci.ResponseBase{Error: abci.StringError("insufficient funds")},
			}},
			notGranted: true,
		},
		"DeliverTx failure": {
			result: &ctypes.ResultBroadcastTxCommit{DeliverTx: abci.ResponseDeliverTx{
				ResponseBase: abci.ResponseBase{Error: abci.StringError("insufficient coins")},
			}},
			notGranted: true,
		},
		"transport error after the request": {
			err: errors.New("timed out waiting for tx to be included in a block"),
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rpc := &fakeRPC{broadcast: func() (*ctypes.ResultBroadcastTxCommit, error) { return tc.result, tc.err }}
			_, err := newFakeRPCDispenser(t, rpc).Send(context.Background(), validAddr, 1_000_000)
			require.Error(t, err)
			assert.Equal(t, tc.notGranted, errors.Is(err, ErrNotGranted))
		})
	}
}

func TestGnoclientDispenser_returnsTheTxHash(t *testing.T) {
	rpc := &fakeRPC{broadcast: func() (*ctypes.ResultBroadcastTxCommit, error) {
		return &ctypes.ResultBroadcastTxCommit{Hash: []byte{0xab, 0xcd}}, nil
	}}
	tx, err := newFakeRPCDispenser(t, rpc).Send(context.Background(), validAddr, 1_000_000)
	require.NoError(t, err)
	assert.Equal(t, "abcd", tx)
}
