package write

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/keystore"
	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/gnoverse/gno-mcp/internal/server"
)

func TestKeyGenerate_testnetProfile_returnsAddress(t *testing.T) {
	s := newTestnetTestServer(t)
	ks := keystore.New(t.TempDir(), "")
	RegisterKeyGenerate(s, ks)

	res, err := s.Registry().Call(context.Background(), "gno_key_generate", map[string]any{
		"profile": "testnet9999",
	})
	if err != nil {
		t.Fatalf("gno_key_generate: %v", err)
	}
	if !strings.HasPrefix(res.Text, "g1") {
		t.Errorf("Text = %q, want g1... address", res.Text)
	}
	sc := res.StructuredContent
	if sc == nil {
		t.Fatal("StructuredContent is nil")
	}
	addr, _ := sc["address"].(string)
	if addr != res.Text {
		t.Errorf("StructuredContent[address] = %q, want %q", addr, res.Text)
	}
}

func TestKeyGenerate_secondCall_keyAlreadyExists(t *testing.T) {
	s := newTestnetTestServer(t)
	ks := keystore.New(t.TempDir(), "")
	RegisterKeyGenerate(s, ks)

	if _, err := s.Registry().Call(context.Background(), "gno_key_generate", map[string]any{
		"profile": "testnet9999",
	}); err != nil {
		t.Fatalf("first call: %v", err)
	}

	_, err := s.Registry().Call(context.Background(), "gno_key_generate", map[string]any{
		"profile": "testnet9999",
	})
	if err == nil {
		t.Fatal("expected key_already_exists error, got nil")
	}
	var te *server.ToolError
	if !errors.As(err, &te) {
		t.Fatalf("expected *server.ToolError, got %T: %v", err, err)
	}
	if te.Code != "key_already_exists" {
		t.Errorf("Code = %q, want %q", te.Code, "key_already_exists")
	}
}

func TestKeyGenerate_noKeyDir_keyStorageUnconfigured(t *testing.T) {
	s := newTestnetTestServer(t)
	ks := keystore.New("", "")
	RegisterKeyGenerate(s, ks)

	_, err := s.Registry().Call(context.Background(), "gno_key_generate", map[string]any{
		"profile": "testnet9999",
	})
	if err == nil {
		t.Fatal("expected key_storage_unconfigured error, got nil")
	}
	var te *server.ToolError
	if !errors.As(err, &te) {
		t.Fatalf("expected *server.ToolError, got %T: %v", err, err)
	}
	if te.Code != "key_storage_unconfigured" {
		t.Errorf("Code = %q, want %q", te.Code, "key_storage_unconfigured")
	}
}

func TestKeyGenerate_localProfile_keyGenerationUnsupported(t *testing.T) {
	s := newLocalTestServerWithTestnet(t)
	ks := keystore.New(t.TempDir(), "")
	RegisterKeyGenerate(s, ks)

	_, err := s.Registry().Call(context.Background(), "gno_key_generate", map[string]any{
		"profile": "local",
	})
	if err == nil {
		t.Fatal("expected key_generation_unsupported error, got nil")
	}
	var te *server.ToolError
	if !errors.As(err, &te) {
		t.Fatalf("expected *server.ToolError, got %T: %v", err, err)
	}
	if te.Code != "key_generation_unsupported" {
		t.Errorf("Code = %q, want %q", te.Code, "key_generation_unsupported")
	}
}

// testnet9999Profile is the testnet profile fixture shared by the key-generate
// server builders.
func testnet9999Profile() profiles.Profile {
	return profiles.Profile{ChainType: profiles.ChainTypeTestnet, RPCURL: "x", ChainID: "test9999"}
}

// newTestnetServerFromProfiles validates ps and wraps it in a Server.
func newTestnetServerFromProfiles(t *testing.T, ps map[string]profiles.Profile) *server.Server {
	t.Helper()
	cfg := &profiles.Config{Profiles: ps}
	if _, err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	return server.NewServer(cfg, "")
}

// newTestnetTestServer builds a Server with one "testnet9999" profile (testnet
// tier, no master-address required for key generation).
func newTestnetTestServer(t *testing.T) *server.Server {
	t.Helper()
	return newTestnetServerFromProfiles(t, map[string]profiles.Profile{
		"testnet9999": testnet9999Profile(),
	})
}

// newLocalTestServerWithTestnet builds a Server with both a "local" profile
// and a "testnet9999" profile, used to test that local profiles are rejected.
func newLocalTestServerWithTestnet(t *testing.T) *server.Server {
	t.Helper()
	return newTestnetServerFromProfiles(t, map[string]profiles.Profile{
		"local":       {ChainType: profiles.ChainTypeLocal, RPCURL: "x", ChainID: "dev"},
		"testnet9999": testnet9999Profile(),
	})
}
