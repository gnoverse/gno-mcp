package chain

import (
	"testing"

	"github.com/gnolang/gno/gno.land/pkg/gnoland/ugnot"
	"github.com/gnolang/gno/tm2/pkg/amino"
	"github.com/gnolang/gno/tm2/pkg/std"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ugnotPrice(gas, amount int64) std.GasPrice {
	return std.GasPrice{Gas: gas, Price: std.Coin{Denom: ugnot.Denom, Amount: amount}}
}

func TestComputeGasFee(t *testing.T) {
	const floor = DefaultGasFeeUgnot // 10_000
	const gasWanted = DefaultGasWanted

	cases := []struct {
		name      string
		price     std.GasPrice
		gasWanted int64
		want      int64
		wantErr   bool
	}{
		{
			// Live test13 at 4ugnot/1000gas: min = 10M*4/1000 = 40_000, ×2 margin = 80_000.
			name:      "congested chain above floor",
			price:     ugnotPrice(1000, 4),
			gasWanted: gasWanted,
			want:      80_000,
		},
		{
			// Genesis floor 1ugnot/1000gas: min = 10_000, ×2 = 20_000 (still above the floor).
			name:      "chain at genesis floor",
			price:     ugnotPrice(1000, 1),
			gasWanted: gasWanted,
			want:      20_000,
		},
		{
			// Non-exact division must round UP, else the offered fee underpays by a hair.
			// min = ceil(10_000_000 / 3) = 3_333_334, ×2 = 6_666_668.
			name:      "ceil rounding",
			price:     ugnotPrice(3, 1),
			gasWanted: gasWanted,
			want:      6_666_668,
		},
		{
			// A zero/empty on-chain price means the dynamic gate is inactive; fall back to the floor.
			name:      "zero gas unit falls back to floor",
			price:     ugnotPrice(0, 4),
			gasWanted: gasWanted,
			want:      floor,
		},
		{
			name:      "zero price amount falls back to floor",
			price:     ugnotPrice(1000, 0),
			gasWanted: gasWanted,
			want:      floor,
		},
		{
			// Margined minimum below the floor → floor wins.
			name:      "margined min below floor clamps to floor",
			price:     ugnotPrice(1000, 1),
			gasWanted: 1,
			want:      floor,
		},
		{
			// An implausibly high price would overflow gasWanted*amount and wrap to
			// a garbage positive fee; the guard must fall back to the floor instead.
			name:      "overflowing price falls back to floor",
			price:     ugnotPrice(1, 1_000_000_000_000),
			gasWanted: gasWanted,
			want:      floor,
		},
		{
			name:      "non-ugnot denom errors",
			price:     std.GasPrice{Gas: 1000, Price: std.Coin{Denom: "foo", Amount: 5}},
			gasWanted: gasWanted,
			wantErr:   true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := computeGasFee(tc.price, tc.gasWanted, floor)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestDecodeGasPrice_aminoJSON_roundTrip pins that auth/gasprice is amino-JSON
// encoded (the chain marshals via amino.MarshalJSONIndent): the gas unit is a
// string ("1000") and the price coin a string ("4ugnot"), neither of which
// encoding/json would parse into std.GasPrice.
func TestDecodeGasPrice_aminoJSON_roundTrip(t *testing.T) {
	original := ugnotPrice(1000, 4)

	data, err := amino.MarshalJSONIndent(original, "", "  ")
	require.NoError(t, err, "amino.MarshalJSONIndent")

	got, err := decodeGasPrice(data)
	require.NoError(t, err, "decodeGasPrice:\npayload:\n%s", data)

	assert.Equal(t, original.Gas, got.Gas)
	assert.Equal(t, original.Price.Denom, got.Price.Denom)
	assert.Equal(t, original.Price.Amount, got.Price.Amount)
}
