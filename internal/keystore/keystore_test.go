package keystore

import (
	"testing"

	"github.com/gnoverse/gno-mcp/internal/profiles"
)

func devProfile() profiles.Profile {
	return profiles.Profile{ChainID: "dev", ChainType: profiles.ChainTypeLocal, RPCURL: "http://127.0.0.1:26657"}
}
func testnetProfile() profiles.Profile {
	return profiles.Profile{ChainID: "test11", ChainType: profiles.ChainTypeTestnet, RPCURL: "https://example:443"}
}

func TestAgentAddress_dev_isTest1(t *testing.T) {
	got, err := New().AgentAddress(devProfile())
	if err != nil {
		t.Fatalf("AgentAddress dev: %v", err)
	}
	const want = "g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5"
	if got != want {
		t.Fatalf("test1 address = %q, want %q", got, want)
	}
}

func TestSignerForProfile_testnet_noAgentKey(t *testing.T) {
	if _, err := New().SignerForProfile(testnetProfile()); err == nil {
		t.Fatal("expected ErrNoAgentKey for testnet in Plan A, got nil")
	}
}
