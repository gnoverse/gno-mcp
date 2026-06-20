package faucet

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"

	"github.com/gnolang/gno/tm2/pkg/crypto"
)

var (
	ErrChainRefused  = errors.New("faucet: chain-id is not a testnet")
	ErrChainMismatch = errors.New("faucet: chain-id does not match this faucet")
	ErrBadAddress    = errors.New("faucet: invalid recipient address")
	ErrFundingLow    = errors.New("faucet: funding wallet below minimum balance")

	// testnet only; dev (local) and everything else are refused.
	testChainRE = regexp.MustCompile(`^test-?\d+$`)
)

// Faucet dispenses a fixed ugnot grant to testnet addresses, bounded by a Limiter.
type Faucet struct {
	chainID    string
	grantUgnot int64
	dispenser  Dispenser
	limiter    *Limiter

	// Balance-aware throttle (optional). When minFundingUgnot > 0 and
	// fundingBalance is set, Fund refuses with ErrFundingLow once the funding
	// wallet drops below the floor, so the faucet degrades gracefully rather than
	// failing mid-dispense on an empty key. The floor is best-effort: the balance
	// read and the dispense are not atomic (and the balance is TTL-cached), so
	// concurrent in-flight grants can undershoot it by up to their combined size.
	// It is a graceful-degradation guard, not a hard solvency invariant.
	minFundingUgnot int64
	fundingBalance  func(context.Context) (int64, error)

	logger         *slog.Logger
	metrics        Metrics
	trustedProxies int
}

// Option configures optional Faucet behavior.
type Option func(*Faucet)

// WithBalanceFloor refuses grants (ErrFundingLow) while the funding wallet's
// balance is below minUgnot. balance reports the funding wallet's current ugnot;
// it should cache internally to avoid an RPC per request.
func WithBalanceFloor(minUgnot int64, balance func(context.Context) (int64, error)) Option {
	return func(f *Faucet) {
		f.minFundingUgnot = minUgnot
		f.fundingBalance = balance
	}
}

// WithLogger sets the structured logger used for the per-request access log and
// internal-error logging. Defaults to slog.Default().
func WithLogger(l *slog.Logger) Option { return func(f *Faucet) { f.logger = l } }

// WithMetrics sets the outcome metrics recorder. Defaults to a no-op.
func WithMetrics(m Metrics) Option { return func(f *Faucet) { f.metrics = m } }

// WithTrustedProxies sets how many reverse-proxy hops in front of the faucet are
// trusted to append honest X-Forwarded-For entries (e.g. 1 for a single ALB). 0
// means use the direct peer address. See clientIP.
func WithTrustedProxies(n int) Option { return func(f *Faucet) { f.trustedProxies = n } }

func New(chainID string, grantUgnot int64, d Dispenser, l *Limiter, opts ...Option) *Faucet {
	f := &Faucet{
		chainID: chainID, grantUgnot: grantUgnot, dispenser: d, limiter: l,
		logger: slog.Default(), metrics: nopMetrics{},
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// IsTestnetChainID reports whether id matches the testnet chain-id pattern (test-?N).
func IsTestnetChainID(id string) bool { return testChainRE.MatchString(id) }

// FaucetLimits is the public, policy-only view served at GET /limits. It
// deliberately omits per-IP, the daily cap, the drip bucket, and all live
// remaining state — those are anti-abuse internals or metrics-only.
type FaucetLimits struct {
	GrantUgnot int64           `json:"grant_ugnot"`
	PerAddress PerAddressLimit `json:"per_address"`
}

// PerAddressLimit is the only rate limit safe to disclose: a fresh key bypasses
// it, so naming it defeats no anti-abuse control.
type PerAddressLimit struct {
	Max           int `json:"max"`
	WindowSeconds int `json:"window_seconds"`
}

// Limits returns the published per-address policy for this faucet.
func (f *Faucet) Limits() FaucetLimits {
	p := f.limiter.Policy()
	return FaucetLimits{
		GrantUgnot: f.grantUgnot,
		PerAddress: PerAddressLimit{Max: p.PerAddrMax, WindowSeconds: int(p.PerAddrWindow.Seconds())},
	}
}

// retryAfterSeconds is the coarse, static back-off advertised on every 429:
// floored at 24h, never the exact remaining time, so it leaks no timing oracle.
func (f *Faucet) retryAfterSeconds() int {
	const floor = 24 * 60 * 60
	if w := int(f.limiter.Policy().PerAddrWindow.Seconds()); w > floor {
		return w
	}
	return floor
}

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

	// Funding-balance floor is a global precondition: if the faucet is broke,
	// refuse fast and never touch the limiter (so a top-up restores service
	// without having burnt anyone's cooldown).
	if f.fundingBalance != nil && f.minFundingUgnot > 0 {
		bal, err := f.fundingBalance(ctx)
		if err != nil {
			return "", fmt.Errorf("faucet: funding balance: %w", err)
		}
		if bal < f.minFundingUgnot {
			return "", ErrFundingLow
		}
	}

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
