package write

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/keystore"
)

func TestKeyList_listsGeneratedKeys(t *testing.T) {
	s := newTestnetTestServer(t)
	ks := keystore.New(t.TempDir(), "", 5)
	_, err := ks.GenerateForProfile("testnet9999", "", testnet9999Profile())
	require.NoError(t, err)
	_, err = ks.GenerateForProfile("testnet9999", "bob", testnet9999Profile())
	require.NoError(t, err)

	RegisterKeyList(s, ks)

	res, callErr := s.Registry().Call(context.Background(), "gno_key_list", map[string]any{
		"profile": "testnet9999",
	})
	require.NoError(t, callErr)
	assert.Contains(t, res.Text, "default")
	assert.Contains(t, res.Text, "bob")

	keys, ok := res.StructuredContent["keys"].([]map[string]any)
	require.True(t, ok, "keys must be a list")
	require.Len(t, keys, 2)
}

func TestKeyList_emptyWhenNoKeys(t *testing.T) {
	s := newTestnetTestServer(t)
	ks := keystore.New(t.TempDir(), "", 5)
	RegisterKeyList(s, ks)

	res, callErr := s.Registry().Call(context.Background(), "gno_key_list", map[string]any{
		"profile": "testnet9999",
	})
	require.NoError(t, callErr)
	assert.Contains(t, res.Text, "gno_key_generate", "empty list should hint at generating a key")
}
