package profiles

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteFile_AtomicAnd0600(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.toml")
	m := map[string]Profile{
		"test11": {RPCURL: "https://rpc.test11.testnets.gno.land:443", ChainID: "test11"},
	}
	err := WriteFile(path, m)
	require.NoError(t, err, "write")

	info, err := os.Stat(path)
	require.NoError(t, err, "stat")
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "file mode")

	// Round-trips back through Load+Validate.
	f, _ := os.Open(path)
	defer f.Close()
	cfg, err := Load(f)
	require.NoError(t, err, "reload")
	_, err = cfg.Validate()
	require.NoError(t, err, "written file must validate")
}

func TestWriteFile_RejectsForbiddenChain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.toml")
	m := map[string]Profile{"bad": {RPCURL: "x", ChainID: "gnoland1"}}
	err := WriteFile(path, m)
	require.Error(t, err, "writing a forbidden chain-id must be rejected")
}

// TestWriteFile_EmptyAllowed guards that removing the last user profile (empty
// map) is permitted — the built-in defaults still apply, so an empty global
// file is legitimate. Previously WriteFile→Validate rejected it ("no profiles").
func TestWriteFile_EmptyAllowed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.toml")
	err := WriteFile(path, map[string]Profile{})
	require.NoError(t, err, "writing an empty profile set must be allowed")

	f, err := os.Open(path)
	require.NoError(t, err, "open")
	defer f.Close()

	cfg, err := Load(f)
	require.NoError(t, err, "reload empty")
	assert.Empty(t, cfg.Profiles, "expected empty profile set")
}
