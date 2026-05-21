package profiles

import "time"

// HardLimits is the per-profile clamp ceiling derived from chain-type.
// Zero-valued fields signal "unlimited" (only set when BypassHardLimits=true).
type HardLimits struct {
	MaxSpendLimit      string        // e.g. "100000000ugnot"; "" = unlimited
	MaxExpiresIn       time.Duration // 0 = unlimited
	MaxAllowPathsCount int           // 0 = unlimited
}

// HardLimits returns the clamp ceiling for the profile.
// BypassHardLimits=true returns all-zero sentinel values (unlimited).
// Unknown chain-types default to testnet limits (safe middle).
func (p *Profile) HardLimits() HardLimits {
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
	case ChainTypeMainnet:
		return HardLimits{
			MaxSpendLimit:      "1000ugnot",
			MaxExpiresIn:       time.Hour,
			MaxAllowPathsCount: 3,
		}
	default:
		return HardLimits{
			MaxSpendLimit:      "10000000ugnot",
			MaxExpiresIn:       7 * 24 * time.Hour,
			MaxAllowPathsCount: 10,
		}
	}
}
