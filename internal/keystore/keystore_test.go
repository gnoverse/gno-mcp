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

func devProfile() profiles.Profile {
	return profiles.Profile{ChainID: "dev", ChainType: profiles.ChainTypeLocal, RPCURL: "http://127.0.0.1:26657"}
}
func testnet9999Profile() profiles.Profile {
	return profiles.Profile{ChainID: "test9999", ChainType: profiles.ChainTypeTestnet, RPCURL: "x"}
}

func TestAgentAddress_dev_isTest1(t *testing.T) {
	got, err := New(t.TempDir(), "").AgentAddress("dev", devProfile())
	require.NoError(t, err, "AgentAddress dev")
	const want = "g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5"
	require.Equal(t, want, got)
}

func TestTestnet_generateThenLoad(t *testing.T) {
	ks := New(t.TempDir(), "")
	addr, err := ks.GenerateForProfile("tnet", testnet9999Profile())
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(addr, "g1"), "bad addr %q", addr)
	// reload via a fresh Keystore over the same dir → same address (persisted)
	got, err := New(ks.rootDir, "").AgentAddress("tnet", testnet9999Profile())
	require.NoError(t, err, "reload")
	require.Equal(t, addr, got)
}

func TestTestnet_noKeyYet_errNoAgentKey(t *testing.T) {
	_, err := New(t.TempDir(), "").SignerForProfile("tnet", testnet9999Profile())
	require.ErrorIs(t, err, ErrNoAgentKey)
}

func TestTestnet_generateTwice_refuses(t *testing.T) {
	ks := New(t.TempDir(), "")
	_, err := ks.GenerateForProfile("tnet", testnet9999Profile())
	require.NoError(t, err)
	_, err = ks.GenerateForProfile("tnet", testnet9999Profile())
	require.Error(t, err, "second generate must refuse (no silent overwrite)")
}

func TestTestnet_existingUndecryptable_refusesAndPreserves(t *testing.T) {
	dir := t.TempDir()
	_, err := New(dir, "a").GenerateForProfile("tnet", testnet9999Profile())
	require.NoError(t, err)
	before, err := os.ReadFile(filepath.Join(dir, "tnet.key"))
	require.NoError(t, err)
	_, err = New(dir, "b").GenerateForProfile("tnet", testnet9999Profile())
	require.Error(t, err, "generate must refuse when a key file exists, even if undecryptable")
	after, err := os.ReadFile(filepath.Join(dir, "tnet.key"))
	require.NoError(t, err)
	require.True(t, bytes.Equal(before, after), "existing key file was overwritten")
}

func TestTestnet_encryptRoundTrip(t *testing.T) {
	dir := t.TempDir()
	addr, err := New(dir, "pass").GenerateForProfile("tnet", testnet9999Profile())
	require.NoError(t, err)
	got, err := New(dir, "pass").AgentAddress("tnet", testnet9999Profile())
	require.NoError(t, err, "reload with passphrase")
	require.Equal(t, addr, got)
}

func TestTestnet_wrongPassphrase_failsNotErrNoAgentKey(t *testing.T) {
	dir := t.TempDir()
	_, err := New(dir, "right").GenerateForProfile("tnet", testnet9999Profile())
	require.NoError(t, err)
	_, err = New(dir, "wrong").SignerForProfile("tnet", testnet9999Profile())
	require.Error(t, err, "wrong passphrase must fail")
	require.NotErrorIs(t, err, ErrNoAgentKey, "wrong passphrase must NOT be reported as ErrNoAgentKey")
}

func TestTestnet_encryptedThenNoPassphrase_clearError(t *testing.T) {
	dir := t.TempDir()
	_, err := New(dir, "pass").GenerateForProfile("tnet", testnet9999Profile())
	require.NoError(t, err)
	// Passphrase unset on reload: loading must fail clearly, not derive a signer
	// from a garbage mnemonic, and must not masquerade as "no key generated".
	_, err = New(dir, "").SignerForProfile("tnet", testnet9999Profile())
	require.Error(t, err, "loading an encrypted key with no passphrase must fail")
	require.NotErrorIs(t, err, ErrNoAgentKey, "must not be reported as ErrNoAgentKey")
}

func TestGenerate_concurrent_oneWinner(t *testing.T) {
	ks := New(t.TempDir(), "")
	const n = 8
	errs := make([]error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			_, errs[i] = ks.GenerateForProfile("tnet", testnet9999Profile())
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
	ks := New(t.TempDir(), "pass")
	_, err := ks.GenerateForProfile("tnet", testnet9999Profile())
	require.NoError(t, err)
	raw, err := os.ReadFile(filepath.Join(ks.rootDir, "tnet.key"))
	require.NoError(t, err)
	// The on-disk bytes must decrypt with the configured passphrase back to a
	// mnemonic (space-separated words).
	plain, err := secret.Decrypt(raw, "pass")
	require.NoError(t, err, "decrypt on-disk key")
	require.True(t, bytes.Contains(plain, []byte(" ")), "decrypted bytes do not look like a mnemonic: %q", plain)
}
