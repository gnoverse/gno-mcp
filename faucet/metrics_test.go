package faucet

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantOut    string
	}{
		{"success", nil, http.StatusOK, OutcomeSuccess},
		{"bad address", ErrBadAddress, http.StatusBadRequest, OutcomeBadRequest},
		{"chain refused", ErrChainRefused, http.StatusForbidden, OutcomeChainRefused},
		{"chain mismatch", ErrChainMismatch, http.StatusForbidden, OutcomeChainMismatch},
		{"cooldown", ErrCooldown, http.StatusTooManyRequests, OutcomeCooldown},
		{"rate limited", ErrRateLimited, http.StatusTooManyRequests, OutcomeRateLimited},
		{"daily cap", ErrDailyCap, http.StatusTooManyRequests, OutcomeDailyCap},
		{"drip limited", ErrDripLimited, http.StatusTooManyRequests, OutcomeDripLimited},
		{"funding low", ErrFundingLow, http.StatusServiceUnavailable, OutcomeFundingLow},
		{"wrapped error is still matched", errors.New("internal boom"), http.StatusBadGateway, OutcomeDispenseFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, out := classify(tt.err)
			assert.Equal(t, tt.wantStatus, status)
			assert.Equal(t, tt.wantOut, out)
		})
	}
}
