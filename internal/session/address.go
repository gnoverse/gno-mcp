package session

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"strings"
)

// Address derivation for the MCP session account.
//
// TODO(v0.3): swap this stub for github.com/gnolang/gno/tm2/pkg/crypto/keys
// once gnopie ships as a library. Real gno addresses are secp256k1 + bech32
// with the "g" HRP; the on-the-wire shape is identical, so the auth flow
// (fund this address, then we sign) remains correct under either backend.
const (
	// HRP is "gmcp" rather than "g" so a session address can never be confused
	// with a real user wallet during the v0.2 demo. The prefix changes back to
	// "g" once we wire tm2 keys.
	addrHRP = "gmcp"
)

// addressFromPub returns a bech32-encoded short address derived from the
// public key: SHA-256(pub)[:20], encoded with HRP "gmcp".
func addressFromPub(pub ed25519.PublicKey) string {
	h := sha256.Sum256(pub)
	return bech32Encode(addrHRP, h[:20])
}

// --- minimal bech32 (BIP-173) encoder, encoding only ---------------------

const bech32Charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

func bech32Encode(hrp string, data []byte) string {
	conv, err := convertBits(data, 8, 5, true)
	if err != nil {
		return ""
	}
	checksum := createChecksum(hrp, conv)
	combined := append(conv, checksum...)
	var sb strings.Builder
	sb.WriteString(hrp)
	sb.WriteByte('1')
	for _, c := range combined {
		sb.WriteByte(bech32Charset[c])
	}
	return sb.String()
}

func convertBits(data []byte, fromBits, toBits uint, pad bool) ([]byte, error) {
	acc := 0
	bits := uint(0)
	out := make([]byte, 0, len(data)*int(fromBits)/int(toBits)+1)
	maxv := (1 << toBits) - 1
	for _, b := range data {
		if int(b) < 0 || int(b)>>fromBits != 0 {
			return nil, errors.New("invalid byte for conversion")
		}
		acc = (acc << fromBits) | int(b)
		bits += fromBits
		for bits >= toBits {
			bits -= toBits
			out = append(out, byte((acc>>bits)&maxv))
		}
	}
	if pad {
		if bits > 0 {
			out = append(out, byte((acc<<(toBits-bits))&maxv))
		}
	} else if bits >= fromBits || (acc<<(toBits-bits))&maxv != 0 {
		return nil, errors.New("invalid conversion padding")
	}
	return out, nil
}

func createChecksum(hrp string, data []byte) []byte {
	values := append(hrpExpand(hrp), data...)
	values = append(values, 0, 0, 0, 0, 0, 0)
	polymod := bech32Polymod(values) ^ 1
	out := make([]byte, 6)
	for i := 0; i < 6; i++ {
		out[i] = byte((polymod >> uint(5*(5-i))) & 31)
	}
	return out
}

func hrpExpand(hrp string) []byte {
	out := make([]byte, 0, len(hrp)*2+1)
	for i := 0; i < len(hrp); i++ {
		out = append(out, hrp[i]>>5)
	}
	out = append(out, 0)
	for i := 0; i < len(hrp); i++ {
		out = append(out, hrp[i]&31)
	}
	return out
}

func bech32Polymod(values []byte) int {
	gen := [5]int{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}
	chk := 1
	for _, v := range values {
		b := chk >> 25
		chk = ((chk & 0x1ffffff) << 5) ^ int(v)
		for i := 0; i < 5; i++ {
			if (b>>uint(i))&1 == 1 {
				chk ^= gen[i]
			}
		}
	}
	return chk
}
