package profiles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFile_AtomicAnd0600(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.toml")
	m := map[string]Profile{
		"test11": {ChainType: ChainTypeTestnet, RPCURL: "https://rpc.test11.testnets.gno.land:443", ChainID: "test11"},
	}
	if err := WriteFile(path, m); err != nil {
		t.Fatalf("write: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", info.Mode().Perm())
	}
	// Round-trips back through Load+Validate.
	f, _ := os.Open(path)
	defer f.Close()
	cfg, err := Load(f)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if _, err := cfg.Validate(); err != nil {
		t.Fatalf("written file must validate: %v", err)
	}
}

func TestWriteFile_RejectsForbiddenChain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.toml")
	m := map[string]Profile{"bad": {RPCURL: "x", ChainID: "gnoland1"}}
	if err := WriteFile(path, m); err == nil {
		t.Fatal("writing a forbidden chain-id must be rejected")
	}
}

// TestWriteFile_EmptyAllowed guards that removing the last user profile (empty
// map) is permitted — the built-in defaults still apply, so an empty global
// file is legitimate. Previously WriteFile→Validate rejected it ("no profiles").
func TestWriteFile_EmptyAllowed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.toml")
	if err := WriteFile(path, map[string]Profile{}); err != nil {
		t.Fatalf("writing an empty profile set must be allowed: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	cfg, err := Load(f)
	if err != nil {
		t.Fatalf("reload empty: %v", err)
	}
	if len(cfg.Profiles) != 0 {
		t.Errorf("expected empty profile set, got %d", len(cfg.Profiles))
	}
}
