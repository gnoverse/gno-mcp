package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProfileAddManual_PersistsAndValidates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.toml")
	err := profileAdd(path, "test-13", profileAddOpts{
		RPC: "https://rpc.test13.testnets.gno.land:443", ChainID: "test-13",
	})
	require.NoError(t, err, "add")
	f, _ := os.Open(path)
	defer f.Close()
	cfg, err := profiles.Load(f)
	require.NoError(t, err, "load")
	assert.NotEmpty(t, cfg.Profiles["test-13"].RPCURL, "profile not persisted")
}

func TestProfileAdd_ReservedName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.toml")
	err := profileAdd(path, "local", profileAddOpts{RPC: "http://127.0.0.1:26657", ChainID: "dev"})
	require.Error(t, err, "expected 'local' to be a reserved name")
}

// TestParseProfileAddArgs_NameBeforeFlags regression-guards the bug where Go's
// flag parser stopped at the positional <name> and silently dropped every flag
// after it (so `add <name> --rpc ... --chain-id ...` lost rpc/chain-id).
func TestParseProfileAddArgs_NameBeforeFlags(t *testing.T) {
	name, o, err := parseProfileAddArgs([]string{"mychain", "--rpc", "https://rpc.test13.testnets.gno.land:443", "--chain-id", "test-13"})
	require.NoError(t, err, "parse")
	assert.Equal(t, "mychain", name)
	assert.Equal(t, "https://rpc.test13.testnets.gno.land:443", o.RPC, "flags after the name were dropped")
	assert.Equal(t, "test-13", o.ChainID)
}

func TestParseProfileAddArgs_FromGnowebAndMaster(t *testing.T) {
	name, o, err := parseProfileAddArgs([]string{"foo", "--from-gnoweb", "https://test13.testnets.gno.land", "--master", "g1abc"})
	require.NoError(t, err, "parse")
	assert.Equal(t, "foo", name)
	assert.Equal(t, "https://test13.testnets.gno.land", o.FromGnoweb)
	assert.Equal(t, "g1abc", o.Master)
}

func TestParseProfileAddArgs_GnowebURL(t *testing.T) {
	name, o, err := parseProfileAddArgs([]string{"foo", "--rpc", "https://rpc.test13.testnets.gno.land:443", "--chain-id", "test-13", "--gnoweb-url", "https://test13.testnets.gno.land"})
	require.NoError(t, err, "parse")
	assert.Equal(t, "foo", name)
	assert.Equal(t, "https://test13.testnets.gno.land", o.GnowebURL)
}

func TestProfileAddFromGnoweb_PersistsGnowebURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<meta name="gnoconnect:rpc" content="https://rpc.test13.testnets.gno.land:443" />`+
			`<meta name="gnoconnect:chainid" content="test-13" />`)
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "profiles.toml")
	err := profileAdd(path, "test-13", profileAddOpts{FromGnoweb: srv.URL + "/r/demo/counter"})
	require.NoError(t, err, "add")
	f, _ := os.Open(path)
	defer f.Close()
	cfg, err := profiles.Load(f)
	require.NoError(t, err, "load")
	assert.Equal(t, srv.URL, cfg.Profiles["test-13"].GnowebURL)
}

func TestParseProfileAddArgs_MissingName(t *testing.T) {
	_, _, err := parseProfileAddArgs([]string{"--rpc", "x"})
	require.Error(t, err, "expected error when no name precedes the flags")
}
