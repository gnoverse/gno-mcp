package clientfaucet

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/chain"
)

func TestLinkFaucet_FundAndFunded(t *testing.T) {
	fake := chain.NewFake()
	lf := &LinkFaucet{faucetURL: "https://faucet.test5.gno.land", chain: fake}

	out, err := lf.Fund(context.Background(), "g1abc", "test5")
	require.NoError(t, err)
	assert.Equal(t, "link", out.Backend)
	assert.Equal(t, "g1abc", out.Address)
	assert.Contains(t, out.Instructions, "https://faucet.test5.gno.land")
	assert.Contains(t, out.Instructions, "g1abc")

	funded, err := lf.Funded(context.Background(), "g1abc")
	require.NoError(t, err)
	assert.False(t, funded, "0 balance -> not funded")

	fake.SetBalance("g1abc", 1_000_000)
	funded, _ = lf.Funded(context.Background(), "g1abc")
	assert.True(t, funded, "positive balance -> funded")
}

func TestLinkFaucet_manualFallback_noURL(t *testing.T) {
	lf := &LinkFaucet{faucetURL: "", chain: chain.NewFake()}
	out, err := lf.Fund(context.Background(), "g1abc", "test5")
	require.NoError(t, err)
	assert.Equal(t, "manual", out.Backend)
	assert.NotContains(t, out.Instructions, "http")
	assert.Contains(t, out.Instructions, "g1abc")
}
