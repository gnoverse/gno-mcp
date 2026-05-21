package session

import (
	"crypto/ed25519"
	"strings"
	"testing"

	"github.com/gnolang/gno/gno.land/pkg/gnoclient"
	"github.com/gnolang/gno/gno.land/pkg/sdk/vm"
	"github.com/gnolang/gno/tm2/pkg/bech32"
	"github.com/gnolang/gno/tm2/pkg/crypto"
	tmed25519 "github.com/gnolang/gno/tm2/pkg/crypto/ed25519"
	"github.com/gnolang/gno/tm2/pkg/std"
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

// TestKeypair_GnoclientSigner_signsTxWithSessionPubkey verifies that the
// adapter signs std.Tx sign-bytes with the session keypair and attaches the
// session pubkey to the resulting Signature. SessionAddr is intentionally
// left zero — chain.Real.Call injects it after Sign returns.
func TestKeypair_GnoclientSigner_signsTxWithSessionPubkey(t *testing.T) {
	kp, err := NewKeypair()
	if err != nil {
		t.Fatalf("NewKeypair: %v", err)
	}

	const chainID = "test-chain"
	signer := kp.GnoclientSigner(chainID)
	if signer == nil {
		t.Fatal("GnoclientSigner returned nil")
	}

	// Build a minimal unsigned tx. Caller is left zero — Sign only needs the
	// tx for GetSignBytes(chainID, accNum, seq); it must not enforce slot match.
	masterAddr := crypto.AddressFromPreimage([]byte("master-address-preimage"))
	tx := std.Tx{
		Msgs: []std.Msg{vm.MsgCall{
			Caller:  masterAddr,
			PkgPath: "gno.land/r/test/blog",
			Func:    "Foo",
		}},
		Fee: std.NewFee(10_000_000, std.NewCoin("ugnot", 10_000_000)),
	}

	const accNum, seq uint64 = 5, 7
	signedTx, err := signer.Sign(gnoclient.SignCfg{
		UnsignedTX:     tx,
		AccountNumber:  accNum,
		SequenceNumber: seq,
	})
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	if len(signedTx.Signatures) != 1 {
		t.Fatalf("len(Signatures) = %d, want 1", len(signedTx.Signatures))
	}

	sig := signedTx.Signatures[0]
	if sig.PubKey == nil {
		t.Fatal("Signature.PubKey is nil")
	}
	pk, ok := sig.PubKey.(tmed25519.PubKeyEd25519)
	if !ok {
		t.Fatalf("Signature.PubKey type = %T, want tmed25519.PubKeyEd25519", sig.PubKey)
	}
	if string(pk[:]) != string(kp.Pub) {
		t.Errorf("Signature.PubKey bytes do not match keypair.Pub")
	}

	if !sig.SessionAddr.IsZero() {
		t.Errorf("Signature.SessionAddr should be zero (caller fills it); got %s", sig.SessionAddr)
	}

	signBytes, err := tx.GetSignBytes(chainID, accNum, seq)
	if err != nil {
		t.Fatalf("GetSignBytes: %v", err)
	}
	if !ed25519.Verify(ed25519.PublicKey(kp.Pub), signBytes, sig.Signature) {
		t.Fatal("ed25519.Verify returned false for session-signed tx")
	}
}

// TestKeypair_GnoclientSigner_Info exposes the session pubkey via the
// gnoclient.Signer.Info() contract. Callers don't usually need it (Real.Call
// uses chain.Signer directly), but the interface requires it.
func TestKeypair_GnoclientSigner_Info(t *testing.T) {
	kp, err := NewKeypair()
	if err != nil {
		t.Fatalf("NewKeypair: %v", err)
	}
	signer := kp.GnoclientSigner("test-chain")
	info, err := signer.Info()
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.GetPubKey() == nil {
		t.Fatal("Info().GetPubKey() is nil")
	}
	pk, ok := info.GetPubKey().(tmed25519.PubKeyEd25519)
	if !ok {
		t.Fatalf("PubKey type = %T", info.GetPubKey())
	}
	if string(pk[:]) != string(kp.Pub) {
		t.Errorf("Info().GetPubKey() bytes do not match keypair.Pub")
	}
}

// TestKeypair_GnoclientSigner_Validate verifies the signer reports itself valid.
func TestKeypair_GnoclientSigner_Validate(t *testing.T) {
	kp, err := NewKeypair()
	if err != nil {
		t.Fatalf("NewKeypair: %v", err)
	}
	if err := kp.GnoclientSigner("test-chain").Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}
