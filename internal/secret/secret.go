package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/scrypt"
)

const (
	saltLen   = 16
	nonceLen  = 12
	keyLen    = 32
	gcmTagLen = 16

	scryptN = 32768
	scryptR = 8
	scryptP = 1
)

// Scheme tags prefix every blob so Decrypt knows how the bytes were produced
// and a passphrase/format mismatch fails loudly instead of mis-decoding.
const (
	schemePlain  = 0x00 // no passphrase: tag || plaintext
	schemeScrypt = 0x01 // tag || salt(16) || nonce(12) || ciphertext+authTag
)

// ErrPassphraseRequired reports that the blob was encrypted with a passphrase
// but Decrypt was called with an empty one (e.g. GNOMCP_SESSION_PASSPHRASE was
// set when the data was written but is unset now).
var ErrPassphraseRequired = errors.New("secret: data is encrypted but no passphrase is set (set GNOMCP_SESSION_PASSPHRASE)")

// Encrypt returns a scheme-tagged blob. With a passphrase the body is
// salt(16) || nonce(12) || ciphertext+authTag (AES-256-GCM, scrypt-derived
// key); with an empty passphrase the body is the plaintext (opt-in encryption).
func Encrypt(plaintext []byte, passphrase string) ([]byte, error) {
	if passphrase == "" {
		return append([]byte{schemePlain}, plaintext...), nil
	}

	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("secret: generate salt: %w", err)
	}

	key, err := deriveKey(passphrase, salt)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("secret: new AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secret: new GCM: %w", err)
	}

	nonce := make([]byte, nonceLen)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("secret: generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	out := make([]byte, 0, 1+saltLen+nonceLen+len(ciphertext))
	out = append(out, schemeScrypt)
	out = append(out, salt...)
	out = append(out, nonce...)
	out = append(out, ciphertext...)
	return out, nil
}

// Decrypt reverses Encrypt, dispatching on the scheme tag. A scrypt blob with
// an empty passphrase returns ErrPassphraseRequired rather than mis-decoding.
func Decrypt(blob []byte, passphrase string) ([]byte, error) {
	if len(blob) == 0 {
		return nil, errors.New("secret: empty input")
	}
	scheme, body := blob[0], blob[1:]
	switch scheme {
	case schemePlain:
		return body, nil
	case schemeScrypt:
		// fall through to the decrypt path below
	default:
		return nil, fmt.Errorf("secret: unknown encryption scheme 0x%02x (file written by an incompatible gnomcp version?) — delete the file and regenerate the key or session", scheme)
	}

	if passphrase == "" {
		return nil, ErrPassphraseRequired
	}
	minLen := saltLen + nonceLen + gcmTagLen
	if len(body) < minLen {
		return nil, errors.New("secret: ciphertext too short")
	}

	salt := body[:saltLen]
	nonce := body[saltLen : saltLen+nonceLen]
	data := body[saltLen+nonceLen:]

	key, err := deriveKey(passphrase, salt)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("secret: new AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secret: new GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return nil, fmt.Errorf("secret: decrypt: %w", err)
	}
	return plaintext, nil
}

func deriveKey(passphrase string, salt []byte) ([]byte, error) {
	key, err := scrypt.Key([]byte(passphrase), salt, scryptN, scryptR, scryptP, keyLen)
	if err != nil {
		return nil, fmt.Errorf("secret: scrypt key derivation: %w", err)
	}
	return key, nil
}
