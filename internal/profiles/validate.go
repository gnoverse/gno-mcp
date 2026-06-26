package profiles

import (
	"fmt"
	"net/url"
	"regexp"
	"time"

	"github.com/gnolang/gno/tm2/pkg/crypto"
)

var (
	// spendLimitRE matches a single-denomination coin amount: digits followed by
	// lowercase letters (e.g. "1000000ugnot", "10gnot"). Cross-denom mixes rejected.
	spendLimitRE = regexp.MustCompile(`^[0-9]+[a-z]+$`)

	// chainIDWritableRE marks the write-capable chains: local dev and numbered
	// testnets. Admits both the bare "test5" and hyphenated "test-13" forms. Anything else
	// (betanet "gnoland1", "staging", ...) is admitted read-only, not writable —
	// it has no agent key path and is excluded from every write tool's profile enum.
	chainIDWritableRE = regexp.MustCompile(`^(dev|test-?\d+)$`)

	// chainIDFormatRE is the format-safety gate applied to every chain-id,
	// writable or read-only: the chain-id is interpolated into the `gnomcp
	// profile add` and gnokey commands the user pastes into a terminal, so
	// whitespace and shell metacharacters must be refused. Lowercase to match
	// gno chain-id convention (dev, test5, gnoland1, portal-loop). First and
	// last char are anchored to [a-z0-9]: the leading anchor stops an id from
	// beginning with '-' and being read as a flag in the pasted command; the
	// trailing anchor rejects dangling separators ("test-", "staging.").
	chainIDFormatRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9._-]*[a-z0-9])?$`)

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

// ChainIDWritable reports whether a chain-id is write-capable: local dev or a
// numbered testnet. These get an agent key path (test1 or a generated key) and
// appear in the write tools' profile enums. Any other chain-id is read-only.
func ChainIDWritable(chainID string) bool {
	return chainIDWritableRE.MatchString(chainID)
}

// ChainIDValid reports whether a chain-id is safe to admit into config and to
// interpolate into pasted shell commands: non-empty, ≤64 chars, lowercase
// alphanumeric with internal '.', '-', '_'. This is the format gate applied to
// read-only and writable chains alike.
func ChainIDValid(chainID string) bool {
	return len(chainID) <= 64 && chainIDFormatRE.MatchString(chainID)
}

// Validate checks required fields. It never mutates profiles.
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
		if p.GnowebURL != "" && !ValidRPCURL(p.GnowebURL) {
			return nil, fmt.Errorf("profile %q: invalid gnoweb-url %q (want an absolute http(s) URL with no spaces or shell metacharacters)", name, p.GnowebURL)
		}
		if p.ChainID == "" {
			return nil, fmt.Errorf("profile %q: missing required chain-id", name)
		}
		if !ChainIDValid(p.ChainID) {
			return nil, fmt.Errorf("profile %q: chain-id %q is malformed (want lowercase alphanumeric with '.', '-', '_', ≤64 chars — it is interpolated into pasted commands)", name, p.ChainID)
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
			if !ChainIDWritable(p.ChainID) {
				return nil, fmt.Errorf("profile %q: master-address is set but chain-id %q is read-only (mainnet/betanet) — read-only chains cannot perform writes; remove master-address or target a dev/testNN chain", name, p.ChainID)
			}
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
	}
	return warn, nil
}
