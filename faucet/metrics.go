package faucet

import (
	"context"
	"errors"
	"net/http"
)

// Outcome values are bounded, low-cardinality labels for the fund-request
// counter; they also appear in the per-request access log.
const (
	OutcomeSuccess        = "success"
	OutcomeBadRequest     = "bad_request"
	OutcomeChainRefused   = "chain_refused"
	OutcomeChainMismatch  = "chain_mismatch"
	OutcomeCooldown       = "cooldown"
	OutcomeRateLimited    = "rate_limited"
	OutcomeDailyCap       = "daily_cap"
	OutcomeDripLimited    = "drip_limited"
	OutcomeFundingLow     = "funding_low"
	OutcomeDispenseFailed = "dispense_failed"
)

// Metrics records faucet business outcomes. Implementations must be safe for
// concurrent use. The default is a no-op; cmd/agentfaucet supplies an
// OTel-backed implementation. Defined with stdlib-only signatures so the faucet
// package stays free of any telemetry dependency.
type Metrics interface {
	RecordOutcome(ctx context.Context, outcome string)
}

type nopMetrics struct{}

func (nopMetrics) RecordOutcome(context.Context, string) {}

// classify maps a Fund result to the HTTP status returned to the caller and the
// bounded outcome label. A nil error is success; an unrecognized error is an
// internal dispense failure (502), whose detail is logged but not returned.
func classify(err error) (status int, outcome string) {
	switch {
	case err == nil:
		return http.StatusOK, OutcomeSuccess
	case errors.Is(err, ErrBadAddress):
		return http.StatusBadRequest, OutcomeBadRequest
	case errors.Is(err, ErrChainRefused):
		return http.StatusForbidden, OutcomeChainRefused
	case errors.Is(err, ErrChainMismatch):
		return http.StatusForbidden, OutcomeChainMismatch
	case errors.Is(err, ErrCooldown):
		return http.StatusTooManyRequests, OutcomeCooldown
	case errors.Is(err, ErrRateLimited):
		return http.StatusTooManyRequests, OutcomeRateLimited
	case errors.Is(err, ErrDailyCap):
		return http.StatusTooManyRequests, OutcomeDailyCap
	case errors.Is(err, ErrDripLimited):
		return http.StatusTooManyRequests, OutcomeDripLimited
	case errors.Is(err, ErrFundingLow):
		return http.StatusServiceUnavailable, OutcomeFundingLow
	default:
		return http.StatusBadGateway, OutcomeDispenseFailed
	}
}
