package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/audit"
	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/keystore"
	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/gnoverse/gno-mcp/internal/server"
	"github.com/gnoverse/gno-mcp/internal/session"
)

// newDynDeps builds a toolDeps with throwaway storage, a fake chain-id
// verifier, and a no-op republish — tests override fields as needed.
func newDynDeps(t *testing.T, s *server.Server, resolver chain.Resolver) *toolDeps {
	t.Helper()
	return &toolDeps{
		srv:             s,
		chainResolver:   resolver,
		indexerResolver: buildIndexerResolver(s),
		sessionMgr:      session.NewManager(t.TempDir(), ""),
		keystore:        keystore.New(t.TempDir(), ""),
		auditLog:        audit.NewLog(io.Discard),
		connectClient:   http.DefaultClient,
		faucetClient:    http.DefaultClient,
		verifyChainID:   func(_ context.Context, _ string) (string, error) { return "test-13", nil },
		onProfileAdded:  func() error { return nil },
	}
}

func localOnlyServer(t *testing.T) *server.Server {
	t.Helper()
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"local": {RPCURL: "http://127.0.0.1:26657", ChainID: "dev"},
	}}
	_, err := cfg.Validate()
	require.NoError(t, err)
	return server.NewServer(cfg, "")
}

// ---- lazy resolvers

func TestBuildChainResolver_resolvesDynamicallyAddedProfile(t *testing.T) {
	s := localOnlyServer(t)
	resolve := buildChainResolver(s)
	require.Nil(t, resolve("dyn"), "unknown profile must resolve to nil before the add")

	require.NoError(t, s.AddDynamicProfile("dyn", profiles.Profile{RPCURL: "http://example", ChainID: "test5"}))
	require.NotNil(t, resolve("dyn"), "resolver must lazily pick up a dynamically added profile")
}

func TestBuildChainResolver_reAddWithNewURLGetsNewClient(t *testing.T) {
	s := localOnlyServer(t)
	resolve := buildChainResolver(s)

	require.NoError(t, s.AddDynamicProfile("dyn", profiles.Profile{RPCURL: "http://one", ChainID: "test5"}))
	c1 := resolve("dyn")
	require.NotNil(t, c1)
	assert.Same(t, c1, resolve("dyn"), "same rpc-url must hit the client cache")

	require.NoError(t, s.AddDynamicProfile("dyn", profiles.Profile{RPCURL: "http://two", ChainID: "test5"}))
	c2 := resolve("dyn")
	require.NotNil(t, c2)
	assert.NotSame(t, c1, c2, "a re-add with a new rpc-url must produce a fresh client")
}

func TestBuildIndexerResolver_resolvesDynamicallyAddedProfile(t *testing.T) {
	s := localOnlyServer(t)
	resolve := buildIndexerResolver(s)
	require.Nil(t, resolve("dyn"), "unknown profile -> nil")

	require.NoError(t, s.AddDynamicProfile("dyn", profiles.Profile{
		RPCURL: "http://example", ChainID: "test5", TxIndexerURL: "http://idx.example/graphql",
	}))
	require.NotNil(t, resolve("dyn"), "indexer resolver must lazily pick up the dynamic profile")

	require.NoError(t, s.AddDynamicProfile("noidx", profiles.Profile{RPCURL: "http://example", ChainID: "test5"}))
	require.Nil(t, resolve("noidx"), "profile without tx-indexer-url -> nil")
}

// ---- re-invocable registration

func TestRegisterAllTools_gateFlipSummonsTools(t *testing.T) {
	s := localOnlyServer(t)
	deps := newDynDeps(t, s, buildChainResolver(s))

	registerAllTools(deps)
	_, ok := s.Registry().Get("gno_faucet_fund")
	require.False(t, ok, "local-only config must not register the faucet tool")
	_, ok = s.Registry().Get("gno_list")
	require.False(t, ok, "no indexer profile -> no indexer tools")
	_, ok = s.Registry().Get("gno_profile_add")
	require.True(t, ok, "gno_profile_add must always register")
	_, ok = s.Registry().Get("gno_key_generate")
	require.True(t, ok, "agent tools are unconditional")

	require.NoError(t, s.AddDynamicProfile("dyn13", profiles.Profile{
		RPCURL: "http://example", ChainID: "test-13", TxIndexerURL: "http://idx.example/graphql",
	}))
	registerAllTools(deps)

	faucet, ok := s.Registry().Get("gno_faucet_fund")
	require.True(t, ok, "adding a testnet profile must summon the faucet tool")
	assert.Contains(t, profileEnumOf(t, faucet), "dyn13", "faucet profile enum must list the dynamic profile")

	_, ok = s.Registry().Get("gno_list")
	require.True(t, ok, "adding an indexer-bearing profile must summon the indexer tools")

	render, ok := s.Registry().Get("gno_render")
	require.True(t, ok)
	assert.Contains(t, profileEnumOf(t, render), "dyn13", "read-tool profile enum must be regenerated")
}

