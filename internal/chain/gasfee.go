package chain

import (
	"context"
	"fmt"
	"math"

	gnoclient "github.com/gnolang/gno/gno.land/pkg/gnoclient"
	"github.com/gnolang/gno/gno.land/pkg/gnoland/ugnot"
	"github.com/gnolang/gno/tm2/pkg/amino"
	"github.com/gnolang/gno/tm2/pkg/std"
)

// gasPriceQueryPath is the ABCI query that returns the chain's last-block gas
// price as an amino-JSON std.GasPrice. Backed by tm2/pkg/sdk/auth queryGasPrice.
const gasPriceQueryPath = "auth/gasprice"

// gasFeeMarginNum / gasFeeMarginDen scale the queried minimum fee up for safety.
// The chain bills the full offered fee (not gas used), and the block gas price
// can rise between the query and block inclusion, so offering exactly the
// minimum risks an insufficient-fee rejection. 2× is cheap insurance: the
// absolute fee is tens of thousands of ugnot against multi-GNOT balances.
const (
	gasFeeMarginNum int64 = 2
	gasFeeMarginDen int64 = 1
)

// computeGasFee returns the ugnot GasFee to offer for a tx of gasWanted gas on a
// chain whose current minimum gas price is `price` (price.Price.Amount ugnot per
// price.Gas gas). The ante check requires GasFee/gasWanted >= price, so the bare
// minimum is ceil(gasWanted * amount / gas); the result is that minimum scaled by
// the safety margin, never below floor.
//
// A zero/empty on-chain price (gas unit or amount <= 0) means the dynamic gas
// price gate is inactive — the ante skips it — so we fall back to floor, the
// genesis-derived minimum. price must be denominated in ugnot.
func computeGasFee(price std.GasPrice, gasWanted, floor int64) (int64, error) {
	if price.Gas <= 0 || price.Price.Amount <= 0 {
		return floor, nil
	}
	if price.Price.Denom != ugnot.Denom {
		return 0, fmt.Errorf("gas price denom %q is not %q; cannot price a tx fee in ugnot", price.Price.Denom, ugnot.Denom)
	}
	// ceil(gasWanted * amount / gas), margin-scaled. The fee is decoded from the
	// node's response, so guard each int64 multiply: an absurd price (no real
	// chain reports amount > ~9e11 ugnot/gas; genesis is amount=1, gas=1000) would
	// overflow and wrap to a garbage positive fee. On overflow fall back to the
	// floor — the same "no usable price" handling as the zero-price case above.
	if price.Price.Amount > (math.MaxInt64-(price.Gas-1))/gasWanted {
		return floor, nil
	}
	minFee := (gasWanted*price.Price.Amount + price.Gas - 1) / price.Gas
	if minFee > math.MaxInt64/gasFeeMarginNum {
		return floor, nil
	}
	withMargin := minFee * gasFeeMarginNum / gasFeeMarginDen
	if withMargin < floor {
		return floor, nil
	}
	return withMargin, nil
}

// decodeGasPrice parses the amino-JSON payload returned by auth/gasprice into a
// std.GasPrice. The chain marshals via amino.MarshalJSONIndent, so the gas unit
// is string-encoded and the price coin is a "<amount><denom>" string;
// encoding/json would not parse either correctly.
func decodeGasPrice(data []byte) (std.GasPrice, error) {
	var gp std.GasPrice
	if err := amino.UnmarshalJSON(data, &gp); err != nil {
		return std.GasPrice{}, fmt.Errorf("unmarshal gas price: %w", err)
	}
	return gp, nil
}

// currentGasFee queries the chain's live gas price and returns the ugnot GasFee
// to offer on a DefaultGasWanted-sized tx, margin-scaled and floored at
// DefaultGasFeeUgnot. Used by every broadcast path so a write adapts to the
// chain's actual (and possibly congestion-raised) minimum instead of a pinned
// constant.
func (r *Real) currentGasFee() (int64, error) {
	qres, err := r.cli.Query(gnoclient.QueryCfg{Path: gasPriceQueryPath})
	if err != nil {
		return 0, fmt.Errorf("query %s: %w", gasPriceQueryPath, err)
	}
	if len(qres.Response.Data) == 0 || string(qres.Response.Data) == "null" {
		// No gas price reported: fall back to the floor.
		return DefaultGasFeeUgnot, nil
	}
	price, err := decodeGasPrice(qres.Response.Data)
	if err != nil {
		return 0, fmt.Errorf("query %s: %w", gasPriceQueryPath, err)
	}
	return computeGasFee(price, DefaultGasWanted, DefaultGasFeeUgnot)
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
