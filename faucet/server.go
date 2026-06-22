package faucet

import (
	"encoding/json"
	"net/http"
	"strconv"
)

type fundRequest struct {
	Address string `json:"address"`
	ChainID string `json:"chain_id"`
}

type fundResponse struct {
	TxHash      string `json:"tx_hash"`
	AmountUgnot int64  `json:"amount_ugnot"`
}

// rateLimitResponse is the JSON body returned on every 429. Limit is set only
// for the per-address cooldown — the one limit safe to name, because a fresh key
// bypasses it. Per-IP, daily-cap, and drip stay generic (Limit empty) so the
// anti-abuse internals are not disclosed.
type rateLimitResponse struct {
	Error             string `json:"error"`
	Limit             string `json:"limit,omitempty"`
	RetryAfterSeconds int    `json:"retry_after_seconds"`
}

// Handler returns the faucet's HTTP mux (POST /fund, GET /health, GET /limits),
// wrapped with the access-log middleware.
func (f *Faucet) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /fund", f.handleFund)
	mux.HandleFunc("GET /health", f.handleHealth)
	mux.HandleFunc("GET /limits", f.handleLimits)
	return f.logRequests(mux)
}

// handleLimits publishes the faucet's static per-address policy (grant size,
// per-address max and window). Policy-only by construction (Faucet.Limits) — it
// carries no funding balance, day-spent, daily cap, per-IP, or drip state.
func (f *Faucet) handleLimits(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(f.Limits())
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
		f.recordFund(r, OutcomeBadRequest, "", "")
		http.Error(w, "bad request: expected JSON {address, chain_id}", http.StatusBadRequest)
		return
	}

	// The access-log middleware extracts the client IP once and stores it here.
	// Guard nil so handleFund stays safe even if ever mounted without it.
	var ip string
	if info := reqInfoFromContext(r.Context()); info != nil {
		ip = info.ip
	}

	tx, err := f.Fund(r.Context(), req.Address, ip, req.ChainID)
	status, outcome := classify(err)
	f.recordFund(r, outcome, req.Address, req.ChainID)

	switch {
	case err == nil:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(fundResponse{TxHash: tx, AmountUgnot: f.grantUgnot})
	case outcome == OutcomeDispenseFailed:
		// Internal dispense failures (CheckTx logs, RPC transport details) aid
		// probing of a service that holds a funded key: log them, return a
		// generic error to the anonymous caller.
		f.logger.Error("dispense failed", "address", req.Address, "err", err.Error())
		http.Error(w, "faucet: dispense failed", http.StatusBadGateway)
	case status == http.StatusTooManyRequests:
		retry := f.retryAfterSeconds()
		w.Header().Set("Retry-After", strconv.Itoa(retry))
		w.Header().Set("Content-Type", "application/json")
		rl := rateLimitResponse{Error: "rate limited", RetryAfterSeconds: retry}
		if outcome == OutcomeCooldown { // per-address — the one limit safe to name
			rl.Limit = "per_address"
		}
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(rl)
	default:
		http.Error(w, err.Error(), status)
	}
}

// recordFund enriches the access-log record and records the outcome metric.
func (f *Faucet) recordFund(r *http.Request, outcome, address, chainID string) {
	if info := reqInfoFromContext(r.Context()); info != nil {
		info.outcome = outcome
		info.address = address
		info.chainID = chainID
	}
	f.metrics.RecordOutcome(r.Context(), outcome)
}
