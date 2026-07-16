package write

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/gnolang/gno/gno.land/pkg/sdk/vm"
	tmerrors "github.com/gnolang/gno/tm2/pkg/errors"
	"github.com/gnolang/gno/tm2/pkg/std"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/audit"
	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/keystore"
	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/gnoverse/gno-mcp/internal/server"
)

// Authoring .gno source is where Go intuitions produce broken realms; the
// description gives chain-native authoring advice (reference packages via
// gno_read) and stays client-agnostic — skill routing belongs to the skill
// layer, not to MCP tool descriptions.
func TestAddPkg_descriptionRoutesAuthoringGuidance(t *testing.T) {
	s := newLocalTestServer(t)
	var auditBuf bytes.Buffer
	RegisterAddPkg(s, keystore.New(t.TempDir(), "", 5), constChainResolver(chain.NewFake()), audit.NewLog(&auditBuf))

	tool, ok := s.Registry().Get("gno_addpkg")
	require.True(t, ok)
	assert.Contains(t, tool.Description, "not Go's")
	assert.Contains(t, tool.Description, "gno_read")
	assert.NotContains(t, tool.Description, "skill", "MCP descriptions are client-agnostic — no plugin-layer references")
}

func TestAddPkg_happyPath(t *testing.T) {
	s := newLocalTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	ks := keystore.New(t.TempDir(), "", 5)

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
	require.NoError(t, err, "AddPkg")
	assert.Contains(t, res.Text, "Signed by: agent test1 (g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5)")
	gk, _ := res.StructuredContent["gnokey_command"].(string)
	assert.Contains(t, gk, "gnokey maketx addpkg", "addpkg must wire its own subcommand")
	assert.Contains(t, gk, "-pkgpath gno.land/r/test/foo")
	assert.Contains(t, gk, "-pkgdir", "addpkg shows the source-dir placeholder")

	// gnomod.toml must have been injected and files must be sorted.
	files := fake.LastAddPackageFiles("gno.land/r/test/foo")
	require.NotNil(t, files, "LastAddPackageFiles returned nil — AddPackage was not called")
	assertHasGnomod(t, files)
	assertSorted(t, files)
}

