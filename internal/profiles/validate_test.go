package profiles

import (
	"fmt"
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
	if _, err := cfg.Validate(); err == nil {
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
	if _, err := cfg.Validate(); err == nil {
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
	if _, err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if got := cfg.Profiles["mystery"].ChainType; got != "testnet" {
		t.Errorf("expected chain-type=testnet default, got %q", got)
	}
}

func TestValidate_rejectsEmptyProfileSet(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{}}
	if _, err := cfg.Validate(); err == nil {
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
	if _, err := cfg.Validate(); err == nil {
		t.Fatal("expected error for unknown chain-type")
	}
}

func TestLoad_rejectsUnknownKey(t *testing.T) {
	src := `
[local]
rpc-url = "http://127.0.0.1:26657"
chain-id = "dev"
foo-bar = true
`
	_, err := Load(strings.NewReader(src))
	if err == nil {
		t.Fatal("expected error for unknown key foo-bar, got nil")
	}
	// Must mention the offending key so users can fix their config.
	if !strings.Contains(err.Error(), "foo-bar") {
		t.Errorf("error %q should mention the offending key", err)
	}
}

func TestValidate_rejectsMalformedDefaultExpiresIn(t *testing.T) {
	cfg, err := Load(strings.NewReader(`
[local]
rpc-url = "http://127.0.0.1:26657"
chain-id = "dev"
default-expires-in = "forever"
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	_, err = cfg.Validate()
	if err == nil {
		t.Fatal("expected error for malformed default-expires-in")
	}
	if !strings.Contains(err.Error(), "default-expires-in") {
		t.Errorf("error %q should mention field name", err)
	}
	if !strings.Contains(err.Error(), "forever") {
		t.Errorf("error %q should mention offending value", err)
	}
}

func TestValidate_rejectsMalformedDefaultSpendLimit(t *testing.T) {
	cases := map[string]string{
		"letters only (no magnitude)": "abc",
		"denom only (no magnitude)":   "ugnot",
		"digits only (no denom)":      "100",
	}
	for name, val := range cases {
		t.Run(name, func(t *testing.T) {
			cfg, err := Load(strings.NewReader(fmt.Sprintf(`
[local]
rpc-url = "http://127.0.0.1:26657"
chain-id = "dev"
default-spend-limit = %q
`, val)))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			_, err = cfg.Validate()
			if err == nil {
				t.Fatalf("expected error for spend-limit %q", val)
			}
			if !strings.Contains(err.Error(), "default-spend-limit") {
				t.Errorf("error %q should mention field name", err)
			}
		})
	}
}

func TestValidate_rejectsBypassWithoutDangerous(t *testing.T) {
	cfg, err := Load(strings.NewReader(`
[local]
rpc-url = "http://127.0.0.1:26657"
chain-id = "dev"
bypass-hard-limits = true
allow-dangerous-tools = false
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	_, err = cfg.Validate()
	if err == nil {
		t.Fatal("expected error for bypass-hard-limits without allow-dangerous-tools")
	}
	if !strings.Contains(err.Error(), "bypass-hard-limits requires allow-dangerous-tools") {
		t.Errorf("error %q should explain the dependency", err)
	}
}

func TestValidate_mainnetDangerousEmitsWarning(t *testing.T) {
	cfg, err := Load(strings.NewReader(`
[prod]
chain-type = "mainnet"
rpc-url = "https://rpc.gno.land:443"
chain-id = "portal-loop"
allow-dangerous-tools = true
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	warn, validateErr := cfg.Validate()

	if validateErr != nil {
		t.Fatalf("Validate returned unexpected error: %v", validateErr)
	}
	if warn == nil {
		t.Fatal("expected non-nil warning for mainnet+allow-dangerous-tools")
	}
	if !strings.Contains(warn.Error(), "mainnet with allow-dangerous-tools") {
		t.Errorf("warning %q should mention mainnet with allow-dangerous-tools", warn)
	}
}

func TestValidate_acceptsValidExpiresIn(t *testing.T) {
	cases := []string{"0s", "500ms", "2h", "72h30m", "168h"}
	for _, val := range cases {
		t.Run(val, func(t *testing.T) {
			cfg, err := Load(strings.NewReader(fmt.Sprintf(`
[local]
rpc-url = "http://127.0.0.1:26657"
chain-id = "dev"
default-expires-in = %q
`, val)))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if _, err := cfg.Validate(); err != nil {
				t.Errorf("Validate rejected valid duration %q: %v", val, err)
			}
		})
	}
}

func TestValidate_acceptsValidWriteFields(t *testing.T) {
	cfg, err := Load(strings.NewReader(`
[local]
chain-type = "local"
rpc-url = "http://127.0.0.1:26657"
chain-id = "dev"
allow-dangerous-tools = true
default-spend-limit = "500000ugnot"
default-expires-in = "2h"
bypass-hard-limits = true
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: unexpected error: %v", err)
	}
}
