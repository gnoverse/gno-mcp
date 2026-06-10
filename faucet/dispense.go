// Package faucet is a standalone automatic testnet faucet service. It imports
// only the gno toolchain and stdlib — no gno-mcp internals — so it can be
// extracted to its own repo later.
package faucet

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"

	gnoclient "github.com/gnolang/gno/gno.land/pkg/gnoclient"
	"github.com/gnolang/gno/gno.land/pkg/gnoland/ugnot"
	"github.com/gnolang/gno/tm2/pkg/crypto"
	"github.com/gnolang/gno/tm2/pkg/sdk/bank"
	"github.com/gnolang/gno/tm2/pkg/std"
)

// Dispenser sends a ugnot grant to an address and returns the tx hash. This
// interface is the seam: gnoclientDispenser uses bank.MsgSend today; a realm
// dispenser (gnoclient.Call) can replace it later without touching the service.
type Dispenser interface {
	Send(ctx context.Context, to string, amountUgnot int64) (txHash string, err error)
}

type gnoclientDispenser struct {
	mu        sync.Mutex // serialises sends so concurrent grants don't race the account sequence
	cli       *gnoclient.Client
	from      crypto.Address
	gasFee    string // e.g. "10000000ugnot"
	gasWanted int64  // e.g. 10_000_000
}

// NewGnoclientDispenser builds the production Dispenser: a bank.MsgSend sender
// signing with the funding key behind cli, paying the given gas.
func NewGnoclientDispenser(cli *gnoclient.Client, from crypto.Address, gasFee string, gasWanted int64) Dispenser {
	return &gnoclientDispenser{cli: cli, from: from, gasFee: gasFee, gasWanted: gasWanted}
}

// Send ignores ctx: gnoclient has no context support (same limitation as chain.Real).
// Sends are serialised: gnoclient queries the account sequence at sign time, so
// two concurrent sends would sign with the same sequence and one would fail.
func (d *gnoclientDispenser) Send(_ context.Context, to string, amountUgnot int64) (string, error) {
	toAddr, err := crypto.AddressFromBech32(to)
	if err != nil {
		return "", fmt.Errorf("faucet: bad recipient %q: %w", to, err)
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	msg := bank.MsgSend{
		FromAddress: d.from,
		ToAddress:   toAddr,
		Amount:      std.Coins{{Denom: ugnot.Denom, Amount: amountUgnot}},
	}
	res, err := d.cli.Send(gnoclient.BaseTxCfg{GasFee: d.gasFee, GasWanted: d.gasWanted}, msg)
	if err != nil {
		return "", fmt.Errorf("faucet: send: %w", err)
	}
	return hex.EncodeToString(res.Hash), nil
}
