package keystore

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/profiles"
	secret "github.com/gnoverse/gno-mcp/internal/secret"
	"github.com/stretchr/testify/require"
)

const testCap = 5

func devProfile() profiles.Profile {
	return profiles.Profile{ChainID: "dev", RPCURL: "http://127.0.0.1:26657"}
}
func testnet9999Profile() profiles.Profile {
	return profiles.Profile{ChainID: "test9999", RPCURL: "x"}
}

func TestAgentAddress_dev_isTest1(t *testing.T) {
	got, err := New(t.TempDir(), "", testCap).AgentAddress("dev", "", devProfile())
	require.NoError(t, err, "AgentAddress dev")
	const want = "g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5"
	require.Equal(t, want, got)
}

func TestTestnet_generateThenLoad(t *testing.T) {
	ks := New(t.TempDir(), "", testCap)
	addr, err := ks.GenerateForProfile("tnet", "", testnet9999Profile())
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(addr, "g1"), "bad addr %q", addr)
	// reload via a fresh Keystore over the same dir → same address (persisted)
	got, err := New(ks.rootDir, "", testCap).AgentAddress("tnet", "", testnet9999Profile())
	require.NoError(t, err, "reload")
	require.Equal(t, addr, got)
}

func TestTestnet_noKeyYet_errNoAgentKey(t *testing.T) {
	_, err := New(t.TempDir(), "", testCap).SignerForProfile("tnet", "", testnet9999Profile())
	require.ErrorIs(t, err, ErrNoAgentKey)
}

func TestTestnet_generateTwice_refuses(t *testing.T) {
	ks := New(t.TempDir(), "", testCap)
	_, err := ks.GenerateForProfile("tnet", "", testnet9999Profile())
	require.NoError(t, err)
	_, err = ks.GenerateForProfile("tnet", "", testnet9999Profile())
	require.ErrorIs(t, err, ErrKeyExists, "second generate must refuse (no silent overwrite)")
}

func TestTestnet_namedKeysCoexist(t *testing.T) {
	ks := New(t.TempDir(), "", testCap)
	def, err := ks.GenerateForProfile("tnet", "", testnet9999Profile())
	require.NoError(t, err)
	bob, err := ks.GenerateForProfile("tnet", "bob", testnet9999Profile())
	require.NoError(t, err)
	require.NotEqual(t, def, bob, "distinct names must yield distinct addresses")

	gotDef, err := ks.AgentAddress("tnet", "default", testnet9999Profile())
	require.NoError(t, err)
	require.Equal(t, def, gotDef, "empty name and 'default' select the same key")
	gotBob, err := ks.AgentAddress("tnet", "bob", testnet9999Profile())
	require.NoError(t, err)
	require.Equal(t, bob, gotBob)
}

func TestTestnet_keyCapReached(t *testing.T) {
	ks := New(t.TempDir(), "", 2)
	_, err := ks.GenerateForProfile("tnet", "a", testnet9999Profile())
	require.NoError(t, err)
	_, err = ks.GenerateForProfile("tnet", "b", testnet9999Profile())
	require.NoError(t, err)
	_, err = ks.GenerateForProfile("tnet", "c", testnet9999Profile())
	require.ErrorIs(t, err, ErrKeyCapReached, "third key over a cap of 2 must refuse")
}

func TestTestnet_deleteThenRegenerateReplacesAtCap(t *testing.T) {
	ks := New(t.TempDir(), "", 2)
	_, err := ks.GenerateForProfile("tnet", "a", testnet9999Profile())
	require.NoError(t, err)
	first, err := ks.GenerateForProfile("tnet", "b", testnet9999Profile())
	require.NoError(t, err)

	// At the cap, a fresh name is refused; deleting frees a slot.
	_, err = ks.GenerateForProfile("tnet", "c", testnet9999Profile())
	require.ErrorIs(t, err, ErrKeyCapReached)

	deleted, err := ks.DeleteForProfile("tnet", "b", testnet9999Profile())
	require.NoError(t, err)
	require.Equal(t, first, deleted, "delete returns the abandoned address")

	// Regenerating the same name now succeeds and yields a different address.
	second, err := ks.GenerateForProfile("tnet", "b", testnet9999Profile())
	require.NoError(t, err, "after delete the slot is free again")
	require.NotEqual(t, first, second, "replacement is a fresh key")
}

