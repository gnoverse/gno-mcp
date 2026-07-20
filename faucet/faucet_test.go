package faucet

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validAddr is a real bech32 address (the well-known test1 account); the faucet
// now parses recipients, so test fixtures must use a genuine address.
const validAddr = "g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5"

type fakeDispenser struct {
	calls   int
	lastTo  string
	lastAmt int64
	err     error // when set, Send returns it (simulates a chain failure)
}

func (f *fakeDispenser) Send(_ context.Context, to string, amt int64) (string, error) {
	f.calls++
	f.lastTo = to
	f.lastAmt = amt
	if f.err != nil {
		return "", f.err
	}
	return "0xhash", nil
}

func TestFaucet_Fund(t *testing.T) {
	fd := &fakeDispenser{}
	f := New("test5", 1_000_000, fd, NewLimiter(LimiterCfg{
		PerAddrMax: 1, PerIPMax: 5, DailyCapUgnot: 1_000_000_000, GrantUgnot: 1_000_000,
	}))

	tx, err := f.Fund(context.Background(), validAddr, "1.1.1.1", "test5")
	require.NoError(t, err)
	assert.Equal(t, "0xhash", tx)
	assert.Equal(t, 1, fd.calls)
	assert.Equal(t, validAddr, fd.lastTo)
	assert.Equal(t, int64(1_000_000), fd.lastAmt)

	_, err = f.Fund(context.Background(), validAddr, "9.9.9.9", "test5")
	require.ErrorIs(t, err, ErrCooldown, "second request, same address, blocked")

	_, err = f.Fund(context.Background(), "g1xyz", "1.1.1.1", "mainnet")
	require.ErrorIs(t, err, ErrChainMismatch, "chain this faucet does not serve refused")

	_, err = f.Fund(context.Background(), "g1xyz", "1.1.1.1", "test99")
	require.ErrorIs(t, err, ErrChainMismatch, "test chain but not this faucet's chain")
}

// The faucet imposes no chain-id naming scheme — the operator's -chain-id is
// authoritative (a codenamed testnet must not require a faucet release naming
// it). The only chain guard is equality with the configured chain-id.
func TestFaucet_Fund_codenamedChain(t *testing.T) {
	fd := &fakeDispenser{}
	f := New("somenet-1", 1_000_000, fd, NewLimiter(LimiterCfg{
		PerAddrMax: 1, PerIPMax: 5, DailyCapUgnot: 1_000_000_000, GrantUgnot: 1_000_000,
	}))

	tx, err := f.Fund(context.Background(), validAddr, "1.1.1.1", "somenet-1")
	require.NoError(t, err)
	assert.Equal(t, "0xhash", tx)

	_, err = f.Fund(context.Background(), "g1xyz", "1.1.1.1", "othernet-1")
	require.ErrorIs(t, err, ErrChainMismatch, "any non-matching chain-id refused")
}

func TestFaucet_caseVariantAddressSharesCooldownBucket(t *testing.T) {
	fd := &fakeDispenser{}
	f := New("test5", 1_000_000, fd, NewLimiter(LimiterCfg{
		PerAddrMax: 1, PerIPMax: 100, DailyCapUgnot: 1_000_000_000, GrantUgnot: 1_000_000,
	}))

	_, err := f.Fund(context.Background(), validAddr, "1.1.1.1", "test5")
	require.NoError(t, err)

	// bech32 accepts an all-uppercase variant of the same address; it must hit
	// the same limiter bucket as the lowercase form, not a fresh one.
	_, err = f.Fund(context.Background(), strings.ToUpper(validAddr), "9.9.9.9", "test5")
	require.ErrorIs(t, err, ErrCooldown, "uppercase variant must share the canonical cooldown bucket")
	assert.Equal(t, 1, fd.calls, "no second dispense for a case-variant of an already-funded address")
}

func TestFaucet_badAddressRejectedBeforeDispense(t *testing.T) {
	fd := &fakeDispenser{}
	f := New("test5", 1_000_000, fd, NewLimiter(LimiterCfg{
		PerAddrMax: 1, PerIPMax: 100, DailyCapUgnot: 1_000_000_000, GrantUgnot: 1_000_000,
	}))

	_, err := f.Fund(context.Background(), "not-a-bech32-address", "1.1.1.1", "test5")
	require.Error(t, err, "garbage recipient must be rejected")
	assert.Equal(t, 0, fd.calls, "dispenser must not run for a bad address")
}

