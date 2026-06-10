//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/gnolang/gno/gno.land/pkg/integration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/keystore"
)

// test1's address — funded in the genesis of the in-process node.
const test1Addr = "g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5"

func TestIntegration_AccountExisting(t *testing.T) {
	c := newNodeBackedReal(t)

	info, err := c.Account(context.Background(), test1Addr)
	require.NoError(t, err, "Account")
	assert.True(t, info.Exists, "test1 is funded in genesis")
	assert.Positive(t, info.Coins.AmountOf("ugnot"), "test1 must hold ugnot")
}

func TestIntegration_AccountUnknownAddress(t *testing.T) {
	c := newNodeBackedReal(t)

	// A valid address derived from test1's mnemonic at index 1 — never funded
	// and never used, so the chain has no record of it.
	key, err := integration.GeneratePrivKeyFromMnemonic(keystore.Test1Mnemonic, "", 0, 1)
	require.NoError(t, err)
	unknown := key.PubKey().Address().String()

	info, err := c.Account(context.Background(), unknown)
	require.NoError(t, err, "an unknown address is a normal answer, not an error")
	assert.False(t, info.Exists)
}

func TestIntegration_Status(t *testing.T) {
	c := newNodeBackedReal(t)

	st, err := c.Status(context.Background())
	require.NoError(t, err, "Status")
	assert.Equal(t, nodeChainID, st.ChainID)
	assert.Positive(t, st.Height)
	assert.False(t, st.BlockTime.IsZero(), "node must report a block time")
}
