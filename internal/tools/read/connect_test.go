package read

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/gnoverse/gno-mcp/internal/server"
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
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if !strings.Contains(res.Text, "gnomcp profile add") || !strings.Contains(res.Text, "test11") {
		t.Errorf("expected add command with chain-id, got:\n%s", res.Text)
	}
}

func TestConnect_RejectsForbiddenChain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<meta name="gnoconnect:rpc" content="https://rpc.betanet.testnets.gno.land" />` +
			`<meta name="gnoconnect:chainid" content="gnoland1" />`))
	}))
	defer srv.Close()
	s := server.NewServer(&profiles.Config{Profiles: profiles.BuiltinProfiles()}, "")
	RegisterConnect(s, srv.Client())
	_, err := s.Registry().Call(context.Background(), "gno_connect", map[string]any{"gnoweb_url": srv.URL})
	if err == nil {
		t.Fatal("expected forbidden chain-id (gnoland1) to be rejected")
	}
}
