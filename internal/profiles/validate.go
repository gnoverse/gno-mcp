package profiles

import (
	"fmt"
	"regexp"
	"time"

	"github.com/gnolang/gno/tm2/pkg/crypto"
)

// Valid chain types.
const (
	ChainTypeLocal   = "local"
	ChainTypeTestnet = "testnet"
	ChainTypeMainnet = "mainnet"
)

var (
	validChainTypes = map[string]bool{
		ChainTypeLocal:   true,
		ChainTypeTestnet: true,
		ChainTypeMainnet: true,
	}

	// spendLimitRE matches a single-denomination coin amount: digits followed by
	// lowercase letters (e.g. "1000000ugnot", "10gnot"). Cross-denom mixes rejected.
	spendLimitRE = regexp.MustCompile(`^[0-9]+[a-z]+$`)
)

// Validate checks required fields and applies defaults in place.
// The returned warning is non-nil when a valid but potentially dangerous
// configuration is detected (e.g. mainnet with allow-dangerous-tools=true).
// The caller decides how to surface the warning (log, stderr, ignore).
func (c *Config) Validate() (warn error, err error) {
	if len(c.Profiles) == 0 {
		return nil, fmt.Errorf("no profiles loaded")
	}
	for name, p := range c.Profiles {
		if p.RPCURL == "" {
			return nil, fmt.Errorf("profile %q: missing required rpc-url", name)
		}
		if p.ChainID == "" {
			return nil, fmt.Errorf("profile %q: missing required chain-id", name)
		}
		if p.ChainType == "" {
			p.ChainType = ChainTypeTestnet
		}
		if !validChainTypes[p.ChainType] {
			return nil, fmt.Errorf("profile %q: unknown chain-type %q (must be local/testnet/mainnet)", name, p.ChainType)
		}

		if p.DefaultExpiresIn != "" {
			if _, err := time.ParseDuration(p.DefaultExpiresIn); err != nil {
				return nil, fmt.Errorf("profile %q: invalid default-expires-in %q: %w", name, p.DefaultExpiresIn, err)
			}
		}
		if p.DefaultSpendLimit != "" {
			if !spendLimitRE.MatchString(p.DefaultSpendLimit) {
				return nil, fmt.Errorf("profile %q: invalid default-spend-limit %q (expected like \"1000ugnot\")", name, p.DefaultSpendLimit)
			}
		}
		if p.BypassHardLimits && !p.AllowDangerousTools {
			return nil, fmt.Errorf("profile %q: bypass-hard-limits requires allow-dangerous-tools=true (bypass is meaningless without write tools)", name)
		}
		if p.MasterAddress != "" {
			if _, err := crypto.AddressFromBech32(p.MasterAddress); err != nil {
				return nil, fmt.Errorf("profile %q: invalid master-address %q: %w", name, p.MasterAddress, err)
			}
		} else if p.AllowDangerousTools {
			return nil, fmt.Errorf("profile %q: master-address is required when allow-dangerous-tools=true", name)
		}

		c.Profiles[name] = p

		if p.ChainType == ChainTypeMainnet && p.AllowDangerousTools {
			warn = fmt.Errorf("profile %q: mainnet with allow-dangerous-tools=true. Real funds at stake", name)
		}
	}
	return warn, nil
}
