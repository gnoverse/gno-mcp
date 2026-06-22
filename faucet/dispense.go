// Package faucet is a standalone automatic testnet faucet service. It imports
// only the gno toolchain, stdlib, and the toolchain-only gno-mcp/gasprice helper
// (no gno-mcp internals) — so it can be extracted to its own repo later, taking
// gasprice with it.
package faucet

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	gnoclient "github.com/gnolang/gno/gno.land/pkg/gnoclient"
	"github.com/gnolang/gno/gno.land/pkg/gnoland/ugnot"
	"github.com/gnolang/gno/tm2/pkg/crypto"
	"github.com/gnolang/gno/tm2/pkg/sdk/bank"
	"github.com/gnolang/gno/tm2/pkg/std"

	"github.com/gnoverse/gno-mcp/gasprice"
)

// Dispenser sends a ugnot grant to an address and returns the tx hash. This
// interface is the seam: gnoclientDispenser uses bank.MsgSend today; a realm
// dispenser (gnoclient.Call) can replace it later without touching the service.
type Dispenser interface {
	Send(ctx context.Context, to string, amountUgnot int64) (txHash string, err error)
}

// Gas-fee policy for a dispense. The fee is priced from the chain's live gas
// price (gasprice.Compute) rather than a fixed amount, so the faucet neither
// overpays at the chain minimum nor gets rejected when the price rises.
const (
	// dispenseGasFeeMarginNum / dispenseGasFeeMarginDen scale the queried minimum
	// fee for safety; the chain bills the full offered fee and the price can rise
	// before inclusion.
	dispenseGasFeeMarginNum int64 = 2
	dispenseGasFeeMarginDen int64 = 1
	// genesisGasPriceDivisor is the gno genesis min gas price as gas-per-ugnot
	// (1 ugnot per 1000 gas); it sets the fee floor used when the chain reports no
	// live price.
	genesisGasPriceDivisor int64 = 1000
)

type gnoclientDispenser struct {
	mu        sync.Mutex // serialises sends so concurrent grants don't race the account sequence
	cli       *gnoclient.Client
	from      crypto.Address
	gasWanted int64 // execution ceiling; a test-13 bank send burns ~1.6M
	floor     int64 // ugnot fee floor at the genesis price (gasWanted / genesisGasPriceDivisor)
}

// NewGnoclientDispenser builds the production Dispenser: a bank.MsgSend sender
// signing with the funding key behind cli. gasWanted is the execution ceiling;
// the GasFee is priced per send from the chain's live gas price.
func NewGnoclientDispenser(cli *gnoclient.Client, from crypto.Address, gasWanted int64) Dispenser {
	return &gnoclientDispenser{
		cli:       cli,
		from:      from,
		gasWanted: gasWanted,
		floor:     gasWanted / genesisGasPriceDivisor,
	}
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

	price, err := gasprice.Fetch(d.cli)
	if err != nil {
		return "", fmt.Errorf("faucet: gas price: %w", err)
	}
	fee, err := gasprice.Compute(price, d.gasWanted, d.floor, dispenseGasFeeMarginNum, dispenseGasFeeMarginDen)
	if err != nil {
		return "", fmt.Errorf("faucet: gas price: %w", err)
	}

	msg := bank.MsgSend{
		FromAddress: d.from,
		ToAddress:   toAddr,
		Amount:      std.Coins{{Denom: ugnot.Denom, Amount: amountUgnot}},
	}
	res, err := d.cli.Send(gnoclient.BaseTxCfg{GasFee: fmt.Sprintf("%dugnot", fee), GasWanted: d.gasWanted}, msg)
	if err != nil {
		return "", fmt.Errorf("faucet: send: %w", err)
	}
	return hex.EncodeToString(res.Hash), nil
}

// NewGnoclientBalance returns a funding-balance source for WithBalanceFloor: it
// queries addr's ugnot balance through cli, caching the result for ttl so a busy
// faucet does not issue an account query per fund request. The funding key is
// always a funded account, so a QueryAccount error is treated as a transport
// failure and propagated (the caller surfaces it) rather than masked as a zero
// balance.
func NewGnoclientBalance(cli *gnoclient.Client, addr crypto.Address, ttl time.Duration) func(context.Context) (int64, error) {
	var (
		mu       sync.Mutex
		cached   int64
		cachedAt time.Time
		seeded   bool
	)
	return func(context.Context) (int64, error) {
		mu.Lock()
		defer mu.Unlock()
		if seeded && time.Since(cachedAt) < ttl {
			return cached, nil
		}
		acc, _, err := cli.QueryAccount(addr)
		if err != nil {
			return 0, fmt.Errorf("query funding account: %w", err)
		}
		cached = acc.Coins.AmountOf(ugnot.Denom)
		cachedAt, seeded = time.Now(), true
		return cached, nil
	}
}