func TestFaucet_badAddressDoesNotBurnDailyCap(t *testing.T) {
	fd := &fakeDispenser{}
	// Daily cap permits exactly one grant.
	f := New("test5", 1_000_000, fd, NewLimiter(LimiterCfg{
		PerAddrMax: 1, PerIPMax: 100, DailyCapUgnot: 1_000_000, GrantUgnot: 1_000_000,
	}))

	for range 5 {
		_, _ = f.Fund(context.Background(), "garbage", "1.1.1.1", "test5")
	}
	_, err := f.Fund(context.Background(), validAddr, "1.1.1.1", "test5")
	require.NoError(t, err, "bad-address requests must not consume the daily cap")
}

func TestFaucet_refusesWhenFundingBalanceBelowFloor(t *testing.T) {
	fd := &fakeDispenser{}
	bal := int64(1_000_000) // below the floor
	f := New("test5", 1_000_000, fd, NewLimiter(LimiterCfg{
		PerAddrMax: 1, PerIPMax: 100, DailyCapUgnot: 1_000_000_000, GrantUgnot: 1_000_000,
	}), WithBalanceFloor(5_000_000, func(context.Context) (int64, error) { return bal, nil }))

	_, err := f.Fund(context.Background(), validAddr, "1.1.1.1", "test5")
	require.ErrorIs(t, err, ErrFundingLow, "drained funding wallet must refuse")
	assert.Equal(t, 0, fd.calls, "no dispense when funding is below floor")

	// The floor check must run before the limiter, leaving the address fundable
	// once the wallet is topped up.
	bal = 5_000_000
	_, err = f.Fund(context.Background(), validAddr, "1.1.1.1", "test5")
	require.NoError(t, err, "above the floor, the same address funds normally")
	assert.Equal(t, 1, fd.calls)
}

func TestFaucet_refundsCooldownAndCapOnDispenseFailure(t *testing.T) {
	fd := &fakeDispenser{err: errors.New("chain hiccup")}
	f := New("test5", 1_000_000, fd, NewLimiter(LimiterCfg{
		PerAddrMax: 1, PerIPMax: 100, DailyCapUgnot: 1_000_000, GrantUgnot: 1_000_000,
	}))

	_, err := f.Fund(context.Background(), validAddr, "1.1.1.1", "test5")
	require.Error(t, err, "dispense failure should surface")

	// The failed attempt must be refunded: same address fundable, cap intact.
	fd.err = nil
	_, err = f.Fund(context.Background(), validAddr, "1.1.1.1", "test5")
	require.NoError(t, err, "a refunded failure must leave the address fundable and the cap unspent")
}

func TestFaucet_Limits(t *testing.T) {
	f := New("test5", 10_000_000, &fakeDispenser{}, NewLimiter(LimiterCfg{
		PerAddrWindow: 24 * time.Hour, PerAddrMax: 1,
		PerIPMax: 5, DailyCapUgnot: 1_000_000_000, GrantUgnot: 10_000_000,
	}))
	lim := f.Limits()
	assert.Equal(t, int64(10_000_000), lim.GrantUgnot)
	assert.Equal(t, 1, lim.PerAddress.Max)
	assert.Equal(t, 86400, lim.PerAddress.WindowSeconds)
}

func TestFaucet_retryAfterSeconds_floor(t *testing.T) {
	// default 24h window -> floored at 24h
	f := New("test5", 1, &fakeDispenser{}, NewLimiter(LimiterCfg{
		PerAddrWindow: 24 * time.Hour, PerAddrMax: 1, PerIPMax: 5,
		DailyCapUgnot: 1, GrantUgnot: 1,
	}))
	assert.Equal(t, 86400, f.retryAfterSeconds())

	// a window longer than the floor wins
	f2 := New("test5", 1, &fakeDispenser{}, NewLimiter(LimiterCfg{
		PerAddrWindow: 48 * time.Hour, PerAddrMax: 1, PerIPMax: 5,
		DailyCapUgnot: 1, GrantUgnot: 1,
	}))
	assert.Equal(t, 172800, f2.retryAfterSeconds())
}
