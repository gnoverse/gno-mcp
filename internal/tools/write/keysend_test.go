package write

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/audit"
	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/keystore"
)

func TestKeySend_movesFundsBetweenOwnKeys(t *testing.T) {
	s := newTestnetTestServer(t)
	ks := keystore.New(t.TempDir(), "", 5)
	from, err := ks.GenerateForProfile("testnet9999", "", testnet9999Profile())
	require.NoError(t, err)
	to, err := ks.GenerateForProfile("testnet9999", "bob", testnet9999Profile())
	require.NoError(t, err)

	fake := chain.NewFake()
	fake.SetBalance(from, 10_000_000) // source is funded (passes the pre-check)

	RegisterKeySend(s, ks, constChainResolver(fake), audit.NewLog(&bytes.Buffer{}))

	res, callErr := s.Registry().Call(context.Background(), "gno_key_send", map[string]any{
		"profile": "testnet9999",
		"to":      "bob",
		"amount":  float64(4_000_000),
	})
	require.NoError(t, callErr)
	assert.Contains(t, res.Text, "0xsend")
	gk, _ := res.StructuredContent["gnokey_command"].(string)
	assert.Contains(t, gk, "gnokey maketx send", "keysend must wire its own subcommand")
	assert.Contains(t, gk, "-to "+to)
	assert.Contains(t, gk, "-send 4000000ugnot")

	sends := fake.BankSends()
	require.Len(t, sends, 1)
	assert.Equal(t, to, sends[0].To, "must send to the resolved destination key address")
	assert.Equal(t, int64(4_000_000), sends[0].Amount)
}

func TestKeySend_unknownDestinationKey(t *testing.T) {
	s := newTestnetTestServer(t)
	ks := keystore.New(t.TempDir(), "", 5)
	from, err := ks.GenerateForProfile("testnet9999", "", testnet9999Profile())
	require.NoError(t, err)
	fake := chain.NewFake()
	fake.SetBalance(from, 10_000_000)

	RegisterKeySend(s, ks, constChainResolver(fake), audit.NewLog(&bytes.Buffer{}))

	_, callErr := s.Registry().Call(context.Background(), "gno_key_send", map[string]any{
		"profile": "testnet9999",
		"to":      "nobody",
		"amount":  float64(1_000_000),
	})
	require.Error(t, callErr, "sending to a key that does not exist must fail")
	assert.Equal(t, 0, len(fake.BankSends()), "no broadcast for an unknown destination")
}

func TestKeySend_rejectsNonPositiveAmount(t *testing.T) {
	s := newTestnetTestServer(t)
	ks := keystore.New(t.TempDir(), "", 5)
	_, err := ks.GenerateForProfile("testnet9999", "", testnet9999Profile())
	require.NoError(t, err)
	_, err = ks.GenerateForProfile("testnet9999", "bob", testnet9999Profile())
	require.NoError(t, err)
	fake := chain.NewFake()

	RegisterKeySend(s, ks, constChainResolver(fake), audit.NewLog(&bytes.Buffer{}))

	_, callErr := s.Registry().Call(context.Background(), "gno_key_send", map[string]any{
		"profile": "testnet9999",
		"to":      "bob",
		"amount":  float64(0),
	})
	require.Error(t, callErr, "zero amount must be rejected")
	assert.Equal(t, 0, len(fake.BankSends()))
}
