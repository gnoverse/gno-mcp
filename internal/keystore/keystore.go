// Package keystore provides gnomcp's own agent write identity for dev/test
// chains. SEPARATE from the user's gnokey and from session keys: it signs
// standard transactions as the agent itself (Caller = agent address).
// Local (dev) chains use the well-known test1 account; testnet chains use
// per-profile mnemonics generated once and persisted to disk. A profile may
// hold several named keys (up to a cap) so an agent can exercise realms that
// involve multiple addresses; the unnamed default is "default". Each persisted
// mnemonic is encrypted when GNOMCP_SESSION_PASSPHRASE is set; with no
// passphrase it is stored as plaintext (acceptable for a dev/test hot key).
package keystore

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

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

// DefaultKeyName is the key a profile uses when no name is given. The empty
// string resolves to it everywhere.
const DefaultKeyName = "default"

// keyFileExt is the on-disk suffix for a persisted mnemonic.
const keyFileExt = ".key"

// keyNameRE constrains a key name to a safe single path segment.
var keyNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)

// ErrNoAgentKey reports that the profile has no agent key available: a testnet
// profile whose key has not been generated yet, or a tier without agent signing.
var ErrNoAgentKey = errors.New("keystore: no agent key for this profile")

// ErrKeyGenTestnetOnly reports that key generation was attempted on a non-testnet
// profile (local uses test1, prod uses sessions).
var ErrKeyGenTestnetOnly = errors.New("keystore: key generation is testnet-only (local uses test1, prod uses sessions)")

// ErrKeyExists reports that the profile already has a persisted key by that name.
var ErrKeyExists = errors.New("keystore: agent key already exists")

// ErrKeyCapReached reports that the profile already holds the maximum number of
// agent keys; rotate an existing one instead of adding another.
var ErrKeyCapReached = errors.New("keystore: per-profile key cap reached")

// ErrNoKeyDir reports that the keystore has no agent-keys directory configured,
// so testnet keys cannot be generated or loaded.
var ErrNoKeyDir = errors.New("keystore: no agent-keys directory configured")

// ErrInvalidKeyName reports that a key name is not a safe single path segment.
var ErrInvalidKeyName = errors.New("keystore: invalid key name (use a-z, 0-9, _ or -, max 32 chars)")

// ErrInvalidProfileName reports that a profile name is not a safe path segment.
var ErrInvalidProfileName = errors.New("keystore: invalid profile name")

// KeyInfo names a persisted key and its derived bech32 address. Err is set
// (and Address left empty) when a key file exists but cannot be read or
// decrypted — so a single bad file surfaces as a flagged entry instead of
// blinding the whole listing.
type KeyInfo struct {
	Name    string
	Address string
	Err     string
}

// Keystore provides agent signers per profile. For local (dev) profiles it signs
// as the well-known test1 account; for testnet profiles it signs with a persisted
// per-profile mnemonic selected by name.
type Keystore struct {
	rootDir    string // per-profile key files live under here; "" makes testnet key generation/load return an error
	passphrase string // GNOMCP_SESSION_PASSPHRASE, reused for at-rest encryption when non-empty
	maxKeys    int    // per-profile key cap; <= 0 means unlimited

	// genMu serialises generation so the cap count and create are atomic within
	// this instance. The cap is a per-instance SOFT guard: two processes (or two
	// Keystores) over the same rootDir can each pass the count check and overshoot
	// the cap by one. That is acceptable for a dev/test hot key; WriteFileExclusive
	// still prevents two creates clobbering the same name even across processes.
	genMu sync.Mutex
}

func New(rootDir, passphrase string, maxKeys int) *Keystore {
	return &Keystore{rootDir: rootDir, passphrase: passphrase, maxKeys: maxKeys}
}

