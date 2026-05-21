package write

import (
	"testing"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/gnoverse/gno-mcp/internal/server"
	"github.com/gnoverse/gno-mcp/internal/session"
)

// newBaseTestServer builds a Server with one "testnet5" profile that has
// AllowDangerousTools=true. No tools registered; callers register only the
// tool under test.
func newBaseTestServer(t *testing.T) *server.Server {
	t.Helper()
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"testnet5": {
			ChainType:           "testnet",
			RPCURL:              "x",
			ChainID:             "test5",
			AllowDangerousTools: true,
			MasterAddress:       "g17ernafy6ctpcz6uepfsq2js8x2vz0wladh5yc3",
		},
	}}
	if _, err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	return server.NewServer(cfg, "")
}

// newReadOnlyTestServer builds a Server with one "testnet5" profile that has
// AllowDangerousTools=false. Used to exercise dangerous_disabled error paths.
func newReadOnlyTestServer(t *testing.T) *server.Server {
	t.Helper()
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"testnet5": {
			ChainType:           "testnet",
			RPCURL:              "x",
			ChainID:             "test5",
			AllowDangerousTools: false,
		},
	}}
	if _, err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	return server.NewServer(cfg, "")
}

// constSessionMgr returns a session.Manager pre-seeded via f. f receives
// the freshly constructed manager and may add sessions before it is used.
func constSessionMgr(t *testing.T, f func(*session.Manager)) *session.Manager {
	t.Helper()
	mgr := session.NewManager(t.TempDir(), "")
	if f != nil {
		f(mgr)
	}
	return mgr
}

// noSessionMgr returns an empty Manager rooted at a temp directory.
func noSessionMgr(t *testing.T) *session.Manager {
	t.Helper()
	return session.NewManager(t.TempDir(), "")
}

// constChainResolver returns a chain.Resolver that yields c regardless of
// the profile argument.
func constChainResolver(c chain.Client) chain.Resolver {
	return func(_ string) chain.Client { return c }
}

// onlyProfileChainResolver returns a chain.Resolver that yields c for the
// given profile name and nil for anything else.
func onlyProfileChainResolver(name string, c chain.Client) chain.Resolver {
	return func(profile string) chain.Client {
		if profile == name {
			return c
		}
		return nil
	}
}
