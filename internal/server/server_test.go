// internal/server/server_test.go
package server

import (
	"context"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewServer_registersZeroToolsInitially(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"testnet5": {ChainType: "testnet", RPCURL: "x", ChainID: "test5"},
	}}
	s := NewServer(cfg, "")
	assert.Equal(t, 0, s.Registry().Count())
}

func TestServer_anyProfileHasIndexer(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"testnet5": {TxIndexerURL: "x"},
		"local":    {},
	}}
	s := NewServer(cfg, "")
	assert.True(t, s.AnyProfileHasIndexer(), "AnyProfileHasIndexer should be true")
}

func TestServer_noProfileHasIndexer(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"local": {},
	}}
	s := NewServer(cfg, "")
	assert.False(t, s.AnyProfileHasIndexer(), "AnyProfileHasIndexer should be false")
}

func TestServer_anyProfileAgentCapable_local(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"dev": {ChainType: "local", RPCURL: "x", ChainID: "dev"},
	}}
	s := NewServer(cfg, "")
	assert.True(t, s.AnyProfileAgentCapable(), "AnyProfileAgentCapable should be true for a local profile")
}

func TestServer_anyProfileAgentCapable_testnet(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"testnet5": {ChainType: "testnet", RPCURL: "x", ChainID: "test5"},
	}}
	s := NewServer(cfg, "")
	assert.True(t, s.AnyProfileAgentCapable(), "AnyProfileAgentCapable should be true for a testnet profile")
}

func TestServer_anyProfileAgentCapable_both(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"dev":      {ChainType: "local", RPCURL: "x", ChainID: "dev"},
		"testnet5": {ChainType: "testnet", RPCURL: "y", ChainID: "test5"},
	}}
	s := NewServer(cfg, "")
	assert.True(t, s.AnyProfileAgentCapable(), "AnyProfileAgentCapable should be true when any profile is local or testnet")
}

func TestServer_noProfileAgentCapable(t *testing.T) {
	// A config with no profiles has nothing agent-capable (edge case).
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{}}
	s := NewServer(cfg, "")
	assert.False(t, s.AnyProfileAgentCapable(), "AnyProfileAgentCapable should be false with no profiles")
}

func TestServer_callsRegisteredTool(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"local": {RPCURL: "x", ChainID: "dev"},
	}}
	s := NewServer(cfg, "")
	s.Registry().Add(&Tool{
		Name: "x", Capability: CapBaseRead,
		Handler: func(ctx context.Context, args map[string]any) (Result, error) {
			return Result{Text: "hi"}, nil
		},
	})
	res, err := s.Registry().Call(context.Background(), "x", nil)
	require.NoError(t, err)
	assert.Equal(t, "hi", res.Text)
}
