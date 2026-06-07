// Package keystore provides gnomcp's own agent write identity for dev/test
// chains. SEPARATE from the user's gnokey and from session keys: it signs
// standard transactions as the agent itself (Caller = agent address).
// Plan A: local (dev) only, using the well-known test1 account.
package keystore

import (
	"errors"
	"fmt"

	gnoclient "github.com/gnolang/gno/gno.land/pkg/gnoclient"

	"github.com/gnoverse/gno-mcp/internal/profiles"
)

// test1: canonical local dev account gnodev always funds. Source:
// gno.land/pkg/integration/node_testing.go (DefaultAccount_*). secp256k1, 44'/118'/0'/0/0.
const (
	Test1Name     = "test1"
	Test1Address  = "g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5"
	Test1Mnemonic = "source bonus chronic canvas draft south burst lottery vacant surface solve popular case indicate oppose farm nothing bullet exhibit title speed wink action roast"
)

// ErrNoAgentKey reports that the profile's tier has no agent key. In Plan A this
// is every non-local tier (testnet arrives in Plan B; prod never).
var ErrNoAgentKey = errors.New("keystore: no agent key for this profile")

// Keystore provides agent signers per profile. Stateless in Plan A (test1 is a
// constant); Plan B adds a persistent keybase for generated testnet keys.
type Keystore struct{}

func New() *Keystore { return &Keystore{} }

// SignerForProfile returns a gnoclient.Signer for the profile's agent identity.
// Local (dev) → test1; any other tier → ErrNoAgentKey. The chain-id allowlist is
// re-checked as defense in depth.
func (k *Keystore) SignerForProfile(p profiles.Profile) (gnoclient.Signer, error) {
	if !profiles.ChainIDAllowed(p.ChainID) {
		return nil, fmt.Errorf("keystore: chain-id %q not allowed", p.ChainID)
	}
	if p.ChainType != profiles.ChainTypeLocal {
		return nil, ErrNoAgentKey
	}
	signer, err := gnoclient.SignerFromBip39(Test1Mnemonic, p.ChainID, "", 0, 0)
	if err != nil {
		return nil, fmt.Errorf("keystore: derive test1 signer: %w", err)
	}
	return signer, nil
}

// AgentAddress returns the bech32 address of the profile's agent identity.
func (k *Keystore) AgentAddress(p profiles.Profile) (string, error) {
	signer, err := k.SignerForProfile(p)
	if err != nil {
		return "", err
	}
	info, err := signer.Info()
	if err != nil {
		return "", fmt.Errorf("keystore: signer info: %w", err)
	}
	return info.GetAddress().String(), nil
}
