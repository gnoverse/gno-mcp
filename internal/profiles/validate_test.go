package profiles

import (
	"strings"
	"testing"
)

func TestValidate_requiresRPCURL(t *testing.T) {
	cfg, err := Load(strings.NewReader(`
[local]
chain-id = "dev"
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for missing rpc-url")
	}
}

func TestValidate_requiresChainID(t *testing.T) {
	cfg, err := Load(strings.NewReader(`
[local]
rpc-url = "http://127.0.0.1:26657"
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for missing chain-id")
	}
}

func TestValidate_chainTypeDefaultsToTestnet(t *testing.T) {
	cfg, err := Load(strings.NewReader(`
[mystery]
rpc-url = "https://rpc.example/"
chain-id = "x"
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if got := cfg.Profiles["mystery"].ChainType; got != "testnet" {
		t.Errorf("expected chain-type=testnet default, got %q", got)
	}
}

func TestValidate_rejectsEmptyProfileSet(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for empty profile set")
	}
}

func TestValidate_rejectsUnknownChainType(t *testing.T) {
	cfg, err := Load(strings.NewReader(`
[weird]
chain-type = "moonchain"
rpc-url = "http://x"
chain-id = "x"
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for unknown chain-type")
	}
}

func TestLoad_rejectsUnknownKey(t *testing.T) {
	src := `
[local]
rpc-url = "http://127.0.0.1:26657"
chain-id = "dev"
allow-dangerous-tools = true
`
	_, err := Load(strings.NewReader(src))
	if err == nil {
		t.Fatal("expected error for unknown key allow-dangerous-tools, got nil")
	}
	// Must mention the offending key so users can fix their config.
	if !strings.Contains(err.Error(), "allow-dangerous-tools") {
		t.Errorf("error %q should mention the offending key", err)
	}
}
