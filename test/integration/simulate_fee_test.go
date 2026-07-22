//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/chain"
)

// TestIntegration_simulateOffersLiveFee pins the simulate/broadcast fee
// symmetry: a simulation must offer the same live-priced GasFee a broadcast
// would, not the floor. A floor-priced simulate is a liar on any chain priced
// above genesis — it sails through the ante's session spend pre-check that
// then kills the real broadcast ("session not allowed").
func TestIntegration_simulateOffersLiveFee(t *testing.T) {
	c := newNodeBackedReal(t)
	ctx := context.Background()

	liveFee, err := c.GasFeeUgnot(ctx)
	require.NoError(t, err, "query live fee")
	require.NotEqual(t, chain.DefaultGasFeeUgnot, liveFee,
		"live fee must differ from the floor for this test to bite")

	res, err := c.Call(ctx, test1Signer(t), "gno.land/r/test/counter", "Increment", nil, "", true)
	require.NoError(t, err, "simulate increment")
	require.True(t, res.Simulated)
	require.Equal(t, liveFee, res.GasFeeUgnot,
		"a light tx floors to DefaultGasWanted, so simulate reports the floor-sized live fee")
	require.Equal(t, chain.DefaultGasWanted, res.GasWanted,
		"simulate reports the right-sized gas-wanted the broadcast would reserve")
}
