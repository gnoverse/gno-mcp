package write

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/keystore"
	"github.com/gnoverse/gno-mcp/internal/server"
)

func TestKeyDelete_removesUnfundedKeyAndReportsAddress(t *testing.T) {
	s := newTestnetTestServer(t)
	ks := keystore.New(t.TempDir(), "", 5)
	_, err := ks.GenerateForProfile("testnet9999", "", testnet9999Profile())
	require.NoError(t, err)
	bob, err := ks.GenerateForProfile("testnet9999", "bob", testnet9999Profile())
	require.NoError(t, err)

	RegisterKeyDelete(s, ks, constChainResolver(chain.NewFake())) // bob has zero balance

	res, callErr := s.Registry().Call(context.Background(), "gno_key_delete", map[string]any{
		"profile": "testnet9999",
		"key":     "bob",
	})
	require.NoError(t, callErr, "an unfunded key deletes without force")
	assert.Contains(t, res.Text, bob, "result names the deleted address")

	keys, err := ks.ListKeys("testnet9999", testnet9999Profile())
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.Equal(t, "default", keys[0].Name)
}

func TestKeyDelete_refusesFundedKeyAndSuggestsSweep(t *testing.T) {
	s := newTestnetTestServer(t)
	ks := keystore.New(t.TempDir(), "", 5)
	_, err := ks.GenerateForProfile("testnet9999", "", testnet9999Profile())
	require.NoError(t, err)
	bob, err := ks.GenerateForProfile("testnet9999", "bob", testnet9999Profile())
	require.NoError(t, err)

	fake := chain.NewFake()
	fake.SetBalance(bob, 5_000_000) // bob holds funds
	RegisterKeyDelete(s, ks, constChainResolver(fake))

	_, callErr := s.Registry().Call(context.Background(), "gno_key_delete", map[string]any{
		"profile": "testnet9999",
		"key":     "bob",
	})
	require.Error(t, callErr, "a funded key must not delete without force")
	var terr *server.ToolError
	require.True(t, errors.As(callErr, &terr))
	assert.Equal(t, "key_has_funds", terr.Code)
	assert.Equal(t, "default", terr.Extra["sweep_target"], "names the default key as the sweep target")
	assert.Contains(t, terr.Message, "sweep_to", "error leads with the one-step sweep_to recovery")

	// bob still exists — the refusal preserved it.
	keys, err := ks.ListKeys("testnet9999", testnet9999Profile())
	require.NoError(t, err)
	require.Len(t, keys, 2)
}

func TestKeyDelete_forceDeletesFundedKeyAndReportsAbandoned(t *testing.T) {
	s := newTestnetTestServer(t)
	ks := keystore.New(t.TempDir(), "", 5)
	_, err := ks.GenerateForProfile("testnet9999", "", testnet9999Profile())
	require.NoError(t, err)
	bob, err := ks.GenerateForProfile("testnet9999", "bob", testnet9999Profile())
	require.NoError(t, err)

	fake := chain.NewFake()
	fake.SetBalance(bob, 5_000_000)
	RegisterKeyDelete(s, ks, constChainResolver(fake))

	res, callErr := s.Registry().Call(context.Background(), "gno_key_delete", map[string]any{
		"profile": "testnet9999",
		"key":     "bob",
		"force":   true,
	})
	require.NoError(t, callErr, "force deletes a funded key")
	assert.Equal(t, int64(5_000_000), res.StructuredContent["abandoned_balance_ugnot"], "reports the abandoned balance")

	keys, err := ks.ListKeys("testnet9999", testnet9999Profile())
	require.NoError(t, err)
	require.Len(t, keys, 1)
}

func TestKeyDelete_lastFundedKeyHasNoSweepTarget(t *testing.T) {
	s := newTestnetTestServer(t)
	ks := keystore.New(t.TempDir(), "", 5)
	def, err := ks.GenerateForProfile("testnet9999", "", testnet9999Profile())
	require.NoError(t, err)

	fake := chain.NewFake()
	fake.SetBalance(def, 5_000_000)
	RegisterKeyDelete(s, ks, constChainResolver(fake))

	_, callErr := s.Registry().Call(context.Background(), "gno_key_delete", map[string]any{
		"profile": "testnet9999",
		"key":     "default",
	})
	require.Error(t, callErr)
	var terr *server.ToolError
	require.True(t, errors.As(callErr, &terr))
	assert.Equal(t, "key_has_funds", terr.Code)
	_, hasTarget := terr.Extra["sweep_target"]
	assert.False(t, hasTarget, "the only key has no sweep target")
	assert.Contains(t, terr.Message, "force=true", "points at force as the only option")
}

func TestKeyDelete_sweepToRecoversFundsThenDeletes(t *testing.T) {
	s := newTestnetTestServer(t)
	ks := keystore.New(t.TempDir(), "", 5)
	main, err := ks.GenerateForProfile("testnet9999", "main", testnet9999Profile())
	require.NoError(t, err)
	bob, err := ks.GenerateForProfile("testnet9999", "bob", testnet9999Profile())
	require.NoError(t, err)

	fake := chain.NewFake()
	const balance = 985_000_000
	fake.SetBalance(bob, balance)
	RegisterKeyDelete(s, ks, constChainResolver(fake))

	res, callErr := s.Registry().Call(context.Background(), "gno_key_delete", map[string]any{
		"profile":  "testnet9999",
		"key":      "bob",
		"sweep_to": "main",
	})
	require.NoError(t, callErr)

	// The full balance minus the gas fee is swept to main; bob is deleted.
	wantSwept := int64(balance - chain.DefaultGasFeeUgnot)
	assert.Equal(t, wantSwept, res.StructuredContent["swept_ugnot"])
	assert.Equal(t, "main", res.StructuredContent["swept_to"])
	mainBal, _ := fake.Balance(context.Background(), main)
	assert.Equal(t, wantSwept, mainBal, "main received the swept funds")

	keys, err := ks.ListKeys("testnet9999", testnet9999Profile())
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.Equal(t, "main", keys[0].Name)
}

