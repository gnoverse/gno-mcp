package chain

import (
	"context"
	"fmt"

	gnoclient "github.com/gnolang/gno/gno.land/pkg/gnoclient"

	"github.com/gnoverse/gno-mcp/gasprice"
)

// gasFeeMarginNum / gasFeeMarginDen scale the queried minimum fee up for safety.
// The chain bills the full offered fee (not gas used), and the block gas price
// can rise between the query and block inclusion, so offering exactly the
// minimum risks an insufficient-fee rejection. 2× is cheap insurance: the
// absolute fee is tens of thousands of ugnot against multi-GNOT balances.
const (
	gasFeeMarginNum int64 = 2
	gasFeeMarginDen int64 = 1
)

// currentGasFee queries the chain's live gas price and returns the ugnot GasFee
// to offer on a DefaultGasWanted-sized tx, margin-scaled and floored at
// DefaultGasFeeUgnot. Used by every broadcast path so a write adapts to the
// chain's actual (and possibly congestion-raised) minimum instead of a pinned
// constant.
func (r *Real) currentGasFee() (int64, error) {
	price, err := gasprice.Fetch(r.cli)
	if err != nil {
		return 0, err
	}
	return gasprice.Compute(price, DefaultGasWanted, DefaultGasFeeUgnot, gasFeeMarginNum, gasFeeMarginDen)
}

// GasFeeUgnot returns the ugnot GasFee gnomcp would offer on a write against this
// chain right now (see currentGasFee). Callers that must reserve the fee before a
// send — e.g. sweeping a key's full balance minus gas — use it to leave exactly
// enough behind.
func (r *Real) GasFeeUgnot(_ context.Context) (int64, error) {
	return r.currentGasFee()
}

// feeForTx is the GasFee to offer for a write. A simulation pays no fee and the
// offered value does not affect the gas estimate, so it uses the floor and skips
// the network round-trip; a broadcast queries the chain's live gas price.
func (r *Real) feeForTx(simulate bool) (int64, error) {
	if simulate {
		return DefaultGasFeeUgnot, nil
	}
	return r.currentGasFee()
}

// baseTxCfg builds the gnoclient tx config for an offered fee of feeUgnot.
func baseTxCfg(feeUgnot int64) gnoclient.BaseTxCfg {
	return gnoclient.BaseTxCfg{
		GasFee:    fmt.Sprintf("%dugnot", feeUgnot),
		GasWanted: DefaultGasWanted,
	}
}
