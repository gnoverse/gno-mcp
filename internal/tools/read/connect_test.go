package read

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/gnoverse/gno-mcp/internal/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnect_EmitsAddCommand(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<meta name="gnoconnect:rpc" content="https://rpc.test11.testnets.gno.land" />` +
			`<meta name="gnoconnect:chainid" content="test11" />`))
	}))
	defer srv.Close()

	s := server.NewServer(&profiles.Config{Profiles: profiles.BuiltinProfiles()}, "")
	RegisterConnect(s, srv.Client())
	res, err := s.Registry().Call(context.Background(), "gno_connect", map[string]any{
		"gnoweb_url": srv.URL, "name": "mychain",
	})
	require.NoError(t, err)
	assert.Contains(t, res.Text, "gnomcp profile add", "expected persist command in output")
	assert.Contains(t, res.Text, "test11", "expected chain-id in output")
	assert.Contains(t, res.Text, "gno_profile_add", "expected the in-session (dynamic add) path in output")
}

func TestConnect_RejectsInjectionInName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<meta name="gnoconnect:rpc" content="https://rpc.test11.testnets.gno.land" />` +
			`<meta name="gnoconnect:chainid" content="test11" />`))
	}))
	defer srv.Close()

	s := server.NewServer(&profiles.Config{Profiles: profiles.BuiltinProfiles()}, "")
	RegisterConnect(s, srv.Client())
	_, err := s.Registry().Call(context.Background(), "gno_connect", map[string]any{
		"gnoweb_url": srv.URL, "name": "evil; rm -rf /",
	})
	require.Error(t, err, "expected injection in name to be rejected before building the paste command")
}

func TestConnect_RejectsInjectionInDiscoveredRPC(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<meta name="gnoconnect:rpc" content="http://evil/$(whoami)" />` +
			`<meta name="gnoconnect:chainid" content="test11" />`))
	}))
	defer srv.Close()

	s := server.NewServer(&profiles.Config{Profiles: profiles.BuiltinProfiles()}, "")
	RegisterConnect(s, srv.Client())
	_, err := s.Registry().Call(context.Background(), "gno_connect", map[string]any{"gnoweb_url": srv.URL})
	require.Error(t, err, "expected a shell-unsafe discovered RPC to be rejected")
}

// A non-test chain (betanet gnoland1) is admitted read-only — auditing deployed
// source on gno.land requires reaching its chain. The result flags it read-only.
func TestConnect_AdmitsReadOnlyChain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<meta name="gnoconnect:rpc" content="https://rpc.betanet.testnets.gno.land" />` +
			`<meta name="gnoconnect:chainid" content="gnoland1" />`))
	}))
	defer srv.Close()

	s := server.NewServer(&profiles.Config{Profiles: profiles.BuiltinProfiles()}, "")
	RegisterConnect(s, srv.Client())
	res, err := s.Registry().Call(context.Background(), "gno_connect", map[string]any{"gnoweb_url": srv.URL})
	require.NoError(t, err, "betanet chain-id (gnoland1) must be admitted read-only")
	assert.Equal(t, true, res.StructuredContent["read_only"])
	assert.Contains(t, res.Text, "read-only")
}

// A discovered chain-id carrying shell metacharacters must still be refused —
// it is interpolated into the pasted profile-add command.
func TestConnect_RejectsMalformedChainID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<meta name="gnoconnect:rpc" content="https://rpc.example" />` +
			`<meta name="gnoconnect:chainid" content="evil$(id)" />`))
	}))
	defer srv.Close()

	s := server.NewServer(&profiles.Config{Profiles: profiles.BuiltinProfiles()}, "")
	RegisterConnect(s, srv.Client())
	_, err := s.Registry().Call(context.Background(), "gno_connect", map[string]any{"gnoweb_url": srv.URL})
	require.Error(t, err, "a shell-unsafe discovered chain-id must be rejected")
}
