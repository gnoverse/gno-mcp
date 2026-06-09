package clientfaucet

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/chain"
)

func TestServiceFaucet_Fund(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /fund", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"tx_hash": "0xabc", "amount_ugnot": 1_000_000})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	sf := &ServiceFaucet{url: srv.URL, http: srv.Client(), chain: chain.NewFake()}
	out, err := sf.Fund(context.Background(), "g1abc", "test5")
	require.NoError(t, err)
	assert.Equal(t, "service", out.Backend)
	assert.Equal(t, "0xabc", out.TxHash)
	assert.Equal(t, "g1abc", out.Address)
}

func TestServiceFaucet_rateLimited(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /fund", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "busy", http.StatusTooManyRequests)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	sf := &ServiceFaucet{url: srv.URL, http: srv.Client(), chain: chain.NewFake()}
	_, err := sf.Fund(context.Background(), "g1abc", "test5")
	require.Error(t, err) // transient; the tool surfaces "faucet busy"
}
