package chain

import (
	"context"
	"testing"

	"github.com/gnolang/gno/tm2/pkg/std"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFakeAccount_returnsSeededState(t *testing.T) {
	f := NewFake()
	f.SetAccount("g1seeded", AccountInfo{
		Exists:        true,
		Coins:         std.Coins{{Denom: "ugnot", Amount: 250000}},
		Sequence:      4,
		AccountNumber: 57,
	})

	info, err := f.Account(context.Background(), "g1seeded")
	require.NoError(t, err)
	assert.True(t, info.Exists)
	assert.Equal(t, int64(250000), info.Coins.AmountOf("ugnot"))
	assert.Equal(t, uint64(4), info.Sequence)
	assert.Equal(t, uint64(57), info.AccountNumber)
}

func TestFakeAccount_unknownAddressIsNotExistsNotError(t *testing.T) {
	f := NewFake()

	info, err := f.Account(context.Background(), "g1neverfunded")
	require.NoError(t, err, "an address with no on-chain record is a normal answer, not an error")
	assert.False(t, info.Exists)
	assert.Empty(t, info.Coins)
}

func TestFakeAccount_seededErrorSurfaces(t *testing.T) {
	f := NewFake()
	f.SetAccountError("g1flaky", assert.AnError)

	_, err := f.Account(context.Background(), "g1flaky")
	require.ErrorIs(t, err, assert.AnError)
}
