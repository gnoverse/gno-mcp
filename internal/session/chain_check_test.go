package session

import (
	"context"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryChain_active(t *testing.T) {
	fake := chain.NewFake()
	fake.SetSession("g1master", "g1session", chain.SessionStatus{
		Active:         true,
		AllowPaths:     []string{"gno.land/r/test/blog"},
		SpendLimit:     "1000000ugnot",
		SpendRemaining: "900000ugnot",
		ExpiresAt:      9999999999,
	})
	var resolver chain.Resolver = func(profile string) chain.Client { return fake }

	res, err := queryChain(context.Background(), resolver, "testnet", "g1master", "g1session")
	require.NoError(t, err)
	require.True(t, res.Active, "expected Active=true")
	assert.Equal(t, "900000ugnot", res.Status.SpendRemaining)
}

func TestQueryChain_inactive(t *testing.T) {
	fake := chain.NewFake()
	fake.SetSession("g1master", "g1expired", chain.SessionStatus{
		Active: false,
	})
	var resolver chain.Resolver = func(profile string) chain.Client { return fake }

	res, err := queryChain(context.Background(), resolver, "testnet", "g1master", "g1expired")
	require.NoError(t, err)
	require.False(t, res.Active, "expected Active=false")
}

func TestQueryChain_emptyMasterIsUnsupported(t *testing.T) {
	fake := chain.NewFake()
	var resolver chain.Resolver = func(profile string) chain.Client { return fake }

	res, err := queryChain(context.Background(), resolver, "testnet", "", "g1session")
	require.NoError(t, err)
	assert.True(t, res.Unsupported, "expected Unsupported=true when master is empty")
	assert.False(t, res.Active, "expected Active=false when master is empty")
}

func TestQueryChain_unknownProfileError(t *testing.T) {
	var resolver chain.Resolver = func(profile string) chain.Client { return nil }

	_, err := queryChain(context.Background(), resolver, "missing-profile", "g1master", "g1session")
	require.Error(t, err)
}
