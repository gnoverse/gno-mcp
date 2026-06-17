package main

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/faucet"
)

func TestSetupTelemetryOutcomeCounter(t *testing.T) {
	tel, err := setupTelemetry(context.Background(), "test")
	require.NoError(t, err)
	defer func() { _ = tel.shutdown(context.Background()) }()

	// Record a couple of outcomes through the faucet.Metrics seam.
	tel.metrics.RecordOutcome(context.Background(), faucet.OutcomeSuccess)
	tel.metrics.RecordOutcome(context.Background(), faucet.OutcomeDripLimited)

	// Scrape the exposed handler and assert the metric family + labels appear.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	tel.handler.ServeHTTP(rec, req)
	body := rec.Body.String()

	require.Contains(t, body, "faucet_fund_requests_total")
	require.Contains(t, body, `outcome="success"`)
	require.Contains(t, body, `outcome="drip_limited"`)
}
