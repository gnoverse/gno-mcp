// internal/server/server_test.go
package server

import (
	"context"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/profiles"
)

func TestNewServer_registersZeroToolsInitially(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"testnet5": {ChainType: "testnet", RPCURL: "x", ChainID: "test5"},
	}}
	s := NewServer(cfg, "")
	if s.Registry().Count() != 0 {
		t.Errorf("expected 0 tools registered, got %d", s.Registry().Count())
	}
}

func TestServer_anyProfileHasIndexer(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"testnet5": {TxIndexerURL: "x"},
		"local":    {},
	}}
	s := NewServer(cfg, "")
	if !s.AnyProfileHasIndexer() {
		t.Error("AnyProfileHasIndexer should be true")
	}
}

func TestServer_noProfileHasIndexer(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"local": {},
	}}
	s := NewServer(cfg, "")
	if s.AnyProfileHasIndexer() {
		t.Error("AnyProfileHasIndexer should be false")
	}
}

func TestServer_anyProfileAllowsDangerous_true(t *testing.T) {
	cfg := &profiles.Config{
		Profiles: map[string]profiles.Profile{
			"safe":      {ChainType: "testnet", RPCURL: "http://x:26657", ChainID: "t", AllowDangerousTools: false},
			"dangerous": {ChainType: "local", RPCURL: "http://x:26657", ChainID: "dev", AllowDangerousTools: true},
		},
	}
	s := NewServer(cfg, "")
	if !s.AnyProfileAllowsDangerous() {
		t.Error("expected true when at least one profile has AllowDangerousTools=true")
	}
}

func TestServer_anyProfileAllowsDangerous_false(t *testing.T) {
	cfg := &profiles.Config{
		Profiles: map[string]profiles.Profile{
			"a": {ChainType: "testnet", RPCURL: "http://x:26657", ChainID: "t", AllowDangerousTools: false},
			"b": {ChainType: "testnet", RPCURL: "http://y:26657", ChainID: "t", AllowDangerousTools: false},
		},
	}
	s := NewServer(cfg, "")
	if s.AnyProfileAllowsDangerous() {
		t.Error("expected false when no profile has AllowDangerousTools=true")
	}
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
	if err != nil || res.Text != "hi" {
		t.Errorf("Call = %+v, %v", res, err)
	}
}
