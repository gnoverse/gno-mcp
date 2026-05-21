// Package session manages gnomcp's chain-bounded session keys: ed25519 generation,
// scrypt+AES-GCM disk persistence, 4-layer scope policy, and per-pubkey chain verification.
package session

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"fmt"

	"github.com/gnolang/gno/gno.land/pkg/gnoclient"
	"github.com/gnolang/gno/tm2/pkg/bech32"
	"github.com/gnolang/gno/tm2/pkg/crypto"
	tmed25519 "github.com/gnolang/gno/tm2/pkg/crypto/ed25519"
	"github.com/gnolang/gno/tm2/pkg/crypto/hd"
	"github.com/gnolang/gno/tm2/pkg/crypto/keys"
	"github.com/gnolang/gno/tm2/pkg/std"
)

const (
	addrPrefix   = "g"
	pubkeyPrefix = "gpub"
	addrSize     = 20
)

// Keypair holds an ed25519 session keypair generated in-process.
// The private key never leaves gnomcp; only the bech32 pubkey is
// sent to the chain (and to the user's gnokey command).
type Keypair struct {
	Pub  []byte // ed25519 public key (32 bytes)
	Priv []byte // ed25519 private key (64 bytes)
}

// NewKeypair generates a fresh ed25519 Keypair using crypto/rand.
// Returns an error only if the OS CSPRNG fails.
func NewKeypair() (*Keypair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("session: generate ed25519 keypair: %w", err)
	}
	return &Keypair{
		Pub:  []byte(pub),
		Priv: []byte(priv),
	}, nil
}

// PubkeyBech32 encodes the public key as a gpub1... bech32 string.
// Mirrors crypto.PubKeyToBech32: the pubkey is amino-marshaled to its
// typed wire form (which includes the ed25519 type prefix) and that byte
// string is bech32-encoded with HRP "gpub". This is the form the chain
// expects on flags like `gnokey maketx session create --pubkey gpub1...`
// and is the inverse of crypto.PubKeyFromBech32.
func (kp *Keypair) PubkeyBech32() string {
	var pk tmed25519.PubKeyEd25519
	copy(pk[:], kp.Pub)
	encoded, err := bech32.Encode(pubkeyPrefix, pk.Bytes())
	if err != nil {
		panic(fmt.Sprintf("session: bech32 encode pubkey: %v", err))
	}
	return encoded
}

// Address derives the session's g1... address from the pubkey.
// Derivation: first 20 bytes of SHA-256(pubkey), bech32-encoded with prefix "g".
func (kp *Keypair) Address() string {
	h := sha256.Sum256(kp.Pub)
	addrBytes := h[:addrSize]
	encoded, err := bech32.Encode(addrPrefix, addrBytes)
	if err != nil {
		panic(fmt.Sprintf("session: bech32 encode address: %v", err))
	}
	return encoded
}

// Sign signs payload with the ed25519 private key and returns the 64-byte signature.
func (kp *Keypair) Sign(payload []byte) ([]byte, error) {
	sig := ed25519.Sign(ed25519.PrivateKey(kp.Priv), payload)
	return sig, nil
}

// Pubkey returns a copy of the raw ed25519 public key (32 bytes), satisfying
// chain.Signer.Pubkey for session-signed tx construction.
func (kp *Keypair) Pubkey() []byte {
	out := make([]byte, len(kp.Pub))
	copy(out, kp.Pub)
	return out
}

// TMPubKey returns the session pubkey wrapped in the tm2 crypto.PubKey type,
// suitable for embedding in std.Signature.PubKey.
func (kp *Keypair) TMPubKey() crypto.PubKey {
	var pk tmed25519.PubKeyEd25519
	copy(pk[:], kp.Pub)
	return pk
}

// GnoclientSigner returns a gnoclient.Signer that signs std.Tx with this
// session's ed25519 keypair against the given chainID. The returned Signer
// populates Signature.PubKey with the session pubkey but leaves
// Signature.SessionAddr zero — callers (chain.Real.Call/Run) inject
// SessionAddr after Sign returns so the chain ante handler can look up the
// session under its master account.
func (kp *Keypair) GnoclientSigner(chainID string) gnoclient.Signer {
	return &gnoclientSigner{kp: kp, chainID: chainID}
}

// gnoclientSigner adapts *Keypair to gnoclient.Signer for session-signed txs.
// It does NOT use the slot-match logic in SignerFromKeybase.Sign — for session
// signing the master address is the signer set but the session keypair signs,
// which makes that slot match fail. Instead it always writes the signature
// into Signatures[0].
type gnoclientSigner struct {
	kp      *Keypair
	chainID string
}

// Sign computes std.Tx sign-bytes against (chainID, accNum, seq) and signs
// them with the session ed25519 key. The returned tx has exactly one
// std.Signature{PubKey: <session pubkey>, Signature: <sig>}; SessionAddr is
// left zero for the caller to populate.
func (s *gnoclientSigner) Sign(cfg gnoclient.SignCfg) (*std.Tx, error) {
	tx := cfg.UnsignedTX
	signBytes, err := tx.GetSignBytes(s.chainID, cfg.AccountNumber, cfg.SequenceNumber)
	if err != nil {
		return nil, fmt.Errorf("session/gnoclientSigner: build sign bytes: %w", err)
	}
	sig := ed25519.Sign(ed25519.PrivateKey(s.kp.Priv), signBytes)
	tx.Signatures = []std.Signature{{
		PubKey:    s.kp.TMPubKey(),
		Signature: sig,
	}}
	return &tx, nil
}

// Info returns the session keypair's identity as a keys.Info.
func (s *gnoclientSigner) Info() (keys.Info, error) {
	return sessionKeyInfo{pubkey: s.kp.TMPubKey()}, nil
}

// Validate reports the signer as ready — an in-memory ed25519 keypair has no
// external state to check.
func (s *gnoclientSigner) Validate() error { return nil }

// sessionKeyInfo implements keys.Info for an in-memory session keypair.
// All exported keys.Info constructors require a Keybase entry, so we satisfy
// the interface directly. Path is unsupported (sessions are not derived from
// BIP44); GetType returns TypeLocal as the closest semantic.
type sessionKeyInfo struct {
	pubkey crypto.PubKey
}

func (i sessionKeyInfo) GetType() keys.KeyType    { return keys.TypeLocal }
func (i sessionKeyInfo) GetName() string          { return "gnomcp-session" }
func (i sessionKeyInfo) GetPubKey() crypto.PubKey { return i.pubkey }
func (i sessionKeyInfo) GetAddress() crypto.Address {
	return i.pubkey.Address()
}

func (i sessionKeyInfo) GetPath() (*hd.BIP44Params, error) {
	return nil, fmt.Errorf("session keys are not derived from a BIP44 path")
}
