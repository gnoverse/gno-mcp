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
	tampered[saltLen+nonceLen+1] ^= 0xFF

	_, err = Decrypt(tampered, "pass")
	require.Error(t, err, "Decrypt() tampered ciphertext returned nil error, want GCM auth failure")
}

func TestEncrypt_emptyPassphrasePassthrough(t *testing.T) {
	plaintext := []byte("visible-bytes")

	ct, err := Encrypt(plaintext, "")
	require.NoError(t, err, "Encrypt(empty passphrase)")
	// Keep raw bytes check: empty-passphrase path must return plaintext unchanged.
	require.True(t, bytes.Equal(ct, plaintext), "Encrypt with empty passphrase should return plaintext unchanged")

	got, err := Decrypt(ct, "")
	require.NoError(t, err, "Decrypt(empty passphrase)")
	require.True(t, bytes.Equal(got, plaintext), "Decrypt with empty passphrase should return input unchanged")
}
