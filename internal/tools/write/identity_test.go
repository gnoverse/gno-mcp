package write

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSignedByLine_localAgent_namesTest1(t *testing.T) {
	got := signedByLine("agent", "g1abc", "", true)
	assert.Contains(t, got, "test1", "local agent line should name test1")
	assert.Contains(t, got, "g1abc", "line should contain the signer address")
}

func TestSignedByLine_testnetAgent_doesNotNameTest1(t *testing.T) {
	got := signedByLine("agent", "g1xyz", "", false)
	assert.NotContains(t, got, "test1", "testnet agent uses a generated key, not test1")
	assert.Contains(t, got, "g1xyz", "line should contain the signer address")
}

func TestSignedByLine_session(t *testing.T) {
	got := signedByLine("session", "g1sess", "g1master", false)
	assert.Contains(t, got, "session")
	assert.Contains(t, got, "g1master")
}