// deriveSigner builds an in-memory signer from a mnemonic for the given chain,
// using account 0 / index 0 with no BIP39 passphrase.
func deriveSigner(mnemonic, chainID string) (gnoclient.Signer, error) {
	return gnoclient.SignerFromBip39(mnemonic, chainID, "", 0, 0)
}

// requireValidProfileName rejects a profile name that is not a safe path
// segment, so it cannot escape the keystore root when used in profileDir.
// profiles.ValidProfileName forbids ".", "/", and "..". This is defense in
// depth: dynamic profiles are already validated upstream, but init-time config
// map keys are not, and this package is the one that builds the path.
func requireValidProfileName(profileName string) error {
	if !profiles.ValidProfileName(profileName) {
		return fmt.Errorf("%w: %q", ErrInvalidProfileName, profileName)
	}
	return nil
}

// resolveKeyName maps "" to DefaultKeyName and validates the result.
func resolveKeyName(keyName string) (string, error) {
	if keyName == "" {
		keyName = DefaultKeyName
	}
	if !keyNameRE.MatchString(keyName) {
		return "", fmt.Errorf("%w: %q", ErrInvalidKeyName, keyName)
	}
	return keyName, nil
}

// SignerForProfile returns a gnoclient.Signer for the profile's named agent key.
// Local (dev) → test1 (keyName ignored, the dev chain has only test1); testnet →
// the persisted mnemonic for keyName (ErrNoAgentKey if not yet generated).
// Write-capability is re-checked as defense in depth: read-only chains have no
// agent key.
func (k *Keystore) SignerForProfile(profileName, keyName string, p profiles.Profile) (gnoclient.Signer, error) {
	if err := requireValidProfileName(profileName); err != nil {
		return nil, err
	}
	if p.IsReadOnly() {
		return nil, fmt.Errorf("keystore: chain-id %q is read-only — read-only chains have no agent key", p.ChainID)
	}
	if p.IsLocal() {
		signer, err := deriveSigner(Test1Mnemonic, p.ChainID)
		if err != nil {
			return nil, fmt.Errorf("keystore: derive test1 signer: %w", err)
		}
		return signer, nil
	}
	keyName, err := resolveKeyName(keyName)
	if err != nil {
		return nil, err
	}
	mnemonic, err := k.loadMnemonic(profileName, keyName)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNoAgentKey
	}
	if err != nil {
		return nil, fmt.Errorf("keystore: load testnet key %q/%q: %w", profileName, keyName, err)
	}
	signer, err := deriveSigner(mnemonic, p.ChainID)
	if err != nil {
		return nil, fmt.Errorf("keystore: derive testnet signer: %w", err)
	}
	return signer, nil
}

// AgentAddress returns the bech32 address of the profile's named agent key.
func (k *Keystore) AgentAddress(profileName, keyName string, p profiles.Profile) (string, error) {
	signer, err := k.SignerForProfile(profileName, keyName, p)
	if err != nil {
		return "", err
	}
	info, err := signer.Info()
	if err != nil {
		return "", fmt.Errorf("keystore: signer info: %w", err)
	}
	return info.GetAddress().String(), nil
}

