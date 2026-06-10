package read

import (
	"context"
	"testing"

	"github.com/gnolang/gno/tm2/pkg/std"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/chain"
)

func TestAccount_existingAccount(t *testing.T) {
	f := chain.NewFake()
	f.SetAccount("g1seeded", chain.AccountInfo{
		Exists:        true,
		Coins:         std.Coins{{Denom: "ugnot", Amount: 250000}},
		Sequence:      4,
		AccountNumber: 57,
	})

	s := newBaseTestServer(t)
	RegisterAccount(s, constResolver(f))
	res, err := s.Registry().Call(context.Background(), "gno_account", map[string]any{
		"address": "g1seeded",
		"profile": "testnet5",
	})
	require.NoError(t, err)
	assert.Contains(t, res.Text, "250000ugnot")
	assert.Contains(t, res.Text, `<untrusted_content kind="account"`, "chain-sourced text must be wrapped")
	require.NotNil(t, res.StructuredContent)
	assert.Equal(t, true, res.StructuredContent["exists"])
	assert.Equal(t, "250000ugnot", res.StructuredContent["coins"])
	assert.Equal(t, uint64(4), res.StructuredContent["sequence"])
	assert.Equal(t, uint64(57), res.StructuredContent["account_number"])
}

func TestAccount_unknownAddressReportsNotExists(t *testing.T) {
	f := chain.NewFake()

	s := newBaseTestServer(t)
	RegisterAccount(s, constResolver(f))
	res, err := s.Registry().Call(context.Background(), "gno_account", map[string]any{
		"address": "g1neverfunded",
		"profile": "testnet5",
	})
	require.NoError(t, err, "unknown address is a normal answer, not an error")
	assert.Contains(t, res.Text, "no on-chain record")
	assert.Equal(t, false, res.StructuredContent["exists"])
}

func TestAccount_zeroBalanceRendersAsZero(t *testing.T) {
	f := chain.NewFake()
	f.SetAccount("g1broke", chain.AccountInfo{Exists: true, Sequence: 1})

	s := newBaseTestServer(t)
	RegisterAccount(s, constResolver(f))
	res, err := s.Registry().Call(context.Background(), "gno_account", map[string]any{
		"address": "g1broke",
		"profile": "testnet5",
	})
	require.NoError(t, err)
	assert.Contains(t, res.Text, "balance 0")
	assert.Equal(t, "0", res.StructuredContent["coins"])
}

func TestAccount_requiresAddress(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterAccount(s, constResolver(chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_account", map[string]any{
		"profile": "testnet5",
	})
	require.Error(t, err)
}

func TestAccount_chainErrorSurfaces(t *testing.T) {
	f := chain.NewFake()
	f.SetAccountError("g1flaky", assert.AnError)

	s := newBaseTestServer(t)
	RegisterAccount(s, constResolver(f))
	_, err := s.Registry().Call(context.Background(), "gno_account", map[string]any{
		"address": "g1flaky",
		"profile": "testnet5",
	})
	require.Error(t, err)
}
