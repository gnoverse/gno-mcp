package write

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/keystore"
	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/gnoverse/gno-mcp/internal/server"
)

func TestKeyGenerate_testnetProfile_returnsAddress(t *testing.T) {
	s := newTestnetTestServer(t)
	ks := keystore.New(t.TempDir(), "")
	RegisterKeyGenerate(s, ks)

	res, err := s.Registry().Call(context.Background(), "gno_key_generate", map[string]any{
		"profile": "testnet9999",
	})
	require.NoError(t, err, "gno_key_generate")
	assert.True(t, strings.HasPrefix(res.Text, "g1"), "expected g1... address, got %q", res.Text)
	require.NotNil(t, res.StructuredContent)
	addr, _ := res.StructuredContent["address"].(string)
	assert.Equal(t, res.Text, addr)
}

func TestKeyGenerate_secondCall_keyAlreadyExists(t *testing.T) {
	s := newTestnetTestServer(t)
	ks := keystore.New(t.TempDir(), "")
	RegisterKeyGenerate(s, ks)

	_, err := s.Registry().Call(context.Background(), "gno_key_generate", map[string]any{
		"profile": "testnet9999",
	})
	require.NoError(t, err, "first call")

	_, err = s.Registry().Call(context.Background(), "gno_key_generate", map[string]any{
		"profile": "testnet9999",
	})
	require.Error(t, err, "expected key_already_exists error, got nil")
	var te *server.ToolError
	require.ErrorAs(t, err, &te)
	assert.Equal(t, "key_already_exists", te.Code)
}

func TestKeyGenerate_noKeyDir_keyStorageUnconfigured(t *testing.T) {
	s := newTestnetTestServer(t)
	ks := keystore.New("", "")
	RegisterKeyGenerate(s, ks)

	_, err := s.Registry().Call(context.Background(), "gno_key_generate", map[string]any{
		"profile": "testnet9999",
	})
	require.Error(t, err, "expected key_storage_unconfigured error, got nil")
	var te *server.ToolError
	require.ErrorAs(t, err, &te)
	assert.Equal(t, "key_storage_unconfigured", te.Code)
}

func TestKeyGenerate_localProfile_keyGenerationUnsupported(t *testing.T) {
	s := newLocalTestServerWithTestnet(t)
	ks := keystore.New(t.TempDir(), "")
	RegisterKeyGenerate(s, ks)

	_, err := s.Registry().Call(context.Background(), "gno_key_generate", map[string]any{
		"profile": "local",
	})
	require.Error(t, err, "expected key_generation_unsupported error, got nil")
	var te *server.ToolError
	require.ErrorAs(t, err, &te)
	assert.Equal(t, "key_generation_unsupported", te.Code)
}

// testnet9999Profile is the testnet profile fixture shared by the key-generate
// server builders.
func testnet9999Profile() profiles.Profile {
	return profiles.Profile{RPCURL: "http://127.0.0.1:26657", ChainID: "test9999"}
}

// newTestnetServerFromProfiles validates ps and wraps it in a Server.
func newTestnetServerFromProfiles(t *testing.T, ps map[string]profiles.Profile) *server.Server {
	t.Helper()
	cfg := &profiles.Config{Profiles: ps}
	_, err := cfg.Validate()
	require.NoError(t, err, "validate")
	return server.NewServer(cfg, "")
}

// newTestnetTestServer builds a Server with one "testnet9999" profile (testnet
// tier, no master-address required for key generation).
func newTestnetTestServer(t *testing.T) *server.Server {
	t.Helper()
	return newTestnetServerFromProfiles(t, map[string]profiles.Profile{
		"testnet9999": testnet9999Profile(),
	})
}

// newLocalTestServerWithTestnet builds a Server with both a "local" profile
// and a "testnet9999" profile, used to test that local profiles are rejected.
func newLocalTestServerWithTestnet(t *testing.T) *server.Server {
	t.Helper()
	return newTestnetServerFromProfiles(t, map[string]profiles.Profile{
		"local":       {RPCURL: "http://127.0.0.1:26657", ChainID: "dev"},
		"testnet9999": testnet9999Profile(),
	})
}
