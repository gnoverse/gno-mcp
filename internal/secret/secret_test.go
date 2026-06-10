package secret

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncrypt_roundTrip(t *testing.T) {
	plaintext := []byte("super-secret-privkey-bytes")
	passphrase := "correct-horse-battery-staple"

	ciphertext, err := Encrypt(plaintext, passphrase)
	require.NoError(t, err, "Encrypt()")
	// Keep raw bytes check: ciphertext must differ from plaintext (encryption happened).
	require.False(t, bytes.Equal(ciphertext, plaintext), "Encrypt() returned plaintext unchanged (expected ciphertext)")

	got, err := Decrypt(ciphertext, passphrase)
	require.NoError(t, err, "Decrypt()")
	require.True(t, bytes.Equal(got, plaintext), "Decrypt() = %q, want %q", got, plaintext)
}

func TestEncrypt_wrongPassphraseRejected(t *testing.T) {
	plaintext := []byte("my-private-key")
	ct, err := Encrypt(plaintext, "correct-pass")
	require.NoError(t, err, "Encrypt()")
	_, err = Decrypt(ct, "wrong-pass")
	require.Error(t, err, "Decrypt() with wrong passphrase returned nil error, want error")
}

func TestEncrypt_tamperedCiphertextRejected(t *testing.T) {
	plaintext := []byte("my-private-key")
	ct, err := Encrypt(plaintext, "pass")
	require.NoError(t, err, "Encrypt()")
	tampered := make([]byte, len(ct))
	copy(tampered, ct)
	tampered[len(ct)-1] ^= 0xFF // flip the last auth-tag byte

	_, err = Decrypt(tampered, "pass")
	require.Error(t, err, "Decrypt() tampered ciphertext returned nil error, want GCM auth failure")
}

func TestDecrypt_encryptedWithEmptyPassphrase_errors(t *testing.T) {
	ct, err := Encrypt([]byte("super-secret-privkey"), "pass")
	require.NoError(t, err, "Encrypt()")
	// Passphrase later unset (operator forgot to re-export it): Decrypt must fail
	// loudly, never hand back the ciphertext as if it were plaintext.
	_, err = Decrypt(ct, "")
	require.Error(t, err, "Decrypt(encrypted, \"\") must fail, not return ciphertext unchanged")
}

func TestDecrypt_legacyUntaggedBlob_actionableError(t *testing.T) {
	// A file written before the scheme tag existed starts with arbitrary data
	// (here 's' from a plaintext mnemonic = scheme 0x73). The error must tell
	// the operator what to do, not just report an unknown byte.
	legacy := []byte("scheme-less legacy mnemonic bytes")
	_, err := Decrypt(legacy, "")
	require.Error(t, err, "legacy untagged blob must be rejected")
	require.Contains(t, err.Error(), "regenerate",
		"the unknown-scheme error must tell the operator to delete the file and regenerate")
}

func TestEncrypt_emptyPassphraseNotEncrypted(t *testing.T) {
	plaintext := []byte("visible-bytes")

	ct, err := Encrypt(plaintext, "")
	require.NoError(t, err, "Encrypt(empty passphrase)")
	// Opt-in encryption: with no passphrase only a scheme tag is prepended, so the
	// plaintext is still visible in the blob (nothing is encrypted).
	require.True(t, bytes.Contains(ct, plaintext), "empty-passphrase blob should carry plaintext unencrypted")

	got, err := Decrypt(ct, "")
	require.NoError(t, err, "Decrypt(empty passphrase)")
	require.True(t, bytes.Equal(got, plaintext), "round-trip with empty passphrase should recover input")
}
