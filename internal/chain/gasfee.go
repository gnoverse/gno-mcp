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

// gasEstimateCeiling is the GasWanted used for the dry-run that measures a tx's
// gas before a broadcast. A simulation pays no fee and is an ABCI query — not
// metered against the block gas limit — so this sits far above any realistic tx:
// high enough to measure a heavy write (e.g. a CLA Sign on a large AVL set) that
// would exceed DefaultGasWanted and out-of-gas under the broadcast limit.
//
// Chain contract this relies on: simulate does NOT enforce the mempool's
// fee ≥ gasWanted × price minimum, so the measuring leg can offer the floor
// fee at this ceiling. Every integration write test exercises that leg
// against the in-process node, so a gno release that starts min-fee-checking
// simulations fails the suite immediately.
const gasEstimateCeiling int64 = 1_000_000_000

// gasWantedMarginNum / gasWantedMarginDen scale measured gas up for execution
// headroom: deliver-time gas can slightly exceed the simulated estimate, so a
// broadcast offers 50% more than the dry-run measured.
const (
	gasWantedMarginNum int64 = 3
	gasWantedMarginDen int64 = 2
)

// gasWantedFor sizes a broadcast's GasWanted from the gas a dry-run measured:
// measured × margin, floored at DefaultGasWanted (light txs share one stable
// limit) and capped at gasEstimateCeiling.
func gasWantedFor(measured int64) int64 {
	want := measured * gasWantedMarginNum / gasWantedMarginDen
	if want < DefaultGasWanted {
		return DefaultGasWanted
	}
	if want > gasEstimateCeiling {
		return gasEstimateCeiling
	}
	return want
}

// currentGasFee queries the chain's live gas price and returns the ugnot GasFee
// to offer on a gasWanted-sized tx, margin-scaled and floored at
// DefaultGasFeeUgnot. Used by every broadcast path so a write adapts to the
// chain's actual (and possibly congestion-raised) minimum instead of a pinned
// constant. A larger gasWanted (a right-sized heavy tx) raises the offered fee
// proportionally, since the chain's minimum fee scales with the gas reserved.
func (r *Real) currentGasFee(gasWanted int64) (int64, error) {
	price, err := gasprice.Fetch(r.cli)
	if err != nil {
		return 0, err
	}
	return gasprice.Compute(price, gasWanted, DefaultGasFeeUgnot, gasFeeMarginNum, gasFeeMarginDen)
}

// GasFeeUgnot returns the ugnot GasFee gnomcp would offer on a write against this
// chain right now (see currentGasFee). Callers that must reserve the fee before a
// send — e.g. sweeping a key's full balance minus gas — use it to leave exactly
// enough behind.
func (r *Real) GasFeeUgnot(_ context.Context) (int64, error) {
	return r.currentGasFee(DefaultGasWanted)
}

// baseTxCfg builds the gnoclient tx config for an offered fee of feeUgnot and a
// gas limit of gasWanted.
func baseTxCfg(feeUgnot, gasWanted int64) gnoclient.BaseTxCfg {
	return gnoclient.BaseTxCfg{
		GasFee:    fmt.Sprintf("%dugnot", feeUgnot),
		GasWanted: gasWanted,
	}
}
