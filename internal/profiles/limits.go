package profiles

import (
	"fmt"
	"time"
)

// HardLimits is the per-profile clamp ceiling derived from chain-type.
// Zero-valued fields signal "unlimited" (only set when BypassHardLimits=true).
type HardLimits struct {
	MaxSpendLimit      string        // e.g. "100000000ugnot"; "" = unlimited
	MaxExpiresIn       time.Duration // 0 = unlimited
	MaxAllowPathsCount int           // 0 = unlimited
}

const (
	// hardDefaultSpendLimit must comfortably exceed the per-call gas-fee
	// reservation (the chain's session ante reserves the full tx GasFee, 10M
	// ugnot here, against the spend-limit before execution — see auth.ante
	// Phase 2a). At 100M a default session covers ~10 calls. Only dev/test
	// chains are allowed, so the spend-limit guards testnet funds only.
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
// Unknown chain-types default to testnet limits (safe middle).
func (p Profile) HardLimits() HardLimits {
	if p.BypassHardLimits {
		return HardLimits{}
	}
	switch p.ChainType {
	case ChainTypeLocal:
		return HardLimits{
			MaxSpendLimit:      "100000000ugnot",
			MaxExpiresIn:       30 * 24 * time.Hour,
			MaxAllowPathsCount: 20,
		}
	default:
		return HardLimits{
			MaxSpendLimit:      "100000000ugnot",
			MaxExpiresIn:       7 * 24 * time.Hour,
			MaxAllowPathsCount: 10,
		}
	}
}
