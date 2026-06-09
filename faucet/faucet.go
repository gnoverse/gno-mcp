package faucet

import (
	"context"
	"errors"
	"fmt"
	"regexp"
)

var (
	ErrChainRefused  = errors.New("faucet: chain-id is not a testnet")
	ErrChainMismatch = errors.New("faucet: chain-id does not match this faucet")

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

// Fund validates the chain-id, applies rate-limits/cap, then dispenses the grant.
// Check order: chain-id is-testnet -> chain-id matches this faucet -> rate-limit -> dispense.
func (f *Faucet) Fund(ctx context.Context, address, ip, reqChainID string) (string, error) {
	if !IsTestnetChainID(reqChainID) {
		return "", ErrChainRefused
	}
	if reqChainID != f.chainID {
		return "", ErrChainMismatch
	}
	if err := f.limiter.Allow(address, ip); err != nil {
		return "", err
	}
	tx, err := f.dispenser.Send(ctx, address, f.grantUgnot)
	if err != nil {
		return "", fmt.Errorf("faucet: dispense: %w", err)
	}
	return tx, nil
}
