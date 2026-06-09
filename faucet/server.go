package faucet

import (
	"encoding/json"
	"errors"
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

// Handler returns the faucet's HTTP mux (POST /fund).
func (f *Faucet) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /fund", f.handleFund)
	return mux
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
	case errors.Is(err, ErrChainRefused), errors.Is(err, ErrChainMismatch):
		http.Error(w, err.Error(), http.StatusForbidden)
	case errors.Is(err, ErrCooldown), errors.Is(err, ErrRateLimited), errors.Is(err, ErrDailyCap):
		http.Error(w, err.Error(), http.StatusTooManyRequests)
	default:
		http.Error(w, err.Error(), http.StatusBadGateway)
	}
}