func TestKeyDelete_sweepSendFailurePreservesKey(t *testing.T) {
	s := newTestnetTestServer(t)
	ks := keystore.New(t.TempDir(), "", 5)
	_, err := ks.GenerateForProfile("testnet9999", "main", testnet9999Profile())
	require.NoError(t, err)
	bob, err := ks.GenerateForProfile("testnet9999", "bob", testnet9999Profile())
	require.NoError(t, err)

	fake := chain.NewFake()
	fake.SetBalance(bob, 985_000_000)
	fake.SetSendError(errors.New("broadcast failed"))
	RegisterKeyDelete(s, ks, constChainResolver(fake))

	_, callErr := s.Registry().Call(context.Background(), "gno_key_delete", map[string]any{
		"profile":  "testnet9999",
		"key":      "bob",
		"sweep_to": "main",
	})
	require.Error(t, callErr, "a failed sweep must abort before deletion")

	// The headline atomicity guarantee: a failed transfer never loses the key.
	keys, _ := ks.ListKeys("testnet9999", testnet9999Profile())
	require.Len(t, keys, 2, "the key is preserved when its sweep fails")
}

func TestKeyDelete_sweepToBelowGasLeavesDust(t *testing.T) {
	s := newTestnetTestServer(t)
	ks := keystore.New(t.TempDir(), "", 5)
	_, err := ks.GenerateForProfile("testnet9999", "main", testnet9999Profile())
	require.NoError(t, err)
	bob, err := ks.GenerateForProfile("testnet9999", "bob", testnet9999Profile())
	require.NoError(t, err)

	fake := chain.NewFake()
	fake.SetBalance(bob, chain.DefaultGasFeeUgnot-1) // below the gas fee — unsweepable
	RegisterKeyDelete(s, ks, constChainResolver(fake))

	res, callErr := s.Registry().Call(context.Background(), "gno_key_delete", map[string]any{
		"profile":  "testnet9999",
		"key":      "bob",
		"sweep_to": "main",
	})
	require.NoError(t, callErr, "sub-gas dust still deletes (nothing the transfer can cover)")
	assert.Nil(t, res.StructuredContent["swept_ugnot"], "no sweep happened")
	assert.Equal(t, int64(chain.DefaultGasFeeUgnot-1), res.StructuredContent["abandoned_balance_ugnot"])
	assert.Equal(t, 0, len(fake.BankSends()), "no transfer attempted below gas")
}

func TestKeyDelete_sweepToUnknownKeyDoesNotDelete(t *testing.T) {
	s := newTestnetTestServer(t)
	ks := keystore.New(t.TempDir(), "", 5)
	bob, err := ks.GenerateForProfile("testnet9999", "bob", testnet9999Profile())
	require.NoError(t, err)
	fake := chain.NewFake()
	fake.SetBalance(bob, 985_000_000)
	RegisterKeyDelete(s, ks, constChainResolver(fake))

	_, callErr := s.Registry().Call(context.Background(), "gno_key_delete", map[string]any{
		"profile":  "testnet9999",
		"key":      "bob",
		"sweep_to": "ghost",
	})
	require.Error(t, callErr, "sweeping to a nonexistent key must error before any delete")
	keys, _ := ks.ListKeys("testnet9999", testnet9999Profile())
	require.Len(t, keys, 1, "bob is preserved when the sweep target is invalid")
}

func TestKeyDelete_sweepToSelfRejected(t *testing.T) {
	s := newTestnetTestServer(t)
	ks := keystore.New(t.TempDir(), "", 5)
	bob, err := ks.GenerateForProfile("testnet9999", "bob", testnet9999Profile())
	require.NoError(t, err)
	fake := chain.NewFake()
	fake.SetBalance(bob, 985_000_000)
	RegisterKeyDelete(s, ks, constChainResolver(fake))

	_, callErr := s.Registry().Call(context.Background(), "gno_key_delete", map[string]any{
		"profile":  "testnet9999",
		"key":      "bob",
		"sweep_to": "bob",
	})
	require.Error(t, callErr, "sweep_to must differ from the key being deleted")
}

func TestKeyDelete_requiresKeyArg(t *testing.T) {
	s := newTestnetTestServer(t)
	ks := keystore.New(t.TempDir(), "", 5)
	RegisterKeyDelete(s, ks, constChainResolver(chain.NewFake()))

	_, callErr := s.Registry().Call(context.Background(), "gno_key_delete", map[string]any{
		"profile": "testnet9999",
	})
	require.Error(t, callErr, "key is required — must not delete a key by omission")
}

func TestKeyDelete_unknownKey(t *testing.T) {
	s := newTestnetTestServer(t)
	ks := keystore.New(t.TempDir(), "", 5)
	RegisterKeyDelete(s, ks, constChainResolver(chain.NewFake()))

	_, callErr := s.Registry().Call(context.Background(), "gno_key_delete", map[string]any{
		"profile": "testnet9999",
		"key":     "ghost",
	})
	require.Error(t, callErr, "deleting a nonexistent key must error")
}