func TestTestnet_deleteMissingKey(t *testing.T) {
	ks := New(t.TempDir(), "", testCap)
	_, err := ks.DeleteForProfile("tnet", "ghost", testnet9999Profile())
	require.ErrorIs(t, err, ErrNoAgentKey, "deleting a nonexistent key reports ErrNoAgentKey")
}

func TestTestnet_invalidKeyNameRefused(t *testing.T) {
	ks := New(t.TempDir(), "", testCap)
	_, err := ks.GenerateForProfile("tnet", "../escape", testnet9999Profile())
	require.ErrorIs(t, err, ErrInvalidKeyName, "path-traversal name must be rejected")
}

func TestTestnet_invalidProfileNameRefused(t *testing.T) {
	ks := New(t.TempDir(), "", testCap)
	// A path-traversal profile name must not escape the keystore root via profileDir.
	_, gErr := ks.GenerateForProfile("../../escape", "", testnet9999Profile())
	require.ErrorIs(t, gErr, ErrInvalidProfileName, "generate must reject a traversal profile name")
	_, sErr := ks.SignerForProfile("../../escape", "", testnet9999Profile())
	require.ErrorIs(t, sErr, ErrInvalidProfileName, "signer must reject a traversal profile name")
	_, lErr := ks.ListKeys("../../escape", testnet9999Profile())
	require.ErrorIs(t, lErr, ErrInvalidProfileName, "list must reject a traversal profile name")
}

func TestListKeys(t *testing.T) {
	ks := New(t.TempDir(), "", testCap)
	_, err := ks.GenerateForProfile("tnet", "", testnet9999Profile())
	require.NoError(t, err)
	_, err = ks.GenerateForProfile("tnet", "bob", testnet9999Profile())
	require.NoError(t, err)

	keys, err := ks.ListKeys("tnet", testnet9999Profile())
	require.NoError(t, err)
	require.Len(t, keys, 2)
	require.Equal(t, "bob", keys[0].Name, "keys are sorted by name")
	require.Equal(t, "default", keys[1].Name)
	for _, k := range keys {
		require.True(t, strings.HasPrefix(k.Address, "g1"), "bad addr %q", k.Address)
	}
}

func TestListKeys_dev_isSingleTest1(t *testing.T) {
	keys, err := New(t.TempDir(), "", testCap).ListKeys("dev", devProfile())
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.Equal(t, DefaultKeyName, keys[0].Name)
	require.Equal(t, Test1Address, keys[0].Address)
}

func TestListKeys_oneCorruptKeyDoesNotBlindTheRest(t *testing.T) {
	dir := t.TempDir()
	ks := New(dir, "", testCap)
	good, err := ks.GenerateForProfile("tnet", "", testnet9999Profile())
	require.NoError(t, err)
	_, err = ks.GenerateForProfile("tnet", "bob", testnet9999Profile())
	require.NoError(t, err)
	// Corrupt bob's key file so it can't be read.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tnet", "bob.key"), []byte("garbage"), 0o600))

	keys, err := ks.ListKeys("tnet", testnet9999Profile())
	require.NoError(t, err, "a single bad key file must not fail the whole listing")
	require.Len(t, keys, 2)
	byName := map[string]KeyInfo{keys[0].Name: keys[0], keys[1].Name: keys[1]}
	require.Equal(t, good, byName["default"].Address, "the good key still resolves")
	require.Empty(t, byName["default"].Err)
	require.NotEmpty(t, byName["bob"].Err, "the corrupt key is flagged, not hidden")
	require.Empty(t, byName["bob"].Address)
}

