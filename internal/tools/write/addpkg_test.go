package write

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gnolang/gno/tm2/pkg/std"

	"github.com/gnoverse/gno-mcp/internal/audit"
	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/keystore"
	"github.com/gnoverse/gno-mcp/internal/server"
)

func TestAddPkg_happyPath(t *testing.T) {
	s := newLocalTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	ks := keystore.New(t.TempDir(), "")

	fake := chain.NewFake()
	fake.SetAddPackage("gno.land/r/test/foo", chain.AddPackageResult{
		TxHash:  "h",
		Height:  3,
		GasUsed: 5000,
	})

	RegisterAddPkg(s, ks, constChainResolver(fake), alog)

	res, err := s.Registry().Call(context.Background(), "gno_addpkg", map[string]any{
		"profile":     "local",
		"deploy_path": "gno.land/r/test/foo",
		"files": []any{
			map[string]any{
				"name": "foo.gno",
				"body": "package foo\n\nfunc Hello() string { return \"hi\" }\n",
			},
		},
	})
	if err != nil {
		t.Fatalf("AddPkg: %v", err)
	}
	if !strings.Contains(res.Text, "Signed by: agent test1 (g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5)") {
		t.Errorf("expected agent identity line in text:\n%s", res.Text)
	}

	// gnomod.toml must have been injected and files must be sorted.
	files := fake.LastAddPackageFiles("gno.land/r/test/foo")
	if files == nil {
		t.Fatal("LastAddPackageFiles returned nil — AddPackage was not called")
	}
	assertHasGnomod(t, files)
	assertSorted(t, files)
}

func TestAddPkg_agentIdentityUnavailable(t *testing.T) {
	// testnet profile → keystore returns ErrNoAgentKey → agent_identity_unavailable.
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	ks := keystore.New(t.TempDir(), "")

	fake := chain.NewFake()
	RegisterAddPkg(s, ks, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_addpkg", map[string]any{
		"profile":     "testnet5",
		"deploy_path": "gno.land/r/test/foo",
		"files": []any{
			map[string]any{
				"name": "foo.gno",
				"body": "package foo\n",
			},
		},
	})
	if err == nil {
		t.Fatal("expected agent_identity_unavailable error")
	}
	te, ok := errors.AsType[*server.ToolError](err)
	if !ok {
		t.Fatalf("expected *server.ToolError, got %T: %v", err, err)
	}
	if te.Code != "agent_identity_unavailable" {
		t.Errorf("expected code=agent_identity_unavailable, got %q", te.Code)
	}
}

// TestAddPkg_agentTestnet_insufficientFunds verifies that gno_addpkg returns
// insufficient_funds when the agent's testnet account has zero balance.
func TestAddPkg_agentTestnet_insufficientFunds(t *testing.T) {
	s := newTestnetTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	ks := keystore.New(t.TempDir(), "")

	agentAddr, err := ks.GenerateForProfile("testnet9999", testnet9999Profile())
	if err != nil {
		t.Fatalf("GenerateForProfile: %v", err)
	}

	fake := chain.NewFake() // balance 0 by default
	RegisterAddPkg(s, ks, constChainResolver(fake), alog)

	_, pkgErr := s.Registry().Call(context.Background(), "gno_addpkg", map[string]any{
		"profile":     "testnet9999",
		"deploy_path": "gno.land/r/test/foo",
		"files": []any{
			map[string]any{"name": "foo.gno", "body": "package foo\n"},
		},
	})
	if pkgErr == nil {
		t.Fatal("expected insufficient_funds error")
	}
	te, ok := errors.AsType[*server.ToolError](pkgErr)
	if !ok {
		t.Fatalf("expected *server.ToolError, got %T: %v", pkgErr, pkgErr)
	}
	if te.Code != "insufficient_funds" {
		t.Errorf("expected code=insufficient_funds, got %q", te.Code)
	}
	if te.Extra["address"] != agentAddr {
		t.Errorf("Extra[address]=%v, want %q", te.Extra["address"], agentAddr)
	}
}

// TestAddPkg_agentTestnet_funded verifies that a funded testnet agent account
// proceeds past the balance check and broadcasts the AddPackage.
func TestAddPkg_agentTestnet_funded(t *testing.T) {
	s := newTestnetTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	ks := keystore.New(t.TempDir(), "")

	agentAddr, err := ks.GenerateForProfile("testnet9999", testnet9999Profile())
	if err != nil {
		t.Fatalf("GenerateForProfile: %v", err)
	}

	fake := chain.NewFake()
	fake.SetBalance(agentAddr, 10_000_000)
	fake.SetAddPackage("gno.land/r/test/foo", chain.AddPackageResult{
		TxHash:  "0xaddpkg",
		Height:  4,
		GasUsed: 8000,
	})
	RegisterAddPkg(s, ks, constChainResolver(fake), alog)

	res, pkgErr := s.Registry().Call(context.Background(), "gno_addpkg", map[string]any{
		"profile":     "testnet9999",
		"deploy_path": "gno.land/r/test/foo",
		"files": []any{
			map[string]any{"name": "foo.gno", "body": "package foo\n"},
		},
	})
	if pkgErr != nil {
		t.Fatalf("expected success for funded account, got: %v", pkgErr)
	}
	if !strings.Contains(res.Text, "0xaddpkg") {
		t.Errorf("expected tx hash in result text:\n%s", res.Text)
	}
}

// assertHasGnomod fails t if no file named "gnomod.toml" is in files.
func assertHasGnomod(t *testing.T, files []*std.MemFile) {
	t.Helper()
	for _, f := range files {
		if f.Name == "gnomod.toml" {
			return
		}
	}
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = f.Name
	}
	t.Errorf("expected gnomod.toml in files, got: %v", names)
}

// assertSorted fails t if files are not sorted lexicographically by Name.
func assertSorted(t *testing.T, files []*std.MemFile) {
	t.Helper()
	for i := 1; i < len(files); i++ {
		if files[i].Name < files[i-1].Name {
			t.Errorf("files not sorted: files[%d].Name=%q < files[%d].Name=%q",
				i, files[i].Name, i-1, files[i-1].Name)
		}
	}
}