// A failed addpkg broadcast still burns gas on-chain (the node charges for the
// type-check / gate rejection at DeliverTx), which can strand a freshly-funded
// key. The handler must validate via simulate first and never broadcast when
// that fails.
func TestAddPkg_validatesBeforeBroadcast(t *testing.T) {
	s := newLocalTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	ks := keystore.New(t.TempDir(), "", 5)

	fake := chain.NewFake()
	fake.SetAddPackageError("gno.land/r/test/bad", errors.New("type check errors: could not import fmt"))
	RegisterAddPkg(s, ks, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_addpkg", map[string]any{
		"profile":     "local",
		"deploy_path": "gno.land/r/test/bad",
		"files":       []any{map[string]any{"name": "bad.gno", "body": "package bad\n"}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation (no gas spent)", "must fail at validation, not broadcast")
	assert.Equal(t, 0, fake.AddPackageBroadcasts("gno.land/r/test/bad"), "must not broadcast after validation fails")

	entries := parseAuditEntries(t, &auditBuf)
	require.Len(t, entries, 1, "a validation-denied deploy must still produce exactly one audit record")
	assert.Equal(t, "validate_err", entries[0].Result, "the zero-gas pre-check failure is its own audit label")
}

// The CLA gate rejects during the validation simulate too, so the cla_unsigned
// hint must reach the user at zero gas — before any broadcast.
func TestAddPkg_claUnsignedCaughtAtValidation(t *testing.T) {
	s := newLocalTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	ks := keystore.New(t.TempDir(), "", 5)

	fake := chain.NewFake()
	claErr := tmerrors.Wrapf(vm.UnauthorizedUserError{}, "deliver transaction failed: log:address g1abc has not signed the required CLA")
	fake.SetAddPackageError("gno.land/r/test/cla", claErr)
	RegisterAddPkg(s, ks, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_addpkg", map[string]any{
		"profile":     "local",
		"deploy_path": "gno.land/r/test/cla",
		"files":       []any{map[string]any{"name": "c.gno", "body": "package c\n"}},
	})
	require.Error(t, err)
	var te *server.ToolError
	require.ErrorAs(t, err, &te)
	assert.Equal(t, "cla_unsigned", te.Code)
	assert.Equal(t, 0, fake.AddPackageBroadcasts("gno.land/r/test/cla"), "CLA caught at validation — no broadcast")
}

func TestAddPkg_agentIdentityUnavailable(t *testing.T) {
	// testnet profile → keystore returns ErrNoAgentKey → agent_identity_unavailable.
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	ks := keystore.New(t.TempDir(), "", 5)

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
	require.Error(t, err, "expected agent_identity_unavailable error")
	var te *server.ToolError
	require.ErrorAs(t, err, &te)
	assert.Equal(t, "agent_identity_unavailable", te.Code)
}

// TestAddPkg_agentTestnet_insufficientFunds verifies that gno_addpkg returns
// insufficient_funds when the agent's testnet account has zero balance.
func TestAddPkg_agentTestnet_insufficientFunds(t *testing.T) {
	s := newTestnetTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	ks := keystore.New(t.TempDir(), "", 5)

	agentAddr, err := ks.GenerateForProfile("testnet9999", "", testnet9999Profile())
	require.NoError(t, err, "GenerateForProfile")

	fake := chain.NewFake() // balance 0 by default
	RegisterAddPkg(s, ks, constChainResolver(fake), alog)

	_, pkgErr := s.Registry().Call(context.Background(), "gno_addpkg", map[string]any{
		"profile":     "testnet9999",
		"deploy_path": "gno.land/r/test/foo",
		"files": []any{
			map[string]any{"name": "foo.gno", "body": "package foo\n"},
		},
	})
	require.Error(t, pkgErr, "expected insufficient_funds error")
	var te *server.ToolError
	require.ErrorAs(t, pkgErr, &te)
	assert.Equal(t, "insufficient_funds", te.Code)
	assert.Equal(t, agentAddr, te.Extra["address"])
}

// TestAddPkg_insufficientFundsDenialAuditsArgs verifies that the audit record
// for an insufficient_funds denial carries the value-free args summary
// (deploy path and post-injection file count), not an empty field.
func TestAddPkg_insufficientFundsDenialAuditsArgs(t *testing.T) {
	s := newTestnetTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	ks := keystore.New(t.TempDir(), "", 5)

	_, err := ks.GenerateForProfile("testnet9999", "", testnet9999Profile())
	require.NoError(t, err, "GenerateForProfile")

	fake := chain.NewFake() // balance 0 by default
	RegisterAddPkg(s, ks, constChainResolver(fake), alog)

	_, pkgErr := s.Registry().Call(context.Background(), "gno_addpkg", map[string]any{
		"profile":     "testnet9999",
		"deploy_path": "gno.land/r/test/foo",
		"files": []any{
			map[string]any{"name": "foo.gno", "body": "package foo\n"},
		},
	})
	require.Error(t, pkgErr, "expected insufficient_funds error")

	entries := parseAuditEntries(t, &auditBuf)
	require.Len(t, entries, 1, "a denied deploy must still produce exactly one audit record")
	assert.Equal(t, "tool_err", entries[0].Result)
	assert.Contains(t, entries[0].ArgsSummary, "deploy_path=gno.land/r/test/foo")
	assert.Contains(t, entries[0].ArgsSummary, "files=2",
		"file count includes the generated gnomod.toml")
}

// TestAddPkg_agentTestnet_funded verifies that a funded testnet agent account
// proceeds past the balance check and broadcasts the AddPackage.
func TestAddPkg_agentTestnet_funded(t *testing.T) {
	s := newTestnetTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	ks := keystore.New(t.TempDir(), "", 5)

	agentAddr, err := ks.GenerateForProfile("testnet9999", "", testnet9999Profile())
	require.NoError(t, err, "GenerateForProfile")

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
	require.NoError(t, pkgErr, "expected success for funded account")
	assert.Contains(t, res.Text, "0xaddpkg")
}

// A successful deploy must hand the agent the exact gnoweb URL where the new
// realm is viewable, derived from the profile — the agent should never have to
// guess the host.
func TestAddPkg_surfacesGnowebURL(t *testing.T) {
	webProfile := profiles.Profile{
		RPCURL:    "http://127.0.0.1:26657",
		ChainID:   "test9999",
		GnowebURL: "https://test9999.gno.land",
	}
	s := newTestnetServerFromProfiles(t, map[string]profiles.Profile{"testnet9999": webProfile})
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	ks := keystore.New(t.TempDir(), "", 5)

	agentAddr, err := ks.GenerateForProfile("testnet9999", "", webProfile)
	require.NoError(t, err, "GenerateForProfile")

	fake := chain.NewFake()
	fake.SetBalance(agentAddr, 10_000_000)
	fake.SetAddPackage("gno.land/r/test/foo", chain.AddPackageResult{
		TxHash: "0xaddpkg", Height: 4, GasUsed: 8000,
	})
	RegisterAddPkg(s, ks, constChainResolver(fake), alog)

	res, pkgErr := s.Registry().Call(context.Background(), "gno_addpkg", map[string]any{
		"profile":     "testnet9999",
		"deploy_path": "gno.land/r/test/foo",
		"files":       []any{map[string]any{"name": "foo.gno", "body": "package foo\n"}},
	})
	require.NoError(t, pkgErr)

	const want = "https://test9999.gno.land/r/test/foo"
	assert.Contains(t, res.Text, want, "deploy result must show the realm's gnoweb URL")
	assert.Equal(t, want, res.StructuredContent["gnoweb_url"], "structured content must carry the gnoweb URL")
}

// A simulate run deploys nothing, so it must not advertise a view URL.
func TestAddPkg_simulateOmitsGnowebURL(t *testing.T) {
	webProfile := profiles.Profile{
		RPCURL:    "http://127.0.0.1:26657",
		ChainID:   "test9999",
		GnowebURL: "https://test9999.gno.land",
	}
	s := newTestnetServerFromProfiles(t, map[string]profiles.Profile{"testnet9999": webProfile})
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	ks := keystore.New(t.TempDir(), "", 5)

	agentAddr, err := ks.GenerateForProfile("testnet9999", "", webProfile)
	require.NoError(t, err, "GenerateForProfile")

	fake := chain.NewFake()
	fake.SetBalance(agentAddr, 10_000_000)
	fake.SetAddPackage("gno.land/r/test/foo", chain.AddPackageResult{Simulated: true})
	RegisterAddPkg(s, ks, constChainResolver(fake), alog)

	res, pkgErr := s.Registry().Call(context.Background(), "gno_addpkg", map[string]any{
		"profile":     "testnet9999",
		"deploy_path": "gno.land/r/test/foo",
		"files":       []any{map[string]any{"name": "foo.gno", "body": "package foo\n"}},
		"simulate":    true,
	})
	require.NoError(t, pkgErr)
	assert.NotContains(t, res.Text, "test9999.gno.land/r/test/foo", "simulate must not advertise a view URL")
	assert.Nil(t, res.StructuredContent["gnoweb_url"], "simulate must not carry a gnoweb URL")
}

// A short deploy_path (no "/") expands to the agent's own-address namespace,
// and the auto-generated gnomod.toml must carry the EXPANDED path — a module
// line naming the bare short name would be rejected by the chain.
func TestAddPkg_expandsShortDeployPath(t *testing.T) {
	s := newTestnetTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	ks := keystore.New(t.TempDir(), "", 5)

	agentAddr, err := ks.GenerateForProfile("testnet9999", "", testnet9999Profile())
	require.NoError(t, err, "GenerateForProfile")

	expanded := "gno.land/r/" + agentAddr + "/hello"
	fake := chain.NewFake()
	fake.SetBalance(agentAddr, 10_000_000)
	fake.SetAddPackage(expanded, chain.AddPackageResult{TxHash: "0xshort", Height: 5, GasUsed: 9000})
	RegisterAddPkg(s, ks, constChainResolver(fake), alog)

	res, pkgErr := s.Registry().Call(context.Background(), "gno_addpkg", map[string]any{
		"profile":     "testnet9999",
		"deploy_path": "hello",
		"files":       []any{map[string]any{"name": "hello.gno", "body": "package hello\n"}},
	})
	require.NoError(t, pkgErr, "short deploy_path must deploy under the agent's own-address namespace")
	assert.Contains(t, res.Text, "0xshort")

	files := fake.LastAddPackageFiles(expanded)
	require.NotNil(t, files, "AddPackage must be called with the expanded path")
	var gnomod *std.MemFile
	for _, f := range files {
		if f.Name == "gnomod.toml" {
			gnomod = f
		}
	}
	require.NotNil(t, gnomod, "gnomod.toml must be injected")
	assert.Contains(t, gnomod.Body, expanded, "the generated gnomod.toml must name the expanded path, not the short name")

	entries := parseAuditEntries(t, &auditBuf)
	require.Len(t, entries, 1)
	assert.Contains(t, entries[0].ArgsSummary, "deploy_path="+expanded,
		"the audit summary must record the path that was actually deployed")
}

// A caller-supplied gnomod.toml names a module path the handler cannot rewrite;
// combined with a short deploy_path the module line and the expanded path would
// disagree and the chain would reject with an opaque error. Refuse it up front.
func TestAddPkg_shortDeployPathRejectsUserGnomod(t *testing.T) {
	s := newTestnetTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	ks := keystore.New(t.TempDir(), "", 5)

	agentAddr, err := ks.GenerateForProfile("testnet9999", "", testnet9999Profile())
	require.NoError(t, err, "GenerateForProfile")

	fake := chain.NewFake()
	fake.SetBalance(agentAddr, 10_000_000)
	RegisterAddPkg(s, ks, constChainResolver(fake), alog)

	_, pkgErr := s.Registry().Call(context.Background(), "gno_addpkg", map[string]any{
		"profile":     "testnet9999",
		"deploy_path": "hello",
		"files": []any{
			map[string]any{"name": "hello.gno", "body": "package hello\n"},
			map[string]any{"name": "gnomod.toml", "body": "module = \"hello\"\n"},
		},
	})
	require.Error(t, pkgErr, "short name + caller-supplied gnomod.toml must be refused before broadcast")
	assert.Contains(t, pkgErr.Error(), "deploy_path")
	assert.Contains(t, pkgErr.Error(), "gnomod.toml")
	assert.Equal(t, 0, fake.AddPackageBroadcasts("gno.land/r/"+agentAddr+"/hello"),
		"nothing may be broadcast for the ambiguous combination")
}

// assertHasGnomod fails t if no file named "gnomod.toml" is in files.
func assertHasGnomod(t *testing.T, files []*std.MemFile) {
	t.Helper()
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = f.Name
	}
	assert.Contains(t, names, "gnomod.toml")
}

// assertSorted fails t if files are not sorted lexicographically by Name.
func assertSorted(t *testing.T, files []*std.MemFile) {
	t.Helper()
	for i := 1; i < len(files); i++ {
		assert.LessOrEqual(t, files[i-1].Name, files[i].Name, "files not sorted at index %d", i)
	}
}
