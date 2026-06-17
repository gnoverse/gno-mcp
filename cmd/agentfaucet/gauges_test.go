package main

import (
	"context"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/faucet"
)

func TestRegisterGaugesScrape(t *testing.T) {
	tel, err := setupTelemetry(context.Background(), "test")
	require.NoError(t, err)
	defer func() { _ = tel.shutdown(context.Background()) }()

	lim := faucet.NewLimiter(faucet.LimiterCfg{
		PerAddrMax: 1, PerIPMax: 10, DailyCapUgnot: 1_000, GrantUgnot: 100,
		DripBurstUgnot: 500, DripRateUgnotPerSec: 10,
	})
	var balance atomic.Int64
	balance.Store(42_000)

	require.NoError(t, tel.registerGauges(lim, &balance))

	rec := httptest.NewRecorder()
	tel.handler.ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	body := rec.Body.String()

	require.Contains(t, body, "faucet_funding_balance_ugnot")
	require.Contains(t, body, "42000")
	require.Contains(t, body, "faucet_drip_tokens_available")
	require.Contains(t, body, "faucet_daily_cap_remaining_ugnot")
}

// An unseeded balance (-1 sentinel) must NOT be observed, so a never-polled
// balance reads as absent rather than a misleading 0.
func TestRegisterGaugesUnseededBalanceAbsent(t *testing.T) {
	tel, err := setupTelemetry(context.Background(), "test")
	require.NoError(t, err)
	defer func() { _ = tel.shutdown(context.Background()) }()

	lim := faucet.NewLimiter(faucet.LimiterCfg{
		PerAddrMax: 1, PerIPMax: 10, DailyCapUgnot: 1_000, GrantUgnot: 100,
	})
	var balance atomic.Int64
	balance.Store(-1) // never successfully polled

	require.NoError(t, tel.registerGauges(lim, &balance))

	rec := httptest.NewRecorder()
	tel.handler.ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	body := rec.Body.String()

	require.NotContains(t, body, "faucet_funding_balance_ugnot",
		"unseeded balance must be absent, not reported as 0")
	// the burst/cap gauges still report
	require.Contains(t, body, "faucet_daily_cap_remaining_ugnot")
}
