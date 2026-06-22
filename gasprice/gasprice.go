// Package gasprice prices a gno transaction from the chain's live minimum gas
// price. It imports only the gno toolchain and stdlib — no gno-mcp internals —
// so both internal/chain and the standalone faucet (which keeps the same
// isolation) share one copy of the fee formula instead of drifting apart.
package gasprice

import (
	"fmt"
	"math"

	gnoclient "github.com/gnolang/gno/gno.land/pkg/gnoclient"
	"github.com/gnolang/gno/gno.land/pkg/gnoland/ugnot"
	"github.com/gnolang/gno/tm2/pkg/amino"
	"github.com/gnolang/gno/tm2/pkg/std"
)

// QueryPath is the ABCI query returning the chain's last-block gas price as an
// amino-JSON std.GasPrice. Backed by tm2/pkg/sdk/auth queryGasPrice.
const QueryPath = "auth/gasprice"

// Fetch queries the chain's live (last-block) gas price through cli. A chain that
// reports no gas price (empty/"null" response) yields the zero std.GasPrice with
// no error, so callers fall back to their floor via Compute.
func Fetch(cli *gnoclient.Client) (std.GasPrice, error) {
	qres, err := cli.Query(gnoclient.QueryCfg{Path: QueryPath})
	if err != nil {
		return std.GasPrice{}, fmt.Errorf("query %s: %w", QueryPath, err)
	}
	if len(qres.Response.Data) == 0 || string(qres.Response.Data) == "null" {
		return std.GasPrice{}, nil
	}
	price, err := decode(qres.Response.Data)
	if err != nil {
		return std.GasPrice{}, fmt.Errorf("query %s: %w", QueryPath, err)
	}
	return price, nil
}

// decode parses the amino-JSON payload returned by auth/gasprice. The chain
// marshals via amino.MarshalJSONIndent, so the gas unit is string-encoded and
// the price coin is a "<amount><denom>" string; encoding/json would not parse
// either correctly.
func decode(data []byte) (std.GasPrice, error) {
	var gp std.GasPrice
	if err := amino.UnmarshalJSON(data, &gp); err != nil {
		return std.GasPrice{}, fmt.Errorf("unmarshal gas price: %w", err)
	}
	return gp, nil
}

// Compute returns the ugnot GasFee to offer for a tx of gasWanted gas on a chain
// whose current minimum gas price is `price` (price.Price.Amount ugnot per
// price.Gas gas). The ante check requires GasFee/gasWanted >= price, so the bare
// minimum is ceil(gasWanted * amount / gas); the result is that minimum scaled by
// marginNum/marginDen, never below floor. price must be denominated in ugnot.
//
// A zero/empty price (gas unit or amount <= 0) means the dynamic gas-price gate
// is inactive — the ante skips it — so Compute returns floor. An implausibly high
// price that would overflow the int64 arithmetic also returns floor (no real
// chain reports amount > ~9e11 ugnot/gas; genesis is amount=1, gas=1000).
//
// gasWanted, marginNum, and marginDen must all be positive; a non-positive value
// is a caller error and returns an error rather than panicking on the divide.
func Compute(price std.GasPrice, gasWanted, floor, marginNum, marginDen int64) (int64, error) {
	if gasWanted <= 0 {
		return 0, fmt.Errorf("gasWanted must be positive, got %d", gasWanted)
	}
	if marginNum <= 0 || marginDen <= 0 {
		return 0, fmt.Errorf("margin must be positive, got %d/%d", marginNum, marginDen)
	}
	if price.Gas <= 0 || price.Price.Amount <= 0 {
		return floor, nil
	}
	if price.Price.Denom != ugnot.Denom {
		return 0, fmt.Errorf("gas price denom %q is not %q; cannot price a tx fee in ugnot", price.Price.Denom, ugnot.Denom)
	}
	// ceil(gasWanted * amount / gas), margin-scaled, with overflow guards on each
	// int64 multiply so an absurd node-reported price can't wrap to a garbage fee.
	if price.Price.Amount > (math.MaxInt64-(price.Gas-1))/gasWanted {
		return floor, nil
	}
	minFee := (gasWanted*price.Price.Amount + price.Gas - 1) / price.Gas
	if minFee > math.MaxInt64/marginNum {
		return floor, nil
	}
	withMargin := minFee * marginNum / marginDen
	if withMargin < floor {
		return floor, nil
	}
	return withMargin, nil
}
