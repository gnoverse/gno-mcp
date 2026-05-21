package profiles

import (
	"fmt"
	"os"
	"regexp"
	"time"
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
// Returns the first encountered error.
func (c *Config) Validate() error {
	if len(c.Profiles) == 0 {
		return fmt.Errorf("no profiles loaded")
	}
	for name, p := range c.Profiles {
		if p.RPCURL == "" {
			return fmt.Errorf("profile %q: missing required rpc-url", name)
		}
		if p.ChainID == "" {
			return fmt.Errorf("profile %q: missing required chain-id", name)
		}
		if p.ChainType == "" {
			p.ChainType = ChainTypeTestnet
		}
		if !validChainTypes[p.ChainType] {
			return fmt.Errorf("profile %q: unknown chain-type %q (must be local/testnet/mainnet)", name, p.ChainType)
		}

		if p.DefaultExpiresIn != "" {
			if _, err := time.ParseDuration(p.DefaultExpiresIn); err != nil {
				return fmt.Errorf("profile %q: invalid default-expires-in %q: %w", name, p.DefaultExpiresIn, err)
			}
		}
		if p.DefaultSpendLimit != "" {
			if !spendLimitRE.MatchString(p.DefaultSpendLimit) {
				return fmt.Errorf("profile %q: invalid default-spend-limit %q (expected like \"1000ugnot\")", name, p.DefaultSpendLimit)
			}
		}
		if p.BypassHardLimits && !p.AllowDangerousTools {
			return fmt.Errorf("profile %q: bypass-hard-limits requires allow-dangerous-tools=true (bypass is meaningless without write tools)", name)
		}

		c.Profiles[name] = p

		if p.ChainType == ChainTypeMainnet && p.AllowDangerousTools {
			fmt.Fprintf(os.Stderr, "WARNING: Profile %q: mainnet with allow-dangerous-tools=true. Real funds at stake.\n", name)
		}
	}
	return nil
}
