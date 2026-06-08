package session

import (
	"crypto/ed25519"
	"testing"

	"github.com/gnolang/gno/gno.land/pkg/gnoclient"
	"github.com/gnolang/gno/gno.land/pkg/sdk/vm"
	"github.com/gnolang/gno/tm2/pkg/bech32"
	"github.com/gnolang/gno/tm2/pkg/crypto"
	tmed25519 "github.com/gnolang/gno/tm2/pkg/crypto/ed25519"
	"github.com/gnolang/gno/tm2/pkg/std"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewKeypair_distinctEachCall(t *testing.T) {
	seen := make(map[string]bool, 100)
	for i := 0; i < 100; i++ {
		kp, err := NewKeypair()
		require.NoError(t, err, "iteration %d: NewKeypair() error", i)
		key := string(kp.Pub)
		require.False(t, seen[key], "iteration %d: duplicate pubkey generated", i)
		seen[key] = true
	}
}

func TestPubkeyBech32_roundTrip(t *testing.T) {
	kp, err := NewKeypair()
	require.NoError(t, err)

	encoded := kp.PubkeyBech32()
	require.True(t, len(encoded) > 5 && encoded[:5] == "gpub1",
		"PubkeyBech32() = %q, want prefix \"gpub1\"", encoded)

	// The encoded form must round-trip through crypto.PubKeyFromBech32 (the
	// chain's inverse), which amino-decodes the bytes into a typed PubKey.
	pk, err := crypto.PubKeyFromBech32(encoded)
	require.NoError(t, err, "crypto.PubKeyFromBech32(%q)", encoded)

	tmpk, ok := pk.(tmed25519.PubKeyEd25519)
	require.True(t, ok, "decoded pubkey type = %T, want tmed25519.PubKeyEd25519", pk)
	require.Equal(t, string(kp.Pub), string(tmpk[:]), "decoded pubkey bytes do not match original")
}

func TestAddress_format(t *testing.T) {
	kp, err := NewKeypair()
	require.NoError(t, err)

	addr := kp.Address()
	require.True(t, len(addr) >= 2 && addr[:2] == "g1",
		"Address() = %q, want prefix \"g1\"", addr)

	hrp, addrBytes, err := bech32.Decode(addr)
	require.NoError(t, err, "bech32.Decode(%q)", addr)
	require.Equal(t, "g", hrp, "address hrp")
	require.Equal(t, addrSize, len(addrBytes), "address byte length")
}

func TestKeypair_Sign_verifiable(t *testing.T) {
	kp, err := NewKeypair()
	require.NoError(t, err)

	payload := []byte("gnomcp-test-payload")
	sig, err := kp.Sign(payload)
	require.NoError(t, err)
	require.True(t, ed25519.Verify(ed25519.PublicKey(kp.Pub), payload, sig),
		"ed25519.Verify returned false for a freshly generated signature")
}

// TestKeypair_GnoclientSigner_signsTxWithSessionPubkey verifies that the
// adapter signs std.Tx sign-bytes with the session keypair and attaches the
// session pubkey to the resulting Signature. SessionAddr is intentionally
// left zero — chain.Real.Call injects it after Sign returns.
func TestKeypair_GnoclientSigner_signsTxWithSessionPubkey(t *testing.T) {
	kp, err := NewKeypair()
	require.NoError(t, err)

	const chainID = "test-chain"
	signer := kp.GnoclientSigner(chainID)
	require.NotNil(t, signer, "GnoclientSigner returned nil")

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
	require.NoError(t, err)
	require.Len(t, signedTx.Signatures, 1)

	sig := signedTx.Signatures[0]
	require.NotNil(t, sig.PubKey, "Signature.PubKey is nil")

	pk, ok := sig.PubKey.(tmed25519.PubKeyEd25519)
	require.True(t, ok, "Signature.PubKey type = %T, want tmed25519.PubKeyEd25519", sig.PubKey)
	assert.Equal(t, string(kp.Pub), string(pk[:]), "Signature.PubKey bytes do not match keypair.Pub")
	assert.True(t, sig.SessionAddr.IsZero(),
		"Signature.SessionAddr should be zero (caller fills it); got %s", sig.SessionAddr)

	signBytes, err := tx.GetSignBytes(chainID, accNum, seq)
	require.NoError(t, err)
	require.True(t, ed25519.Verify(ed25519.PublicKey(kp.Pub), signBytes, sig.Signature),
		"ed25519.Verify returned false for session-signed tx")
}

// TestKeypair_GnoclientSigner_Info exposes the session pubkey via the
// gnoclient.Signer.Info() contract. Callers don't usually need it (Real.Call
// uses chain.Signer directly), but the interface requires it.
func TestKeypair_GnoclientSigner_Info(t *testing.T) {
	kp, err := NewKeypair()
	require.NoError(t, err)

	signer := kp.GnoclientSigner("test-chain")
	info, err := signer.Info()
	require.NoError(t, err)
	require.NotNil(t, info.GetPubKey(), "Info().GetPubKey() is nil")

	pk, ok := info.GetPubKey().(tmed25519.PubKeyEd25519)
	require.True(t, ok, "PubKey type = %T", info.GetPubKey())
	assert.Equal(t, string(kp.Pub), string(pk[:]), "Info().GetPubKey() bytes do not match keypair.Pub")
}

// TestKeypair_GnoclientSigner_Validate verifies the signer reports itself valid.
func TestKeypair_GnoclientSigner_Validate(t *testing.T) {
	kp, err := NewKeypair()
	require.NoError(t, err)
	assert.NoError(t, kp.GnoclientSigner("test-chain").Validate())
}
