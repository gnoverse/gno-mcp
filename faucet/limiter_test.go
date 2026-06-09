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

	require.NoError(t, l.Allow("g1a", "1.1.1.1"))                 // 1st grant ok
	require.ErrorIs(t, l.Allow("g1a", "1.1.1.2"), ErrCooldown)    // same addr within window
	require.NoError(t, l.Allow("g1b", "1.1.1.1"))                 // 2nd hit on that IP ok
	require.ErrorIs(t, l.Allow("g1c", "1.1.1.1"), ErrRateLimited) // 3rd hit on that IP blocked
	require.NoError(t, l.Allow("g1d", "2.2.2.2"))                 // 3rd grant total = 3M, at cap
	require.ErrorIs(t, l.Allow("g1e", "3.3.3.3"), ErrDailyCap)    // 4th grant would exceed 3M cap

	// next UTC day resets the daily counter (and windows have elapsed)
	now = now.Add(25 * time.Hour)
	require.NoError(t, l.Allow("g1f", "4.4.4.4"), "new day resets daily cap")
}
