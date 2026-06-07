package profiles

// Built-in network endpoints. testnet is a release-time constant: bump when the
// canonical persistent testnet rolls (verified live 2026-06-04: test11, block 2.27M+).
// test12 was dead and test13 had no canonical host at the time of writing.
const (
	builtinLocalRPC   = "http://127.0.0.1:26657"
	builtinLocalChain = "dev"

	builtinTestnetRPC   = "https://rpc.test11.testnets.gno.land:443"
	builtinTestnetChain = "test11"
)

// BuiltinProfiles returns the zero-config default profiles. Both are read-only
// (no master-address); the user opts into writes by setting one. Returned as a
// fresh map each call so callers may mutate it.
func BuiltinProfiles() map[string]Profile {
	return map[string]Profile{
		"local": {
			ChainType: ChainTypeLocal,
			RPCURL:    builtinLocalRPC,
			ChainID:   builtinLocalChain,
		},
		"testnet": {
			ChainType: ChainTypeTestnet,
			RPCURL:    builtinTestnetRPC,
			ChainID:   builtinTestnetChain,
		},
	}
}
