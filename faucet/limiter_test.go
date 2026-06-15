package faucet

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLimiter(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := NewLimiter(LimiterCfg{
		Now:           func() time.Time { return now },
		PerAddrWindow: time.Hour, PerAddrMax: 1,
		PerIPWindow: time.Hour, PerIPMax: 2,
		DailyCapUgnot: 3_000_000, GrantUgnot: 1_000_000,
	})

	allow := func(addr, ip string) error { _, err := l.Allow(addr, ip); return err }

	require.NoError(t, allow("g1a", "1.1.1.1"))                 // 1st grant ok
	require.ErrorIs(t, allow("g1a", "1.1.1.2"), ErrCooldown)    // same addr within window
	require.NoError(t, allow("g1b", "1.1.1.1"))                 // 2nd hit on that IP ok
	require.ErrorIs(t, allow("g1c", "1.1.1.1"), ErrRateLimited) // 3rd hit on that IP blocked
	require.NoError(t, allow("g1d", "2.2.2.2"))                 // 3rd grant total = 3M, at cap
	require.ErrorIs(t, allow("g1e", "3.3.3.3"), ErrDailyCap)    // 4th grant would exceed 3M cap

	// next UTC day resets the daily counter (and windows have elapsed)
	now = now.Add(25 * time.Hour)
	require.NoError(t, allow("g1f", "4.4.4.4"), "new day resets daily cap")
}

func TestLimiter_globalDripBurstThenRefill(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := NewLimiter(LimiterCfg{
		Now:           func() time.Time { return now },
		PerAddrWindow: time.Hour, PerAddrMax: 100,
		PerIPWindow: time.Hour, PerIPMax: 100,
		DailyCapUgnot: 1_000_000_000, GrantUgnot: 1_000_000,
		// Bucket holds 2 grants; refills 100k/s (one grant per 10s).
		DripBurstUgnot: 2_000_000, DripRateUgnotPerSec: 100_000,
	})
	allow := func(addr string) error { _, err := l.Allow(addr, "1.1.1.1"); return err }

	require.NoError(t, allow("g1a"), "1st grant within burst")
	require.NoError(t, allow("g1b"), "2nd grant drains the burst")
	require.ErrorIs(t, allow("g1c"), ErrDripLimited, "burst spent, no tokens to drip")

	// Refill one grant's worth (10s * 100k/s = 1M).
	now = now.Add(10 * time.Second)
	require.NoError(t, allow("g1d"), "drip refilled one grant")
	require.ErrorIs(t, allow("g1e"), ErrDripLimited, "only one grant refilled")
}

func TestLimiter_globalDripRefundOnFailure(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := NewLimiter(LimiterCfg{
		Now:           func() time.Time { return now },
		PerAddrWindow: time.Hour, PerAddrMax: 100,
		PerIPWindow: time.Hour, PerIPMax: 100,
		DailyCapUgnot: 1_000_000_000, GrantUgnot: 1_000_000,
		DripBurstUgnot: 1_000_000, DripRateUgnotPerSec: 1, // burst = exactly one grant
	})

	grantedAt, err := l.Allow("g1a", "1.1.1.1")
	require.NoError(t, err, "1st grant drains the single-grant burst")
	_, err = l.Allow("g1b", "1.1.1.1")
	require.ErrorIs(t, err, ErrDripLimited, "burst spent")

	// The grant's dispense fails and refunds: the bucket must be credited back so
	// a chain hiccup doesn't permanently consume drip capacity.
	l.Refund("g1a", "1.1.1.1", grantedAt)
	_, err = l.Allow("g1c", "1.1.1.1")
	require.NoError(t, err, "refund restores the drip token")
}

// A refund must reconstruct the exact counterfactual "this grant never happened"
// state of the drip bucket — restoring the spent tokens AND accounting for real
// refill since the charge, without inflating the bucket above that counterfactual.
func TestLimiter_globalDripRefundIsConservative(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := NewLimiter(LimiterCfg{
		Now:           func() time.Time { return now },
		PerAddrWindow: time.Hour, PerAddrMax: 100,
		PerIPWindow: time.Hour, PerIPMax: 100,
		DailyCapUgnot: 1_000_000_000, GrantUgnot: 100,
		DripBurstUgnot: 1000, DripRateUgnotPerSec: 10,
	})

	// Two grants drain the bucket to 800 (start 1000, -100, -100).
	_, err := l.Allow("a", "ip")
	require.NoError(t, err)
	grantedAt, err := l.Allow("b", "ip")
	require.NoError(t, err)

	// 5s later b's dispense fails and is refunded.
	now = now.Add(5 * time.Second)
	l.Refund("b", "ip", grantedAt)

	// Counterfactual (only a ever charged): min(burst, (1000-100) + 5*10) = 950.
	// If the refund double-counted elapsed time it would read higher.
	require.Equal(t, float64(950), l.dripTokens, "refund must restore the counterfactual, not inflate the bucket")
}

func TestLimiter_evictsStaleKeysOnDayRollover(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := NewLimiter(LimiterCfg{
		Now:           func() time.Time { return now },
		PerAddrWindow: time.Hour, PerAddrMax: 1,
		PerIPWindow: time.Hour, PerIPMax: 100,
		DailyCapUgnot: 1_000_000_000, GrantUgnot: 1,
	})
	allow := func(addr, ip string) error { _, err := l.Allow(addr, ip); return err }
	for _, a := range []string{"g1a", "g1b", "g1c", "g1d"} {
		require.NoError(t, allow(a, "1.1.1.1"))
	}

	// Advance past a full day so every recorded hit is stale and the day rolls over.
	now = now.Add(48 * time.Hour)
	require.NoError(t, allow("g1z", "2.2.2.2"))

	require.LessOrEqual(t, len(l.addrHits), 1, "stale address keys must be evicted on day rollover")
	require.LessOrEqual(t, len(l.ipHits), 1, "stale IP keys must be evicted on day rollover")
}

func TestLimiter_refundAcrossDayBoundaryDoesNotUndercountNewDay(t *testing.T) {
	now := time.Date(2026, 1, 1, 23, 59, 0, 0, time.UTC)
	l := NewLimiter(LimiterCfg{
		Now:           func() time.Time { return now },
		PerAddrWindow: time.Minute, PerAddrMax: 100,
		PerIPWindow: time.Minute, PerIPMax: 100,
		DailyCapUgnot: 1_000_000, GrantUgnot: 1_000_000, // exactly one grant per day
	})

	// Grant on day D (consumes day D's single-grant cap).
	grantedAt, err := l.Allow("g1a", "1.1.1.1")
	require.NoError(t, err)

	// Roll over to day D+1 and grant once (consumes the new day's cap).
	now = time.Date(2026, 1, 2, 0, 1, 0, 0, time.UTC)
	_, err = l.Allow("g1b", "2.2.2.2")
	require.NoError(t, err, "new day permits one fresh grant")

	// The day-D grant's dispense fails and refunds after the rollover. The reset
	// already discarded its contribution, so the refund must not credit day D+1.
	l.Refund("g1a", "1.1.1.1", grantedAt)

	_, err = l.Allow("g1c", "3.3.3.3")
	require.ErrorIs(t, err, ErrDailyCap, "cross-day refund must not free a slot on the new day")
}
