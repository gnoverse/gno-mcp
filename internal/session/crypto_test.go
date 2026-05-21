package session

import (
	"bytes"
	"testing"
)

func TestEncrypt_roundTrip(t *testing.T) {
	plaintext := []byte("super-secret-privkey-bytes")
	passphrase := "correct-horse-battery-staple"

	ciphertext, err := Encrypt(plaintext, passphrase)
	if err != nil {
		t.Fatalf("Encrypt(): %v", err)
	}
	if bytes.Equal(ciphertext, plaintext) {
		t.Fatal("Encrypt() returned plaintext unchanged (expected ciphertext)")
	}

	got, err := Decrypt(ciphertext, passphrase)
	if err != nil {
		t.Fatalf("Decrypt(): %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("Decrypt() = %q, want %q", got, plaintext)
	}
}

func TestEncrypt_wrongPassphraseRejected(t *testing.T) {
	plaintext := []byte("my-private-key")
	ct, err := Encrypt(plaintext, "correct-pass")
	if err != nil {
		t.Fatalf("Encrypt(): %v", err)
	}
	_, err = Decrypt(ct, "wrong-pass")
	if err == nil {
		t.Fatal("Decrypt() with wrong passphrase returned nil error, want error")
	}
}

func TestEncrypt_tamperedCiphertextRejected(t *testing.T) {
	plaintext := []byte("my-private-key")
	ct, err := Encrypt(plaintext, "pass")
	if err != nil {
		t.Fatalf("Encrypt(): %v", err)
	}
	tampered := make([]byte, len(ct))
	copy(tampered, ct)
	tampered[saltLen+nonceLen+1] ^= 0xFF

	_, err = Decrypt(tampered, "pass")
	if err == nil {
		t.Fatal("Decrypt() tampered ciphertext returned nil error, want GCM auth failure")
	}
}

func TestEncrypt_emptyPassphrasePassthrough(t *testing.T) {
	plaintext := []byte("visible-bytes")

	ct, err := Encrypt(plaintext, "")
	if err != nil {
		t.Fatalf("Encrypt(empty passphrase): %v", err)
	}
	if !bytes.Equal(ct, plaintext) {
		t.Fatal("Encrypt with empty passphrase should return plaintext unchanged")
	}

	got, err := Decrypt(ct, "")
	if err != nil {
		t.Fatalf("Decrypt(empty passphrase): %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatal("Decrypt with empty passphrase should return input unchanged")
	}
}
