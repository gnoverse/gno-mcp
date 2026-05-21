// Package session manages gnomcp's chain-bounded session keys: ed25519 generation,
// scrypt+AES-GCM disk persistence, 4-layer scope policy, and per-pubkey chain verification.
package session

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"fmt"

	"github.com/gnolang/gno/tm2/pkg/bech32"
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
func (kp *Keypair) PubkeyBech32() string {
	encoded, err := bech32.Encode(pubkeyPrefix, kp.Pub)
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