func TestListKeys_noKeysYet_empty(t *testing.T) {
	keys, err := New(t.TempDir(), "", testCap).ListKeys("tnet", testnet9999Profile())
	require.NoError(t, err)
	require.Empty(t, keys)
}

func TestTestnet_existingUndecryptable_refusesAndPreserves(t *testing.T) {
	dir := t.TempDir()
	_, err := New(dir, "a", testCap).GenerateForProfile("tnet", "", testnet9999Profile())
	require.NoError(t, err)
	keyFile := filepath.Join(dir, "tnet", "default.key")
	before, err := os.ReadFile(keyFile)
	require.NoError(t, err)
	_, err = New(dir, "b", testCap).GenerateForProfile("tnet", "", testnet9999Profile())
	require.Error(t, err, "generate must refuse when a key file exists, even if undecryptable")
	after, err := os.ReadFile(keyFile)
	require.NoError(t, err)
	require.True(t, bytes.Equal(before, after), "existing key file was overwritten")
}

func TestTestnet_encryptRoundTrip(t *testing.T) {
	dir := t.TempDir()
	addr, err := New(dir, "pass", testCap).GenerateForProfile("tnet", "", testnet9999Profile())
	require.NoError(t, err)
	got, err := New(dir, "pass", testCap).AgentAddress("tnet", "", testnet9999Profile())
	require.NoError(t, err, "reload with passphrase")
	require.Equal(t, addr, got)
}

func TestTestnet_wrongPassphrase_failsNotErrNoAgentKey(t *testing.T) {
	dir := t.TempDir()
	_, err := New(dir, "right", testCap).GenerateForProfile("tnet", "", testnet9999Profile())
	require.NoError(t, err)
	_, err = New(dir, "wrong", testCap).SignerForProfile("tnet", "", testnet9999Profile())
	require.Error(t, err, "wrong passphrase must fail")
	require.NotErrorIs(t, err, ErrNoAgentKey, "wrong passphrase must NOT be reported as ErrNoAgentKey")
}

func TestTestnet_encryptedThenNoPassphrase_clearError(t *testing.T) {
	dir := t.TempDir()
	_, err := New(dir, "pass", testCap).GenerateForProfile("tnet", "", testnet9999Profile())
	require.NoError(t, err)
	// Passphrase unset on reload: loading must fail clearly, not derive a signer
	// from a garbage mnemonic, and must not masquerade as "no key generated".
	_, err = New(dir, "", testCap).SignerForProfile("tnet", "", testnet9999Profile())
	require.Error(t, err, "loading an encrypted key with no passphrase must fail")
	require.NotErrorIs(t, err, ErrNoAgentKey, "must not be reported as ErrNoAgentKey")
}

func TestGenerate_concurrent_oneWinner(t *testing.T) {
	ks := New(t.TempDir(), "", testCap)
	const n = 8
	errs := make([]error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			_, errs[i] = ks.GenerateForProfile("tnet", "", testnet9999Profile())
		}(i)
	}
	wg.Wait()

	ok := 0
	for _, e := range errs {
		if e == nil {
			ok++
		}
	}
	require.Equal(t, 1, ok, "exactly one concurrent generate must succeed (no clobber)")
}

func TestTestnet_encryptedAtRest(t *testing.T) {
	ks := New(t.TempDir(), "pass", testCap)
	_, err := ks.GenerateForProfile("tnet", "", testnet9999Profile())
	require.NoError(t, err)
	raw, err := os.ReadFile(filepath.Join(ks.rootDir, "tnet", "default.key"))
	require.NoError(t, err)
	// The on-disk bytes must decrypt with the configured passphrase back to a
	// mnemonic (space-separated words).
	plain, err := secret.Decrypt(raw, "pass")
	require.NoError(t, err, "decrypt on-disk key")
	require.True(t, bytes.Contains(plain, []byte(" ")), "decrypted bytes do not look like a mnemonic: %q", plain)
}
