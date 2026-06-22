// Package profiles parses and represents the multi-chain profile
// configuration that drives gnomcp's connections to gno chains.
package profiles

import (
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"

	"github.com/BurntSushi/toml"
)

// Profile is a single chain profile loaded from profiles.toml. Locality
// (local vs testnet) is derived from ChainID via IsLocal/IsTestnet, never
// configured.
type Profile struct {
	// Connection settings.
	RPCURL       string `toml:"rpc-url"`
	ChainID      string `toml:"chain-id"`
	TxIndexerURL string `toml:"tx-indexer-url"`
	GnowebURL    string `toml:"gnoweb-url"` // optional; the gnoweb frontend where this chain's realms are viewable (e.g. https://gno.land). Display only — gnomcp reads state via RPC, not gnoweb.

	// Faucet settings (optional; testnet only). FaucetServiceURL points at an
	// automatic agent-faucet service (tier 2); FaucetURL at a human faucet page
	// (tier 1). Precedence: service > link > manual fallback.
	FaucetServiceURL string `toml:"faucet-service-url"`
	FaucetURL        string `toml:"faucet-url"`

	// Write authorization settings.
	MasterAddress     string `toml:"master-address"`      // bech32 master account address (g1...); presence enables write tools for this profile. No key material — public address only.
	DefaultSpendLimit string `toml:"default-spend-limit"` // optional; clamped at use
	DefaultExpiresIn  string `toml:"default-expires-in"`  // optional; Go duration string
	BypassHardLimits  bool   `toml:"bypass-hard-limits"`  // default false; disables per-chain clamps
}

// IsLocal reports whether the profile targets a local dev node. Locality is
// derived from chain-id, never configured.
func (p Profile) IsLocal() bool { return p.ChainID == "dev" }

// IsTestnet reports whether the profile targets a write-capable testnet (a
// numbered "testNN" chain). Read-only chains (mainnet/betanet) are NOT testnets:
// they have no agent key path and no faucet.
func (p Profile) IsTestnet() bool { return ChainIDWritable(p.ChainID) && !p.IsLocal() }

// IsReadOnly reports whether the profile targets a non-write-capable chain
// (anything other than dev or a numbered testnet, e.g. betanet "gnoland1").
// Read-only profiles are readable via the read tools but excluded from every
// write tool's profile enum.
func (p Profile) IsReadOnly() bool { return !ChainIDWritable(p.ChainID) }

// RealmViewURL returns the gnoweb URL where pkgPath is viewable, or "" when this
// profile has no usable gnoweb host (e.g. a local node). It prefers the
// configured GnowebURL and otherwise derives the host from RPCURL by dropping
// the "rpc." prefix and the :443 port. gnoweb serves realm "gno.land/r/x" at
// "/r/x", so the "gno.land/" path prefix is dropped.
func (p Profile) RealmViewURL(pkgPath string) string {
	base := p.gnowebBase()
	if base == "" {
		return ""
	}
	return base + "/" + strings.TrimPrefix(pkgPath, "gno.land/")
}

// gnowebBase returns the base gnoweb URL for this profile (no trailing slash):
// the configured GnowebURL when set, otherwise the RPC host with the "rpc."
// prefix and the :443 port stripped. Returns "" when neither yields a public
// http host (a local node has no gnoweb to point at).
func (p Profile) gnowebBase() string {
	if p.GnowebURL != "" {
		return strings.TrimSuffix(p.GnowebURL, "/")
	}
	base := strings.Replace(p.RPCURL, "://rpc.", "://", 1)
	base = strings.TrimSuffix(base, ":443")
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || !isPublicHost(u.Hostname()) {
		return ""
	}
	return strings.TrimSuffix(base, "/")
}

// isPublicHost reports whether host is a routable public gnoweb host: not a
// loopback/unspecified/private/link-local IP, and not a bare dotless name
// (localhost, a container alias) that has no public gnoweb to point at.
func isPublicHost(host string) bool {
	if ip := net.ParseIP(host); ip != nil {
		return !ip.IsLoopback() && !ip.IsUnspecified() && !ip.IsPrivate() && !ip.IsLinkLocalUnicast()
	}
	return strings.Contains(host, ".")
}

// Kind returns "local", "testnet", or "read-only" for display (signed-by lines,
// clamp warnings, startup instructions).
func (p Profile) Kind() string {
	switch {
	case p.IsLocal():
		return "local"
	case p.IsReadOnly():
		return "read-only"
	default:
		return "testnet"
	}
}

// Config is the root of profiles.toml.
type Config struct {
	Profiles map[string]Profile
}

// Load parses a profiles.toml document from r.
func Load(r io.Reader) (*Config, error) {
	cfg := &Config{Profiles: map[string]Profile{}}
	md, err := toml.NewDecoder(r).Decode(&cfg.Profiles)
	if err != nil {
		return nil, fmt.Errorf("parse profiles.toml: %w", err)
	}
	if undec := md.Undecoded(); len(undec) > 0 {
		return nil, fmt.Errorf("parse profiles.toml: unknown keys: %v", undec)
	}
	return cfg, nil
}

// Built-in network endpoints. testnet is a release-time constant: bump when the
// canonical persistent testnet rolls. The chain reports its chain-id hyphenated
// ("test-13") while its hosts use the unhyphenated form ("test13.testnets.gno.land").
const (
	builtinLocalRPC   = "http://127.0.0.1:26657"
	builtinLocalChain = "dev"

	builtinTestnetRPC     = "https://rpc.test13.testnets.gno.land:443"
	builtinTestnetChain   = "test-13"
	builtinTestnetGnoweb  = "https://test13.testnets.gno.land"
	builtinTestnetIndexer = "https://indexer.test13.testnets.gno.land/graphql/query"
	builtinTestnetFaucet  = "https://faucet-agent.test13.testnets.gno.land"
)

// BuiltinProfiles returns the zero-config default profiles. Both are read-only
// (no master-address); the user opts into writes by setting one. Returned as a
// fresh map each call so callers may mutate it.
func BuiltinProfiles() map[string]Profile {
	return map[string]Profile{
		"local": {
			RPCURL:  builtinLocalRPC,
			ChainID: builtinLocalChain,
		},
		"testnet": {
			RPCURL:           builtinTestnetRPC,
			ChainID:          builtinTestnetChain,
			GnowebURL:        builtinTestnetGnoweb,
			TxIndexerURL:     builtinTestnetIndexer,
			FaucetServiceURL: builtinTestnetFaucet,
		},
	}
}

// Merge returns a new profile map where each overlay entry overrides the base
// entry of the same name (whole-profile replacement, not field-level merge).
// Base is not mutated.
func Merge(base, overlay map[string]Profile) map[string]Profile {
	out := make(map[string]Profile, len(base)+len(overlay))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overlay {
		out[k] = v
	}
	return out
}
