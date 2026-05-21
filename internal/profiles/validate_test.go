package profiles

import (
	"bytes"
	"io"
	"os"
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
	err = cfg.Validate()
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
	cfg, err := Load(strings.NewReader(`
[local]
rpc-url = "http://127.0.0.1:26657"
chain-id = "dev"
default-spend-limit = "abc"
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for malformed default-spend-limit")
	}
}

func TestValidate_rejectsMalformedDefaultSpendLimitNoMagnitude(t *testing.T) {
	cfg, err := Load(strings.NewReader(`
[local]
rpc-url = "http://127.0.0.1:26657"
chain-id = "dev"
default-spend-limit = "ugnot"
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for spend-limit missing numeric magnitude")
	}
}

func TestValidate_rejectsMalformedDefaultSpendLimitNoDenom(t *testing.T) {
	cfg, err := Load(strings.NewReader(`
[local]
rpc-url = "http://127.0.0.1:26657"
chain-id = "dev"
default-spend-limit = "100"
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for spend-limit missing denomination")
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
	err = cfg.Validate()
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

	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("os.Pipe: %v", pipeErr)
	}
	old := os.Stderr
	os.Stderr = w

	validateErr := cfg.Validate()

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	if _, copyErr := io.Copy(&buf, r); copyErr != nil {
		t.Fatalf("io.Copy: %v", copyErr)
	}
	r.Close()

	if validateErr != nil {
		t.Fatalf("Validate returned unexpected error: %v", validateErr)
	}
	if !strings.Contains(buf.String(), "mainnet with allow-dangerous-tools") {
		t.Errorf("expected mainnet warning on stderr, got: %q", buf.String())
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
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: unexpected error: %v", err)
	}
}
