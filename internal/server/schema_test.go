// internal/server/schema_test.go
package server

import (
	"testing"

	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProfileArgSchema_singleProfile_optionalDefault(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"testnet5": {RPCURL: "x", ChainID: "test5"},
	}}
	s := ProfileArgSchema(cfg, "" /* discovered */)
	assert.False(t, s.Required, "single profile: profile arg should be optional")
	assert.Equal(t, "testnet5", s.Default, "single profile: default should be the only profile")
}

func TestProfileArgSchema_multipleWithLocalDiscovered_optionalDefault(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"local":    {RPCURL: "x", ChainID: "dev"},
		"testnet5": {RPCURL: "y", ChainID: "test5"},
	}}
	s := ProfileArgSchema(cfg, "local")
	assert.False(t, s.Required, "local discovered: profile arg should be optional")
	assert.Equal(t, "local", s.Default, "local discovered: default should be 'local'")
}

func TestProfileArgSchema_staleDiscoveredLocalIgnored(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"testnet5": {RPCURL: "x", ChainID: "test5"},
		"mainnet":  {RPCURL: "y", ChainID: "portal-loop"},
	}}
	// Discovery returned a name that is not in the loaded config.
	// Stale name must not become the Default; falls back to smart default instead.
	s := ProfileArgSchema(cfg, "ghost")
	assert.False(t, s.Required, "stale discoveredLocal: profile arg should still be optional (smart default)")
	assert.NotEqual(t, "ghost", s.Default, "stale discoveredLocal must not become Default")
	require.NotEmpty(t, s.Default, "stale discoveredLocal: expected a smart default")
}

func TestProfileArgSchema_multipleNoLocal_smartDefault(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"testnet5": {RPCURL: "x", ChainID: "test5"},
		"mainnet":  {RPCURL: "y", ChainID: "portal-loop"},
	}}
	// No local discovered and no "testnet" key — falls back to first alphabetical profile.
	s := ProfileArgSchema(cfg, "")
	assert.False(t, s.Required, "multi + no local: profile arg should be optional (smart default)")
	assert.NotEmpty(t, s.Default, "multi + no local: expected a smart default")
}

func TestProfileArgSchema_SmartDefault(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"local":   {ChainID: "dev", RPCURL: "x"},
		"testnet": {ChainID: "test11", RPCURL: "y"},
	}}
	// local discovered → default local
	got := ProfileArgSchema(cfg, "local")
	assert.Equal(t, "local", got.Default, "with local discovered: default")
	assert.False(t, got.Required, "with local discovered: required")

	// local NOT discovered → default testnet, still optional
	got = ProfileArgSchema(cfg, "")
	assert.Equal(t, "testnet", got.Default, "without local: default should be testnet")
	assert.False(t, got.Required, "without local: required")
}
