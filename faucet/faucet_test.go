package faucet

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDispenser struct {
	calls   int
	lastTo  string
	lastAmt int64
}

func (f *fakeDispenser) Send(_ context.Context, to string, amt int64) (string, error) {
	f.calls++
	f.lastTo = to
	f.lastAmt = amt
	return "0xhash", nil
}

func TestFaucet_Fund(t *testing.T) {
	fd := &fakeDispenser{}
	f := New("test5", 1_000_000, fd, NewLimiter(LimiterCfg{
		PerAddrMax: 1, PerIPMax: 5, DailyCapUgnot: 1_000_000_000, GrantUgnot: 1_000_000,
	}))

	tx, err := f.Fund(context.Background(), "g1abc", "1.1.1.1", "test5")
	require.NoError(t, err)
	assert.Equal(t, "0xhash", tx)
	assert.Equal(t, 1, fd.calls)
	assert.Equal(t, "g1abc", fd.lastTo)
	assert.Equal(t, int64(1_000_000), fd.lastAmt)

	_, err = f.Fund(context.Background(), "g1abc", "9.9.9.9", "test5")
	require.ErrorIs(t, err, ErrCooldown, "second request, same address, blocked")

	_, err = f.Fund(context.Background(), "g1xyz", "1.1.1.1", "mainnet")
	require.ErrorIs(t, err, ErrChainRefused, "non-test chain refused")

	_, err = f.Fund(context.Background(), "g1xyz", "1.1.1.1", "test99")
	require.ErrorIs(t, err, ErrChainMismatch, "test chain but not this faucet's chain")
}
