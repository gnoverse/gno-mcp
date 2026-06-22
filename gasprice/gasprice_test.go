package gasprice

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

func TestCompute(t *testing.T) {
	const floor = 10_000
	const gasWanted = 10_000_000
	const marginNum, marginDen = 2, 1

	cases := []struct {
		name      string
		price     std.GasPrice
		gasWanted int64
		want      int64
		wantErr   bool
	}{
		{
			// A congested chain at 4ugnot/1000gas (4× the genesis floor): min = 10M*4/1000 = 40_000, ×2 = 80_000.
			name:      "congested chain above floor",
			price:     ugnotPrice(1000, 4),
			gasWanted: gasWanted,
			want:      80_000,
		},
		{
			// Genesis floor 1ugnot/1000gas: min = 10_000, ×2 = 20_000 (above floor).
			name:      "chain at genesis floor",
			price:     ugnotPrice(1000, 1),
			gasWanted: gasWanted,
			want:      20_000,
		},
		{
			// Non-exact division must round UP: min = ceil(10M/3) = 3_333_334, ×2.
			name:      "ceil rounding",
			price:     ugnotPrice(3, 1),
			gasWanted: gasWanted,
			want:      6_666_668,
		},
		{
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
			name:      "margined min below floor clamps to floor",
			price:     ugnotPrice(1000, 1),
			gasWanted: 1,
			want:      floor,
		},
		{
			// Implausible price would overflow gasWanted*amount; guard → floor.
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
			got, err := Compute(tc.price, tc.gasWanted, floor, marginNum, marginDen)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestCompute_invalidParams rejects caller-programming errors (an operator's
// -gas-wanted 0, or a bad margin) with an error instead of a divide-by-zero panic.
func TestCompute_invalidParams(t *testing.T) {
	price := ugnotPrice(1000, 4)
	cases := []struct {
		name                 string
		gasWanted, floor     int64
		marginNum, marginDen int64
	}{
		{"zero gasWanted", 0, 10_000, 2, 1},
		{"negative gasWanted", -1, 10_000, 2, 1},
		{"zero marginNum", 10_000_000, 10_000, 0, 1},
		{"zero marginDen", 10_000_000, 10_000, 2, 0},
		{"negative marginDen", 10_000_000, 10_000, 2, -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Compute(price, tc.gasWanted, tc.floor, tc.marginNum, tc.marginDen)
			require.Error(t, err)
		})
	}
}

// TestDecode_aminoJSON_roundTrip pins that auth/gasprice is amino-JSON encoded
// (the chain marshals via amino.MarshalJSONIndent): the gas unit is a string
// ("1000") and the price coin a string ("4ugnot"), neither of which
// encoding/json would parse into std.GasPrice.
func TestDecode_aminoJSON_roundTrip(t *testing.T) {
	original := ugnotPrice(1000, 4)

	data, err := amino.MarshalJSONIndent(original, "", "  ")
	require.NoError(t, err, "amino.MarshalJSONIndent")

	got, err := decode(data)
	require.NoError(t, err, "decode:\npayload:\n%s", data)

	assert.Equal(t, original.Gas, got.Gas)
	assert.Equal(t, original.Price.Denom, got.Price.Denom)
	assert.Equal(t, original.Price.Amount, got.Price.Amount)
}
