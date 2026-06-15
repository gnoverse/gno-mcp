package write

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/keystore"
)

func TestKeyDelete_removesKeyAndReportsAddress(t *testing.T) {
	s := newTestnetTestServer(t)
	ks := keystore.New(t.TempDir(), "", 5)
	_, err := ks.GenerateForProfile("testnet9999", "", testnet9999Profile())
	require.NoError(t, err)
	bob, err := ks.GenerateForProfile("testnet9999", "bob", testnet9999Profile())
	require.NoError(t, err)

	RegisterKeyDelete(s, ks)

	res, callErr := s.Registry().Call(context.Background(), "gno_key_delete", map[string]any{
		"profile": "testnet9999",
		"key":     "bob",
	})
	require.NoError(t, callErr)
	assert.Contains(t, res.Text, bob, "result names the abandoned address")

	// The key is gone; the default key is untouched.
	keys, err := ks.ListKeys("testnet9999", testnet9999Profile())
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.Equal(t, "default", keys[0].Name)
}

func TestKeyDelete_requiresKeyArg(t *testing.T) {
	s := newTestnetTestServer(t)
	ks := keystore.New(t.TempDir(), "", 5)
	RegisterKeyDelete(s, ks)

	_, callErr := s.Registry().Call(context.Background(), "gno_key_delete", map[string]any{
		"profile": "testnet9999",
	})
	require.Error(t, callErr, "key is required — must not delete a key by omission")
}

func TestKeyDelete_unknownKey(t *testing.T) {
	s := newTestnetTestServer(t)
	ks := keystore.New(t.TempDir(), "", 5)
	RegisterKeyDelete(s, ks)

	_, callErr := s.Registry().Call(context.Background(), "gno_key_delete", map[string]any{
		"profile": "testnet9999",
		"key":     "ghost",
	})
	require.Error(t, callErr, "deleting a nonexistent key must error")
}
