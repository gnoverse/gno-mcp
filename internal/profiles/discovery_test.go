package profiles

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDiscoverLocal_reachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": map[string]any{
				"node_info": map[string]any{"network": "dev"},
			},
		})
	}))
	defer srv.Close()

	p := Profile{RPCURL: srv.URL, ChainID: "dev"}
	ok, err := DiscoverLocal(context.Background(), p, 2*time.Second)
	require.NoError(t, err)
	require.True(t, ok, "expected ok=true for reachable matching chain-id")
}

func TestDiscoverLocal_chainIDMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": map[string]any{
				"node_info": map[string]any{"network": "actually-different"},
			},
		})
	}))
	defer srv.Close()

	p := Profile{RPCURL: srv.URL, ChainID: "dev"}
	ok, err := DiscoverLocal(context.Background(), p, 2*time.Second)
	require.NoError(t, err)
	require.False(t, ok, "expected ok=false for chain-id mismatch")
}

func TestDiscoverLocal_unreachable(t *testing.T) {
	p := Profile{RPCURL: "http://127.0.0.1:1", ChainID: "dev"}
	ok, _ := DiscoverLocal(context.Background(), p, 250*time.Millisecond)
	require.False(t, ok, "expected ok=false for unreachable endpoint")
}

func TestDiscoverLocal_skipsNonLocal(t *testing.T) {
	p := Profile{RPCURL: "https://rpc.test5.gno.land:443", ChainID: "test5"}
	ok, err := DiscoverLocal(context.Background(), p, 250*time.Millisecond)
	require.NoError(t, err)
	require.False(t, ok, "DiscoverLocal should always return false for non-local profiles")
}
