// Package keystore provides gnomcp's own agent write identity for dev/test
// chains. SEPARATE from the user's gnokey and from session keys: it signs
// standard transactions as the agent itself (Caller = agent address).
// Local (dev) chains use the well-known test1 account; testnet chains use a
// per-profile mnemonic generated once and persisted to disk. The persisted
// mnemonic is encrypted when GNOMCP_SESSION_PASSPHRASE is set; with no
// passphrase it is stored as plaintext (acceptable for a dev/test hot key).
package keystore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	bip39 "github.com/gnolang/gno/tm2/pkg/crypto/bip39"

	gnoclient "github.com/gnolang/gno/gno.land/pkg/gnoclient"

	"github.com/gnoverse/gno-mcp/internal/fsutil"
	"github.com/gnoverse/gno-mcp/internal/profiles"
	secret "github.com/gnoverse/gno-mcp/internal/secret"
)

// test1: canonical local dev account gnodev always funds. Source:
// gno.land/pkg/integration/node_testing.go (DefaultAccount_*). secp256k1, 44'/118'/0'/0/0.
const (
	Test1Name     = "test1"
	Test1Address  = "g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5"
	Test1Mnemonic = "source bonus chronic canvas draft south burst lottery vacant surface solve popular case indicate oppose farm nothing bullet exhibit title speed wink action roast"
)

// ErrNoAgentKey reports that the profile has no agent key available: a testnet
// profile whose key has not been generated yet, or a tier without agent signing.
var ErrNoAgentKey = errors.New("keystore: no agent key for this profile")

// ErrKeyGenTestnetOnly reports that key generation was attempted on a non-testnet
// profile (local uses test1, prod uses sessions).
var ErrKeyGenTestnetOnly = errors.New("keystore: key generation is testnet-only (local uses test1, prod uses sessions)")

// ErrKeyExists reports that the profile already has a persisted agent key.
var ErrKeyExists = errors.New("keystore: agent key already exists")

// ErrNoKeyDir reports that the keystore has no agent-keys directory configured,
// so testnet keys cannot be generated or loaded.
var ErrNoKeyDir = errors.New("keystore: no agent-keys directory configured")

// Keystore provides agent signers per profile. For local (dev) profiles it signs
// as the well-known test1 account; for testnet profiles it signs with a persisted
// per-profile mnemonic.
type Keystore struct {
	rootDir    string // per-profile mnemonic files live here; "" makes testnet key generation/load return an error
	passphrase string // GNOMCP_SESSION_PASSPHRASE, reused for at-rest encryption when non-empty
}

func New(rootDir, passphrase string) *Keystore {
	return &Keystore{rootDir: rootDir, passphrase: passphrase}
}

// deriveSigner builds an in-memory signer from a mnemonic for the given chain,
// using account 0 / index 0 with no BIP39 passphrase.
func deriveSigner(mnemonic, chainID string) (gnoclient.Signer, error) {
	return gnoclient.SignerFromBip39(mnemonic, chainID, "", 0, 0)
}

// SignerForProfile returns a gnoclient.Signer for the profile's agent identity.
// Local (dev) → test1; testnet → persisted per-profile mnemonic (ErrNoAgentKey
// if not yet generated). The chain-id allowlist is re-checked as defense in depth.
func (k *Keystore) SignerForProfile(profileName string, p profiles.Profile) (gnoclient.Signer, error) {
	if !profiles.ChainIDAllowed(p.ChainID) {
		return nil, fmt.Errorf("keystore: chain-id %q not allowed", p.ChainID)
	}
	switch p.ChainType {
	case profiles.ChainTypeLocal:
		signer, err := deriveSigner(Test1Mnemonic, p.ChainID)
		if err != nil {
			return nil, fmt.Errorf("keystore: derive test1 signer: %w", err)
		}
		return signer, nil
	case profiles.ChainTypeTestnet:
		mnemonic, err := k.loadMnemonic(profileName)
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNoAgentKey
		}
		if err != nil {
			return nil, fmt.Errorf("keystore: load testnet key %q: %w", profileName, err)
		}
		signer, err := deriveSigner(mnemonic, p.ChainID)
		if err != nil {
			return nil, fmt.Errorf("keystore: derive testnet signer: %w", err)
		}
		return signer, nil
	default:
		return nil, ErrNoAgentKey
	}
}

