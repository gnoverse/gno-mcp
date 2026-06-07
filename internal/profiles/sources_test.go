package profiles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadResolved_MissingFilesUsesDefaults(t *testing.T) {
	cfg, err := LoadResolved(Sources{
		GlobalPath:  filepath.Join(t.TempDir(), "nope.toml"),
		ProjectPath: filepath.Join(t.TempDir(), "also-nope.toml"),
	})
	if err != nil {
		t.Fatalf("missing files must not be fatal: %v", err)
	}
	if _, ok := cfg.Profiles["testnet"]; !ok {
		t.Error("expected built-in testnet default when no files present")
	}
}

func TestLoadResolved_ExplicitFileLayers(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "profiles.toml")
	if err := os.WriteFile(p, []byte(`
[testnet]
rpc-url = "https://rpc.test11.testnets.gno.land:443"
chain-id = "test11"
master-address = "g17ernafy6ctpcz6uepfsq2js8x2vz0wladh5yc3"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadResolved(Sources{ExplicitPath: p})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Profiles["testnet"].MasterAddress == "" {
		t.Error("explicit file should override the testnet default")
	}
}
