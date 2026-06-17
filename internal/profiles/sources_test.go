package profiles

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadResolved_MissingFilesUsesDefaults(t *testing.T) {
	cfg, err := LoadResolved(Sources{
		GlobalPath:  filepath.Join(t.TempDir(), "nope.toml"),
		ProjectPath: filepath.Join(t.TempDir(), "also-nope.toml"),
	})
	require.NoError(t, err, "missing files must not be fatal")
	assert.Contains(t, cfg.Profiles, "testnet", "expected built-in testnet default when no files present")
}

func TestLoadResolved_ExplicitFileLayers(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "profiles.toml")
	err := os.WriteFile(p, []byte(`
[testnet]
rpc-url = "https://rpc.test13.testnets.gno.land:443"
chain-id = "test-13"
master-address = "g17ernafy6ctpcz6uepfsq2js8x2vz0wladh5yc3"
`), 0o600)
	require.NoError(t, err)

	cfg, err := LoadResolved(Sources{ExplicitPath: p})
	require.NoError(t, err)
	assert.NotEmpty(t, cfg.Profiles["testnet"].MasterAddress, "explicit file should override the testnet default")
}