// AgentAddress returns the bech32 address of the profile's agent identity.
func (k *Keystore) AgentAddress(profileName string, p profiles.Profile) (string, error) {
	signer, err := k.SignerForProfile(profileName, p)
	if err != nil {
		return "", err
	}
	info, err := signer.Info()
	if err != nil {
		return "", fmt.Errorf("keystore: signer info: %w", err)
	}
	return info.GetAddress().String(), nil
}

// GenerateForProfile creates and persists a fresh 24-word testnet agent key,
// returning its bech32 address. Testnet only; refuses to overwrite an existing
// key file. The key is written encrypted when a passphrase is configured,
// otherwise as plaintext.
func (k *Keystore) GenerateForProfile(profileName string, p profiles.Profile) (string, error) {
	if !profiles.ChainIDAllowed(p.ChainID) {
		return "", fmt.Errorf("keystore: chain-id %q not allowed", p.ChainID)
	}
	if p.ChainType != profiles.ChainTypeTestnet {
		return "", fmt.Errorf("keystore: profile %q: %w", profileName, ErrKeyGenTestnetOnly)
	}
	if k.rootDir == "" {
		return "", ErrNoKeyDir
	}
	if _, err := os.Stat(k.keyPath(profileName)); err == nil {
		return "", fmt.Errorf("keystore: profile %q: %w", profileName, ErrKeyExists)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("keystore: stat key %q: %w", profileName, err)
	}
	entropy, err := bip39.NewEntropy(256)
	if err != nil {
		return "", fmt.Errorf("keystore: entropy: %w", err)
	}
	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return "", fmt.Errorf("keystore: mnemonic: %w", err)
	}
	// Derive the address before persisting so a derive/Info failure can't orphan
	// a key file and wedge the profile against retry (ErrKeyExists).
	signer, err := deriveSigner(mnemonic, p.ChainID)
	if err != nil {
		return "", fmt.Errorf("keystore: derive: %w", err)
	}
	info, err := signer.Info()
	if err != nil {
		return "", fmt.Errorf("keystore: info: %w", err)
	}
	addr := info.GetAddress().String()
	if err := k.saveMnemonic(profileName, mnemonic); err != nil {
		return "", err
	}
	return addr, nil
}

func (k *Keystore) keyPath(profileName string) string {
	return filepath.Join(k.rootDir, profileName+".key")
}

func (k *Keystore) loadMnemonic(profileName string) (string, error) {
	if k.rootDir == "" {
		return "", ErrNoKeyDir
	}
	raw, err := os.ReadFile(k.keyPath(profileName))
	if err != nil {
		return "", err
	}
	plain, err := secret.Decrypt(raw, k.passphrase)
	if err != nil {
		return "", fmt.Errorf("keystore: decrypt %q: %w", profileName, err)
	}
	return string(plain), nil
}

func (k *Keystore) saveMnemonic(profileName, mnemonic string) error {
	enc, err := secret.Encrypt([]byte(mnemonic), k.passphrase)
	if err != nil {
		return fmt.Errorf("keystore: encrypt: %w", err)
	}
	// Exclusive atomic write: the create-or-fail is race-safe against a concurrent
	// generate, and a crash mid-write never leaves a partial file that would wedge
	// the profile behind ErrKeyExists.
	if err := fsutil.WriteFileExclusive(k.keyPath(profileName), enc, 0o600); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("keystore: profile %q: %w", profileName, ErrKeyExists)
		}
		return fmt.Errorf("keystore: write: %w", err)
	}
	return nil
}
