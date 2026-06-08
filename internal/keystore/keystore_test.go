package keystore

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/profiles"
	secret "github.com/gnoverse/gno-mcp/internal/secret"
)

func devProfile() profiles.Profile {
	return profiles.Profile{ChainID: "dev", ChainType: profiles.ChainTypeLocal, RPCURL: "http://127.0.0.1:26657"}
}
func testnet9999Profile() profiles.Profile {
	return profiles.Profile{ChainID: "test9999", ChainType: profiles.ChainTypeTestnet, RPCURL: "x"}
}

func TestAgentAddress_dev_isTest1(t *testing.T) {
	got, err := New(t.TempDir(), "").AgentAddress("dev", devProfile())
	if err != nil {
		t.Fatalf("AgentAddress dev: %v", err)
	}
	const want = "g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5"
	if got != want {
		t.Fatalf("test1 address = %q, want %q", got, want)
	}
}

func TestTestnet_generateThenLoad(t *testing.T) {
	ks := New(t.TempDir(), "")
	addr, err := ks.GenerateForProfile("tnet", testnet9999Profile())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(addr, "g1") {
		t.Fatalf("bad addr %q", addr)
	}
	// reload via a fresh Keystore over the same dir → same address (persisted)
	got, err := New(ks.rootDir, "").AgentAddress("tnet", testnet9999Profile())
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got != addr {
		t.Fatalf("reload addr %q != %q", got, addr)
	}
}

func TestTestnet_noKeyYet_errNoAgentKey(t *testing.T) {
	if _, err := New(t.TempDir(), "").SignerForProfile("tnet", testnet9999Profile()); !errors.Is(err, ErrNoAgentKey) {
		t.Fatalf("want ErrNoAgentKey, got %v", err)
	}
}

func TestTestnet_generateTwice_refuses(t *testing.T) {
	ks := New(t.TempDir(), "")
	if _, err := ks.GenerateForProfile("tnet", testnet9999Profile()); err != nil {
		t.Fatal(err)
	}
	if _, err := ks.GenerateForProfile("tnet", testnet9999Profile()); err == nil {
		t.Fatal("second generate must refuse (no silent overwrite)")
	}
}

func TestTestnet_existingUndecryptable_refusesAndPreserves(t *testing.T) {
	dir := t.TempDir()
	if _, err := New(dir, "a").GenerateForProfile("tnet", testnet9999Profile()); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(dir, "tnet.key"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(dir, "b").GenerateForProfile("tnet", testnet9999Profile()); err == nil {
		t.Fatal("generate must refuse when a key file exists, even if undecryptable")
	}
	after, err := os.ReadFile(filepath.Join(dir, "tnet.key"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("existing key file was overwritten")
	}
}

func TestTestnet_encryptRoundTrip(t *testing.T) {
	dir := t.TempDir()
	addr, err := New(dir, "pass").GenerateForProfile("tnet", testnet9999Profile())
	if err != nil {
		t.Fatal(err)
	}
	got, err := New(dir, "pass").AgentAddress("tnet", testnet9999Profile())
	if err != nil {
		t.Fatalf("reload with passphrase: %v", err)
	}
	if got != addr {
		t.Fatalf("reload addr %q != %q", got, addr)
	}
}

func TestTestnet_wrongPassphrase_failsNotErrNoAgentKey(t *testing.T) {
	dir := t.TempDir()
	if _, err := New(dir, "right").GenerateForProfile("tnet", testnet9999Profile()); err != nil {
		t.Fatal(err)
	}
	_, err := New(dir, "wrong").SignerForProfile("tnet", testnet9999Profile())
	if err == nil {
		t.Fatal("wrong passphrase must fail")
	}
	if errors.Is(err, ErrNoAgentKey) {
		t.Fatal("wrong passphrase must NOT be reported as ErrNoAgentKey")
	}
}

func TestTestnet_encryptedAtRest(t *testing.T) {
	ks := New(t.TempDir(), "pass")
	if _, err := ks.GenerateForProfile("tnet", testnet9999Profile()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(ks.rootDir, "tnet.key"))
	if err != nil {
		t.Fatal(err)
	}
	// The on-disk bytes must decrypt with the configured passphrase back to a
	// mnemonic (space-separated words).
	plain, err := secret.Decrypt(raw, "pass")
	if err != nil {
		t.Fatalf("decrypt on-disk key: %v", err)
	}
	if !bytes.Contains(plain, []byte(" ")) {
		t.Fatalf("decrypted bytes do not look like a mnemonic: %q", plain)
	}
}
