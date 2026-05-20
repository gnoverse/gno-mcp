package profiles

import "fmt"

// Valid chain types.
const (
	ChainTypeLocal   = "local"
	ChainTypeTestnet = "testnet"
	ChainTypeMainnet = "mainnet"
)

var validChainTypes = map[string]bool{
	ChainTypeLocal:   true,
	ChainTypeTestnet: true,
	ChainTypeMainnet: true,
}

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
		c.Profiles[name] = p
	}
	return nil
}