func profileEnumOf(t *testing.T, tool *server.Tool) []string {
	t.Helper()
	props, ok := tool.InputSchema["properties"].(map[string]any)
	require.True(t, ok, "tool %s: schema has no properties", tool.Name)
	prof, ok := props["profile"].(map[string]any)
	require.True(t, ok, "tool %s: no profile property", tool.Name)
	enum, ok := prof["enum"].([]string)
	require.True(t, ok, "tool %s: profile enum is %T", tool.Name, props["profile"])
	return enum
}

// ---- in-process MCP end-to-end

func TestDynamicProfileAdd_endToEnd(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s := localOnlyServer(t)
	fake := chain.NewFake()
	fake.SetRender("gno.land/r/demo/home", "", "# dyn hello")
	deps := newDynDeps(t, s, func(profile string) chain.Client {
		if profile == "dyn13" {
			return fake
		}
		return nil
	})

	mcpServer := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "gnomcp-test", Version: "dev"}, nil)
	var republishMu sync.Mutex
	deps.onProfileAdded = func() error {
		republishMu.Lock()
		defer republishMu.Unlock()
		registerAllTools(deps)
		return publishTools(mcpServer, s, deps.auditLog, false)
	}
	registerAllTools(deps)
	require.NoError(t, publishTools(mcpServer, s, deps.auditLog, false))

	clientTr, serverTr := mcpsdk.NewInMemoryTransports()
	_, err := mcpServer.Connect(ctx, serverTr, nil)
	require.NoError(t, err)

	changed := make(chan struct{}, 16)
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "e2e-client", Version: "dev"}, &mcpsdk.ClientOptions{
		ToolListChangedHandler: func(context.Context, *mcpsdk.ToolListChangedRequest) {
			changed <- struct{}{}
		},
	})
	cs, err := client.Connect(ctx, clientTr, nil)
	require.NoError(t, err)
	defer cs.Close()

	list1, err := cs.ListTools(ctx, nil)
	require.NoError(t, err)
	require.NotContains(t, wireToolNames(list1), "gno_faucet_fund", "faucet tool must be absent on a local-only config")
	require.Contains(t, wireToolNames(list1), "gno_profile_add")
	require.NotContains(t, wireProfileEnum(t, list1, "gno_render"), "dyn13")

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{Name: "gno_profile_add", Arguments: map[string]any{
		"name": "dyn13", "rpc_url": "https://rpc.example", "chain_id": "test-13",
	}})
	require.NoError(t, err)
	require.False(t, res.IsError, "gno_profile_add failed: %s", wireText(res))

	select {
	case <-changed:
	case <-ctx.Done():
		t.Fatal("no tools/list_changed notification received after the dynamic add")
	}

	list2, err := cs.ListTools(ctx, nil)
	require.NoError(t, err)
	assert.Contains(t, wireToolNames(list2), "gno_faucet_fund", "testnet gate must summon the faucet tool mid-session")
	assert.Contains(t, wireProfileEnum(t, list2, "gno_render"), "dyn13", "published enum must include the dynamic profile")

	rr, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{Name: "gno_render", Arguments: map[string]any{
		"realm": "gno.land/r/demo/home", "profile": "dyn13",
	}})
	require.NoError(t, err)
	require.False(t, rr.IsError, "gno_render on the dynamic profile failed: %s", wireText(rr))
	b, err := json.Marshal(rr.Content)
	require.NoError(t, err)
	assert.Contains(t, string(b), "dyn hello", "render via the dynamic profile must return the chain content")
}

func wireToolNames(l *mcpsdk.ListToolsResult) []string {
	names := make([]string, 0, len(l.Tools))
	for _, tl := range l.Tools {
		names = append(names, tl.Name)
	}
	return names
}

// wireProfileEnum extracts the profile enum of the named tool from the wire
// tool list (schema round-tripped through JSON).
func wireProfileEnum(t *testing.T, l *mcpsdk.ListToolsResult, name string) []string {
	t.Helper()
	for _, tl := range l.Tools {
		if tl.Name != name {
			continue
		}
		b, err := json.Marshal(tl.InputSchema)
		require.NoError(t, err)
		var schema struct {
			Properties struct {
				Profile struct {
					Enum []string `json:"enum"`
				} `json:"profile"`
			} `json:"properties"`
		}
		require.NoError(t, json.Unmarshal(b, &schema))
		return schema.Properties.Profile.Enum
	}
	t.Fatalf("tool %q not in wire list", name)
	return nil
}

func wireText(res *mcpsdk.CallToolResult) string {
	var sb strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcpsdk.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}
