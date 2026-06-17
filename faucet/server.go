package faucet

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
)

type fundRequest struct {
	Address string `json:"address"`
	ChainID string `json:"chain_id"`
}

type fundResponse struct {
	TxHash      string `json:"tx_hash"`
	AmountUgnot int64  `json:"amount_ugnot"`
}

// Handler returns the faucet's HTTP mux (POST /fund, GET /health).
func (f *Faucet) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /fund", f.handleFund)
	mux.HandleFunc("GET /health", f.handleHealth)
	return mux
}

// handleHealth is a liveness probe for the load balancer: 200 while the
// process is serving. It deliberately does not probe the chain — a transient
// RPC blip must not take the instance out of rotation.
func (f *Faucet) handleHealth(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("ok"))
}

func (f *Faucet) handleFund(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10) // tiny request; bound untrusted input
	var req fundRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Address == "" {
		http.Error(w, "bad request: expected JSON {address, chain_id}", http.StatusBadRequest)
		return
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr // fall back to the raw addr instead of collapsing to ""
	}

	tx, err := f.Fund(r.Context(), req.Address, ip, req.ChainID)
	switch {
	case err == nil:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(fundResponse{TxHash: tx, AmountUgnot: f.grantUgnot})
	case errors.Is(err, ErrBadAddress):
		http.Error(w, "bad request: invalid recipient address", http.StatusBadRequest)
	case errors.Is(err, ErrChainRefused), errors.Is(err, ErrChainMismatch):
		http.Error(w, err.Error(), http.StatusForbidden)
	case errors.Is(err, ErrCooldown), errors.Is(err, ErrRateLimited), errors.Is(err, ErrDailyCap), errors.Is(err, ErrDripLimited):
		http.Error(w, err.Error(), http.StatusTooManyRequests)
	case errors.Is(err, ErrFundingLow):
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	default:
		// Internal dispense failures (CheckTx logs, RPC transport details) aid
		// probing of a service that holds a funded key: log them, return a
		// generic error to the anonymous caller.
		log.Printf("faucet: dispense failed for %s: %v", req.Address, err)
		http.Error(w, "faucet: dispense failed", http.StatusBadGateway)
	}
}
