package faucet

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/gnolang/gno/tm2/pkg/crypto"
)

var (
	ErrChainRefused  = errors.New("faucet: chain-id is not a testnet")
	ErrChainMismatch = errors.New("faucet: chain-id does not match this faucet")
	ErrBadAddress    = errors.New("faucet: invalid recipient address")

	// testnet only; dev (local) and everything else are refused.
	testChainRE = regexp.MustCompile(`^test-?\d+$`)
)

// Faucet dispenses a fixed ugnot grant to testnet addresses, bounded by a Limiter.
type Faucet struct {
	chainID    string
	grantUgnot int64
	dispenser  Dispenser
	limiter    *Limiter
}

func New(chainID string, grantUgnot int64, d Dispenser, l *Limiter) *Faucet {
	return &Faucet{chainID: chainID, grantUgnot: grantUgnot, dispenser: d, limiter: l}
}

// IsTestnetChainID reports whether id matches the testnet chain-id pattern (test-?N).
func IsTestnetChainID(id string) bool { return testChainRE.MatchString(id) }

// Fund validates the chain-id and recipient, applies rate-limits/cap, then
// dispenses the grant. Check order: chain-id is-testnet -> chain-id matches this
// faucet -> recipient is valid -> rate-limit -> dispense. The recipient is parsed
// before the limiter is touched so a garbage address cannot burn the daily cap,
// and the limiter is keyed on the canonical address so case variants share one
// bucket. A dispense failure refunds the limiter so a chain hiccup doesn't
// consume the requester's cooldown or the global budget.
func (f *Faucet) Fund(ctx context.Context, address, ip, reqChainID string) (string, error) {
	if !IsTestnetChainID(reqChainID) {
		return "", ErrChainRefused
	}
	if reqChainID != f.chainID {
		return "", ErrChainMismatch
	}
	addr, err := crypto.AddressFromBech32(address)
	if err != nil {
		return "", fmt.Errorf("%w: %q", ErrBadAddress, address)
	}
	canonical := addr.String()

	grantedAt, err := f.limiter.Allow(canonical, ip)
	if err != nil {
		return "", err
	}
	tx, err := f.dispenser.Send(ctx, canonical, f.grantUgnot)
	if err != nil {
		f.limiter.Refund(canonical, ip, grantedAt)
		return "", fmt.Errorf("faucet: dispense: %w", err)
	}
	return tx, nil
}