// ListKeys returns the profile's agent keys with their derived addresses.
// Local (dev) profiles report a single synthetic "default" entry (test1).
// Read-only profiles have no agent keys and return an error.
func (k *Keystore) ListKeys(profileName string, p profiles.Profile) ([]KeyInfo, error) {
	if err := requireValidProfileName(profileName); err != nil {
		return nil, err
	}
	if p.IsReadOnly() {
		return nil, fmt.Errorf("keystore: chain-id %q is read-only — read-only chains have no agent key", p.ChainID)
	}
	if p.IsLocal() {
		addr, err := k.AgentAddress(profileName, DefaultKeyName, p)
		if err != nil {
			return nil, err
		}
		return []KeyInfo{{Name: DefaultKeyName, Address: addr}}, nil
	}
	if k.rootDir == "" {
		return nil, ErrNoKeyDir
	}
	entries, err := os.ReadDir(k.profileDir(profileName))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil // no keys generated yet
	}
	if err != nil {
		return nil, fmt.Errorf("keystore: list keys %q: %w", profileName, err)
	}
	var out []KeyInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), keyFileExt) {
			continue
		}
		name := strings.TrimSuffix(e.Name(), keyFileExt)
		// Per-key cost is a decrypt + BIP39 derive; bounded by the per-profile
		// cap (default 5), so listing is cheap. A key that fails to read/decrypt
		// (truncated file, passphrase change) is reported as a flagged entry
		// rather than failing the whole list — this is a recovery tool.
		addr, err := k.AgentAddress(profileName, name, p)
		if err != nil {
			// Report a stable reason, not err.Error() — the wrapped os.PathError
			// embeds the absolute key path (home dir, username), which would leak
			// to the model/client via gno_key_list.
			out = append(out, KeyInfo{Name: name, Err: unreadableReason(err)})
			continue
		}
		out = append(out, KeyInfo{Name: name, Address: addr})
	}
	slices.SortFunc(out, func(a, b KeyInfo) int { return cmp.Compare(a.Name, b.Name) })
	return out, nil
}

// GenerateForProfile creates and persists a fresh 24-word testnet key under
// keyName, returning its bech32 address. Testnet only. It is purely additive: it
// refuses to overwrite an existing name (ErrKeyExists) and refuses to add a new
// name once the per-profile cap is reached (ErrKeyCapReached). To replace a key,
// DeleteForProfile it first, then generate again. The key is encrypted when a
// passphrase is configured, otherwise stored as plaintext.
func (k *Keystore) GenerateForProfile(profileName, keyName string, p profiles.Profile) (string, error) {
	if err := requireValidProfileName(profileName); err != nil {
		return "", err
	}
	if p.IsReadOnly() {
		return "", fmt.Errorf("keystore: chain-id %q is read-only — read-only chains have no agent key", p.ChainID)
	}
	if !p.IsTestnet() {
		return "", fmt.Errorf("keystore: profile %q: %w", profileName, ErrKeyGenTestnetOnly)
	}
	if k.rootDir == "" {
		return "", ErrNoKeyDir
	}
	keyName, err := resolveKeyName(keyName)
	if err != nil {
		return "", err
	}

	k.genMu.Lock()
	defer k.genMu.Unlock()

	exists, err := k.keyExists(profileName, keyName)
	if err != nil {
		return "", err
	}
	if exists {
		return "", fmt.Errorf("keystore: profile %q key %q: %w", profileName, keyName, ErrKeyExists)
	}
	count, err := k.countKeys(profileName)
	if err != nil {
		return "", err
	}
	if k.maxKeys > 0 && count >= k.maxKeys {
		return "", fmt.Errorf("keystore: profile %q (%d/%d keys): %w", profileName, count, k.maxKeys, ErrKeyCapReached)
	}

	mnemonic, err := newMnemonic()
	if err != nil {
		return "", err
	}
	// Derive the address before persisting so a derive/Info failure can't orphan
	// a key file and wedge the name against retry (ErrKeyExists).
	signer, err := deriveSigner(mnemonic, p.ChainID)
	if err != nil {
		return "", fmt.Errorf("keystore: derive: %w", err)
	}
	info, err := signer.Info()
	if err != nil {
		return "", fmt.Errorf("keystore: info: %w", err)
	}
	addr := info.GetAddress().String()
	if err := k.saveMnemonic(profileName, keyName, mnemonic); err != nil {
		return "", err
	}
	return addr, nil
}

