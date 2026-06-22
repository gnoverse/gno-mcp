package profiles

import (
	"fmt"
	"time"
)

// HardLimits is the per-profile clamp ceiling derived from the chain-id (local dev vs testnet).
// Zero-valued fields signal "unlimited" (only set when BypassHardLimits=true).
type HardLimits struct {
	MaxSpendLimit      string        // e.g. "100000000ugnot"; "" = unlimited
	MaxExpiresIn       time.Duration // 0 = unlimited
	MaxAllowPathsCount int           // 0 = unlimited
}

const (
	// hardDefaultSpendLimit must comfortably exceed the per-call cost the session
	// ante charges against the spend-limit before execution: the tx GasFee (the
	// chain's live gas price, floored at chain.DefaultGasFeeUgnot ~10K ugnot) plus
	// any coins the call sends — see auth.ante Phase 2a/2b. At 100M a default
	// session covers many fee-only calls with ample headroom for sends. Only
	// dev/test chains are allowed, so the spend-limit guards testnet funds only.
	hardDefaultSpendLimit = "100000000ugnot"
	hardDefaultExpiresIn  = time.Hour
)

// EffectiveDefaults returns the scope defaults that apply when the agent
// omits values in gno_session_propose. Implements layers 2→3 of the scope
// policy: profile override falls back to hardcoded. The returned values are
// NOT clamped to HardLimits — callers apply clamping separately.
func (p Profile) EffectiveDefaults() (spendLimit string, expiresIn time.Duration, err error) {
	spendLimit = p.DefaultSpendLimit
	if spendLimit == "" {
		spendLimit = hardDefaultSpendLimit
	}
	if p.DefaultExpiresIn == "" {
		expiresIn = hardDefaultExpiresIn
	} else {
		expiresIn, err = time.ParseDuration(p.DefaultExpiresIn)
		if err != nil {
			return "", 0, fmt.Errorf("invalid default-expires-in %q: %w", p.DefaultExpiresIn, err)
		}
	}
	return spendLimit, expiresIn, nil
}

// HardLimits returns the clamp ceiling for the profile.
// BypassHardLimits=true returns all-zero sentinel values (unlimited).
func (p Profile) HardLimits() HardLimits {
	if p.BypassHardLimits {
		return HardLimits{}
	}
	if p.IsLocal() {
		return HardLimits{
			MaxSpendLimit:      "100000000ugnot",
			MaxExpiresIn:       30 * 24 * time.Hour,
			MaxAllowPathsCount: 20,
		}
	}
	return HardLimits{
		MaxSpendLimit:      "100000000ugnot",
		MaxExpiresIn:       7 * 24 * time.Hour,
		MaxAllowPathsCount: 10,
	}
}
