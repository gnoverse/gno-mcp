package session

import (
	"crypto/ed25519"
	"strings"
	"testing"

	"github.com/gnolang/gno/tm2/pkg/bech32"
)

func TestNewKeypair_distinctEachCall(t *testing.T) {
	seen := make(map[string]bool, 100)
	for i := 0; i < 100; i++ {
		kp, err := NewKeypair()
		if err != nil {
			t.Fatalf("iteration %d: NewKeypair() error: %v", i, err)
		}
		key := string(kp.Pub)
		if seen[key] {
			t.Fatalf("iteration %d: duplicate pubkey generated", i)
		}
		seen[key] = true
	}
}

func TestPubkeyBech32_roundTrip(t *testing.T) {
	kp, err := NewKeypair()
	if err != nil {
		t.Fatalf("NewKeypair(): %v", err)
	}
	encoded := kp.PubkeyBech32()
	if !strings.HasPrefix(encoded, "gpub1") {
		t.Fatalf("PubkeyBech32() = %q, want prefix \"gpub1\"", encoded)
	}
	hrp, decoded, err := bech32.Decode(encoded)
	if err != nil {
		t.Fatalf("bech32.Decode(%q): %v", encoded, err)
	}
	if hrp != "gpub" {
		t.Fatalf("decoded hrp = %q, want \"gpub\"", hrp)
	}
	if string(decoded) != string(kp.Pub) {
		t.Fatalf("decoded pubkey bytes do not match original:\n got  %x\n want %x", decoded, kp.Pub)
	}
}

func TestAddress_format(t *testing.T) {
	kp, err := NewKeypair()
	if err != nil {
		t.Fatalf("NewKeypair(): %v", err)
	}
	addr := kp.Address()
	if !strings.HasPrefix(addr, "g1") {
		t.Fatalf("Address() = %q, want prefix \"g1\"", addr)
	}
	hrp, addrBytes, err := bech32.Decode(addr)
	if err != nil {
		t.Fatalf("bech32.Decode(%q): %v", addr, err)
	}
	if hrp != "g" {
		t.Fatalf("address hrp = %q, want \"g\"", hrp)
	}
	if len(addrBytes) != addrSize {
		t.Fatalf("address byte length = %d, want %d", len(addrBytes), addrSize)
	}
}

func TestKeypair_Sign_verifiable(t *testing.T) {
	kp, err := NewKeypair()
	if err != nil {
		t.Fatalf("NewKeypair(): %v", err)
	}
	payload := []byte("gnomcp-test-payload")
	sig, err := kp.Sign(payload)
	if err != nil {
		t.Fatalf("Sign(): %v", err)
	}
	if !ed25519.Verify(ed25519.PublicKey(kp.Pub), payload, sig) {
		t.Fatal("ed25519.Verify returned false for a freshly generated signature")
	}
}
