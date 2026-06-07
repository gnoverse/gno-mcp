// Package profiles parses and represents the multi-chain profile
// configuration that drives gnomcp's connections to gno chains.
package profiles

import (
	"fmt"
	"io"

	"github.com/BurntSushi/toml"
)

// Profile is a single chain profile loaded from profiles.toml.
type Profile struct {
	// Connection settings.
	ChainType    string `toml:"chain-type"`
	RPCURL       string `toml:"rpc-url"`
	ChainID      string `toml:"chain-id"`
	TxIndexerURL string `toml:"tx-indexer-url"`

	// Write authorization settings.
	MasterAddress     string `toml:"master-address"`      // bech32 master account address (g1...); presence enables write tools for this profile. No key material — public address only.
	DefaultSpendLimit string `toml:"default-spend-limit"` // optional; clamped at use
	DefaultExpiresIn  string `toml:"default-expires-in"`  // optional; Go duration string
	BypassHardLimits  bool   `toml:"bypass-hard-limits"`  // default false; disables per-chain clamps
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
