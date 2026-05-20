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
	ChainType    string `toml:"chain-type"`
	RPCURL       string `toml:"rpc-url"`
	ChainID      string `toml:"chain-id"`
	TxIndexerURL string `toml:"tx-indexer-url"`
}

// Config is the root of profiles.toml.
type Config struct {
	Profiles map[string]Profile
}

// Load parses a profiles.toml document from r.
func Load(r io.Reader) (*Config, error) {
	cfg := &Config{Profiles: map[string]Profile{}}
	if _, err := toml.NewDecoder(r).Decode(&cfg.Profiles); err != nil {
		return nil, fmt.Errorf("parse profiles.toml: %w", err)
	}
	return cfg, nil
}
