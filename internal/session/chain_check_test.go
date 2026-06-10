package session

import (
	"context"
	"errors"
	"fmt"
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

func TestQueryChain_queryFlakePreservesState(t *testing.T) {
	fake := chain.NewFake()
	// A transient RPC or malformed-response flake surfaces (possibly wrapped) as
	// ErrSessionQueryUnsupported — exactly what Real.QuerySession returns on a
	// decode failure. The session must be preserved, not dropped.
	fake.SetSessionError("g1master", "g1sess", fmt.Errorf("decode failed: %w", chain.ErrSessionQueryUnsupported))
	var resolver chain.Resolver = func(string) chain.Client { return fake }

	res, err := queryChain(context.Background(), resolver, "testnet", "g1master", "g1sess")
	require.NoError(t, err)
	assert.True(t, res.Unsupported, "a query flake must preserve local state")
	assert.False(t, res.Active)
}

func TestQueryChain_hardErrorDropsSession(t *testing.T) {
	fake := chain.NewFake()
	fake.SetSessionError("g1master", "g1sess", errors.New("hard error, not a flake"))
	var resolver chain.Resolver = func(string) chain.Client { return fake }

	res, err := queryChain(context.Background(), resolver, "testnet", "g1master", "g1sess")
	require.NoError(t, err)
	assert.False(t, res.Unsupported, "a non-Unsupported error is treated as inactive")
	assert.False(t, res.Active)
}

func TestQueryChain_unknownProfileError(t *testing.T) {
	var resolver chain.Resolver = func(profile string) chain.Client { return nil }

	_, err := queryChain(context.Background(), resolver, "missing-profile", "g1master", "g1session")
	require.Error(t, err)
}
