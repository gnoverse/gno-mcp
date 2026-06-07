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

func TestServer_anyProfileLocal(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"dev":      {ChainType: "local", RPCURL: "x", ChainID: "dev"},
		"testnet5": {ChainType: "testnet", RPCURL: "y", ChainID: "test5"},
	}}
	s := NewServer(cfg, "")
	if !s.AnyProfileLocal() {
		t.Error("AnyProfileLocal should be true")
	}
}

func TestServer_noProfileLocal(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"testnet5": {ChainType: "testnet", RPCURL: "x", ChainID: "test5"},
	}}
	s := NewServer(cfg, "")
	if s.AnyProfileLocal() {
		t.Error("AnyProfileLocal should be false")
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
