package main

import (
	"context"
	"encoding/json"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/gnoverse/gno-mcp/internal/server"
)

// TestBuildChainResolver_unknownProfileReturnsUntypedNil pins the fix for the
// crash: an unresolved profile previously returned a typed-nil *chain.Real boxed
// in a non-nil chain.Client, so every handler's `if c == nil` guard was bypassed
// and the next method call (e.g. Doc) segfaulted, killing the server. resolve
// must return an untyped-nil interface so the guards work.
func TestBuildChainResolver_unknownProfileReturnsUntypedNil(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"testnet": {RPCURL: "http://example", ChainID: "test-13"},
	}}
	resolve := buildChainResolver(server.NewServer(cfg, ""))

	if c := resolve("ghost"); c != nil {
		t.Fatalf("unknown profile must resolve to an untyped-nil interface; got typed-nil %T", c)
	}
	if c := resolve("testnet"); c == nil {
		t.Fatal("known profile must resolve to a non-nil client")
	}
}

// TestApplyProfileDefault pins the server-side default: JSON-Schema defaults are
// advisory (clients don't inject them), so when a tool accepts `profile` and the
// caller omits it, the server fills in its documented default rather than passing
// "" downstream (which resolves to nothing and used to crash).
func TestApplyProfileDefault(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"testnet": {RPCURL: "http://x", ChainID: "test-13"},
		"local":   {RPCURL: "http://y", ChainID: "dev"},
	}}
	s := server.NewServer(cfg, "") // no discovered-local -> default "testnet"

	withProfile := &server.Tool{Name: "t", InputSchema: map[string]any{
		"properties": map[string]any{"profile": map[string]any{"type": "string"}},
	}}
	noProfile := &server.Tool{Name: "n", InputSchema: map[string]any{
		"properties": map[string]any{},
	}}

	// omitted -> default applied
	a := map[string]any{}
	applyProfileDefault(withProfile, s, a)
	require.Equal(t, "testnet", a["profile"])

	// empty string -> default applied
	a = map[string]any{"profile": ""}
	applyProfileDefault(withProfile, s, a)
	require.Equal(t, "testnet", a["profile"])

	// explicit -> unchanged
	a = map[string]any{"profile": "local"}
	applyProfileDefault(withProfile, s, a)
	require.Equal(t, "local", a["profile"])

	// tool without a profile arg -> no injection
	a = map[string]any{}
	applyProfileDefault(noProfile, s, a)
	_, present := a["profile"]
	require.False(t, present, "must not inject profile into a tool that doesn't accept it")

	// malformed (non-string) profile -> left alone for the handler to reject
	a = map[string]any{"profile": 42}
	applyProfileDefault(withProfile, s, a)
	require.Equal(t, 42, a["profile"], "non-string profile must not be overwritten by the default")
}

// TestMakeHandler_recoversAdapterPanic pins the SDK-boundary safety net. The MCP
// SDK runs this handler on a goroutine with no recover of its own, so an
// unrecovered panic in the adapter crashes the whole server. A nil config makes
// applyProfileDefault -> ProfileSchema panic *before* Registry.Call, exercising
// makeHandler's own recover rather than the registry's.
func TestMakeHandler_recoversAdapterPanic(t *testing.T) {
	s := server.NewServer(nil, "")
	tool := &server.Tool{
		Name:        "x",
		Capability:  server.CapBaseRead,
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
	}
	s.Registry().Add(tool)
	h := makeHandler(tool, s, nil, false)

	res, err := h(context.Background(), &mcpsdk.CallToolRequest{
		Params: &mcpsdk.CallToolParamsRaw{Name: "x", Arguments: json.RawMessage(`{}`)},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.True(t, res.IsError, "adapter panic must degrade to an error result, not crash the server")
}
