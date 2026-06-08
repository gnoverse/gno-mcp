package write

import (
	"strings"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/profiles"
)

func TestSignedByLine_localAgent_namesTest1(t *testing.T) {
	got := signedByLine("agent", "g1abc", "", profiles.ChainTypeLocal)
	if !strings.Contains(got, "test1") {
		t.Fatalf("local agent line should name test1, got %q", got)
	}
	if !strings.Contains(got, "g1abc") {
		t.Fatalf("line should contain the signer address, got %q", got)
	}
}

func TestSignedByLine_testnetAgent_doesNotNameTest1(t *testing.T) {
	got := signedByLine("agent", "g1xyz", "", profiles.ChainTypeTestnet)
	if strings.Contains(got, "test1") {
		t.Fatalf("testnet agent uses a generated key, not test1, got %q", got)
	}
	if !strings.Contains(got, "g1xyz") {
		t.Fatalf("line should contain the signer address, got %q", got)
	}
}

func TestSignedByLine_session(t *testing.T) {
	got := signedByLine("session", "g1sess", "g1master", profiles.ChainTypeTestnet)
	if !strings.Contains(got, "session") || !strings.Contains(got, "g1master") {
		t.Fatalf("session line should name the session and master, got %q", got)
	}
}
