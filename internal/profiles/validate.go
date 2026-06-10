package profiles

import (
	"fmt"
	"net/url"
	"regexp"
	"time"

	"github.com/gnolang/gno/tm2/pkg/crypto"
)

// Valid chain types.
const (
	ChainTypeLocal   = "local"
	ChainTypeTestnet = "testnet"
)

var (
	validChainTypes = map[string]bool{
		ChainTypeLocal:   true,
		ChainTypeTestnet: true,
	}

	// spendLimitRE matches a single-denomination coin amount: digits followed by
	// lowercase letters (e.g. "1000000ugnot", "10gnot"). Cross-denom mixes rejected.
	spendLimitRE = regexp.MustCompile(`^[0-9]+[a-z]+$`)

	// chainIDRE is the allowlist: only local dev and numbered testnets.
	// Admits "test11" and the hyphenated "test-13" form. Betanet ("gnoland1"),
	// "staging", and arbitrary ids are rejected — they cannot enter the config.
	chainIDRE = regexp.MustCompile(`^(dev|test-?\d+)$`)

	// profileNameRE matches a safe profile identifier: lowercase alphanumeric,
	// with internal '-'/'_'. It excludes whitespace and shell metacharacters so
	// a name can be interpolated into the gnomcp command gno_connect prints.
	profileNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

	// rpcURLRE matches an http(s) URL safe to paste into a shell command:
	// scheme, host[:port], and an optional simple path, with no shell
	// metacharacters or whitespace. A url.Parse alone is insufficient because it
	// accepts a metacharacter-bearing path like "http://h/$(cmd)".
	rpcURLRE = regexp.MustCompile(`^https?://[A-Za-z0-9.-]+(:[0-9]+)?(/[A-Za-z0-9._/-]*)?$`)
)

// SpendLimitValid reports whether s is a well-formed single-denomination coin
// amount (digits then lowercase letters, e.g. "1000000ugnot").
func SpendLimitValid(s string) bool {
	return spendLimitRE.MatchString(s)
}

// ValidProfileName reports whether name is a safe profile identifier.
func ValidProfileName(name string) bool {
	return profileNameRE.MatchString(name)
}

// ValidRPCURL reports whether s is an http(s) URL containing only characters
// safe to interpolate into a pasted shell command.
func ValidRPCURL(s string) bool {
	return rpcURLRE.MatchString(s)
}

// ChainIDAllowed reports whether a chain-id is permitted (local dev or a
// numbered testnet). Betanet/mainnet/staging are rejected.
func ChainIDAllowed(chainID string) bool {
	return chainIDRE.MatchString(chainID)
}

// Validate checks required fields and applies defaults in place.
// The returned warning is non-nil when a valid but potentially dangerous
// configuration is detected. The caller decides how to surface the warning
// (log, stderr, ignore).
func (c *Config) Validate() (warn error, err error) {
	if len(c.Profiles) == 0 {
		return nil, fmt.Errorf("no profiles loaded")
	}
	for name, p := range c.Profiles {
		if p.RPCURL == "" {
			return nil, fmt.Errorf("profile %q: missing required rpc-url", name)
		}
		if !ValidRPCURL(p.RPCURL) {
			return nil, fmt.Errorf("profile %q: invalid rpc-url %q (want an absolute http(s) URL with no spaces or shell metacharacters — it is interpolated into pasted gnokey commands)", name, p.RPCURL)
		}
		if p.ChainID == "" {
			return nil, fmt.Errorf("profile %q: missing required chain-id", name)
		}
		if !ChainIDAllowed(p.ChainID) {
			return nil, fmt.Errorf("profile %q: chain-id %q is not allowed (only dev or testNN are permitted; betanet/mainnet/staging are forbidden)", name, p.ChainID)
		}
		if p.ChainType == "" {
			if p.ChainID == "dev" {
				p.ChainType = ChainTypeLocal
			} else {
				p.ChainType = ChainTypeTestnet
			}
		}
		if !validChainTypes[p.ChainType] {
			return nil, fmt.Errorf("profile %q: unknown chain-type %q (must be local or testnet)", name, p.ChainType)
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
		if p.BypassHardLimits && p.MasterAddress == "" {
			return nil, fmt.Errorf("profile %q: bypass-hard-limits requires master-address (bypass only affects write sessions)", name)
		}
		if p.MasterAddress != "" {
			if _, err := crypto.AddressFromBech32(p.MasterAddress); err != nil {
				return nil, fmt.Errorf("profile %q: invalid master-address %q: %w", name, p.MasterAddress, err)
			}
		}

		for _, f := range []struct{ name, val string }{
			{"faucet-url", p.FaucetURL},
			{"faucet-service-url", p.FaucetServiceURL},
		} {
			if f.val == "" {
				continue
			}
			if u, err := url.Parse(f.val); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				return nil, fmt.Errorf("profile %q: invalid %s %q (want an absolute http(s) URL)", name, f.name, f.val)
			}
		}

		c.Profiles[name] = p
	}
	return warn, nil
}