// DeleteForProfile removes the named testnet key, returning the bech32 address it
// held (so the caller can warn that its funds become unreachable). Testnet only.
// Returns ErrNoAgentKey if no such key exists. Deleting then generating is the
// supported way to replace a key.
func (k *Keystore) DeleteForProfile(profileName, keyName string, p profiles.Profile) (string, error) {
	if err := requireValidProfileName(profileName); err != nil {
		return "", err
	}
	if !p.IsTestnet() {
		return "", fmt.Errorf("keystore: profile %q: %w", profileName, ErrKeyGenTestnetOnly)
	}
	if k.rootDir == "" {
		return "", ErrNoKeyDir
	}
	keyName, err := resolveKeyName(keyName)
	if err != nil {
		return "", err
	}

	k.genMu.Lock()
	defer k.genMu.Unlock()

	// Derive the address before removing so the result can name the abandoned
	// account; a missing key reports ErrNoAgentKey.
	addr, err := k.AgentAddress(profileName, keyName, p)
	if errors.Is(err, ErrNoAgentKey) {
		return "", ErrNoAgentKey
	}
	if err != nil {
		return "", err
	}
	if err := os.Remove(k.keyPath(profileName, keyName)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrNoAgentKey
		}
		return "", fmt.Errorf("keystore: delete key %q/%q: %w", profileName, keyName, err)
	}
	return addr, nil
}

// newMnemonic returns a fresh 24-word BIP-39 mnemonic.
func newMnemonic() (string, error) {
	entropy, err := bip39.NewEntropy(256)
	if err != nil {
		return "", fmt.Errorf("keystore: entropy: %w", err)
	}
	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return "", fmt.Errorf("keystore: mnemonic: %w", err)
	}
	return mnemonic, nil
}

// unreadableReason maps a key-load failure to a stable, path-free reason for
// ListKeys output. It distinguishes an I/O/permission failure from a decrypt
// failure without echoing the wrapped error (which carries the file path).
func unreadableReason(err error) string {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return "unreadable (I/O or permission error)"
	}
	return "could not decrypt (passphrase mismatch or corrupt file)"
}

func (k *Keystore) profileDir(profileName string) string {
	return filepath.Join(k.rootDir, profileName)
}

func (k *Keystore) keyPath(profileName, keyName string) string {
	return filepath.Join(k.profileDir(profileName), keyName+keyFileExt)
}

// keyExists reports whether a key file for keyName exists.
func (k *Keystore) keyExists(profileName, keyName string) (bool, error) {
	_, err := os.Stat(k.keyPath(profileName, keyName))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("keystore: stat key %q/%q: %w", profileName, keyName, err)
}

// countKeys returns how many key files the profile holds (0 if none).
func (k *Keystore) countKeys(profileName string) (int, error) {
	entries, err := os.ReadDir(k.profileDir(profileName))
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("keystore: count keys %q: %w", profileName, err)
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), keyFileExt) {
			n++
		}
	}
	return n, nil
}

func (k *Keystore) loadMnemonic(profileName, keyName string) (string, error) {
	if k.rootDir == "" {
		return "", ErrNoKeyDir
	}
	raw, err := os.ReadFile(k.keyPath(profileName, keyName))
	if err != nil {
		return "", err
	}
	plain, err := secret.Decrypt(raw, k.passphrase)
	if err != nil {
		return "", fmt.Errorf("keystore: decrypt %q/%q: %w", profileName, keyName, err)
	}
	return string(plain), nil
}

// saveMnemonic persists an encrypted mnemonic with a create-or-fail write, so a
// concurrent generate of the same name cannot clobber and a crash mid-write never
// leaves a partial file that would wedge the name behind ErrKeyExists.
func (k *Keystore) saveMnemonic(profileName, keyName, mnemonic string) error {
	enc, err := secret.Encrypt([]byte(mnemonic), k.passphrase)
	if err != nil {
		return fmt.Errorf("keystore: encrypt: %w", err)
	}
	if err := fsutil.WriteFileExclusive(k.keyPath(profileName, keyName), enc, 0o600); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("keystore: profile %q key %q: %w", profileName, keyName, ErrKeyExists)
		}
		return fmt.Errorf("keystore: write: %w", err)
	}
	return nil
}
