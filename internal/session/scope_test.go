package session

import (
	"strings"
	"testing"
	"time"

	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testProfile(chainType string) *profiles.Profile {
	return &profiles.Profile{ChainType: chainType}
}

// ---- Layer 1: agent-explicit values win

func TestResolveScope_agentExplicitWins(t *testing.T) {
	args := ScopeArgs{
		SpendLimit: "500ugnot",
		ExpiresIn:  "30m",
		AllowPaths: []string{"gno.land/r/myorg/blog"},
	}
	p := testProfile(profiles.ChainTypeTestnet)
	scope, warns, err := ResolveScope(args, p)
	require.NoError(t, err)
	assert.Empty(t, warns)
	assert.Equal(t, "500ugnot", scope.SpendLimit)
	assert.Equal(t, 30*time.Minute, scope.ExpiresIn)
	require.Len(t, scope.AllowPaths, 1)
	assert.Equal(t, "gno.land/r/myorg/blog", scope.AllowPaths[0])
}

// ---- Layer 2: profile defaults apply when agent omits values

func TestResolveScope_profileDefaultsApply(t *testing.T) {
	p := &profiles.Profile{
		ChainType:         profiles.ChainTypeTestnet,
		DefaultSpendLimit: "200000ugnot",
		DefaultExpiresIn:  "2h",
	}
	args := ScopeArgs{AllowPaths: []string{"gno.land/r/myorg/blog"}}
	scope, _, err := ResolveScope(args, p)
	require.NoError(t, err)
	assert.Equal(t, "200000ugnot", scope.SpendLimit)
	assert.Equal(t, 2*time.Hour, scope.ExpiresIn)
}

// ---- Layer 3: hardcoded fallback when profile and agent both omit values

func TestResolveScope_hardcodedFallback(t *testing.T) {
	p := testProfile(profiles.ChainTypeTestnet)
	args := ScopeArgs{AllowPaths: []string{"gno.land/r/myorg/blog"}}
	scope, _, err := ResolveScope(args, p)
	require.NoError(t, err)
	assert.Equal(t, "100000000ugnot", scope.SpendLimit)
	assert.Equal(t, time.Hour, scope.ExpiresIn)
}

// ---- Layer 4: hard-limit clamps

func TestResolveScope_clampsSpendLimit(t *testing.T) {
	// testnet cap is 100000000ugnot; agent asks for 500000000ugnot
	p := testProfile(profiles.ChainTypeTestnet)
	args := ScopeArgs{
		SpendLimit: "500000000ugnot",
		AllowPaths: []string{"gno.land/r/myorg/blog"},
	}
	scope, warns, err := ResolveScope(args, p)
	require.NoError(t, err)
	assert.Equal(t, "100000000ugnot", scope.SpendLimit, "SpendLimit should be clamped")
	require.NotEmpty(t, warns, "expected a warning for clamped spend_limit")
	assert.Contains(t, warns[0], "spend_limit")
}

func TestResolveScope_clampsExpiresIn(t *testing.T) {
	// testnet cap is 7*24h; agent asks for 30*24h
	p := testProfile(profiles.ChainTypeTestnet)
	args := ScopeArgs{
		SpendLimit: "100ugnot",
		ExpiresIn:  "720h",
		AllowPaths: []string{"gno.land/r/myorg/blog"},
	}
	scope, warns, err := ResolveScope(args, p)
	require.NoError(t, err)
	assert.Equal(t, 7*24*time.Hour, scope.ExpiresIn, "ExpiresIn should be clamped")
	assert.Equal(t, 7*24*time.Hour, scope.SpendPeriod, "SpendPeriod should be clamped")
	require.NotEmpty(t, warns, "expected a warning for clamped expires_in")
	assert.Contains(t, warns[0], "expires_in")
}

func TestResolveScope_clampsAllowPathsCount(t *testing.T) {
	// testnet cap is 10; agent supplies 15
	p := testProfile(profiles.ChainTypeTestnet)
	var paths []string
	for i := 0; i < 15; i++ {
		paths = append(paths, "gno.land/r/myorg/p"+string(rune('0'+i)))
	}
	args := ScopeArgs{
		SpendLimit: "100ugnot",
		AllowPaths: paths,
	}
	scope, warns, err := ResolveScope(args, p)
	require.NoError(t, err)
	assert.Len(t, scope.AllowPaths, 10, "AllowPaths count should be clamped to 10")
	require.NotEmpty(t, warns, "expected a warning for clamped allow_paths")
	assert.Contains(t, warns[0], "allow_paths")
}

// ---- BypassHardLimits skips layer 4

func TestResolveScope_bypassSkipsClamps(t *testing.T) {
	p := &profiles.Profile{
		ChainType:        profiles.ChainTypeTestnet,
		BypassHardLimits: true,
	}
	args := ScopeArgs{
		SpendLimit: "999999ugnot",
		ExpiresIn:  "720h",
		AllowPaths: []string{"gno.land/r/a", "gno.land/r/b", "gno.land/r/c", "gno.land/r/d"},
	}
	scope, warns, err := ResolveScope(args, p)
	require.NoError(t, err)
	assert.Empty(t, warns, "expected no warnings with bypass")
	assert.Equal(t, "999999ugnot", scope.SpendLimit)
	assert.Equal(t, 720*time.Hour, scope.ExpiresIn)
	assert.Len(t, scope.AllowPaths, 4)
}

// ---- Error: empty allow_paths and allow_run=false

func TestResolveScope_emptyAllowPathsAndNoRunError(t *testing.T) {
	p := testProfile(profiles.ChainTypeTestnet)
	args := ScopeArgs{}
	_, _, err := ResolveScope(args, p)
	require.Error(t, err, "expected error for empty allow_paths + allow_run=false")
	assert.True(t, strings.Contains(err.Error(), "allow_paths") && strings.Contains(err.Error(), "allow_run"),
		"error should mention allow_paths and allow_run: %v", err)
}

// ---- AllowRun-only is accepted (empty allow_paths is fine)

func TestResolveScope_allowRunOnlyOK(t *testing.T) {
	p := testProfile(profiles.ChainTypeTestnet)
	args := ScopeArgs{AllowRun: true}
	scope, _, err := ResolveScope(args, p)
	require.NoError(t, err)
	assert.True(t, scope.AllowRun, "scope.AllowRun should be true")
	assert.Empty(t, scope.AllowPaths)
}

// ---- Both allow_paths and allow_run are accepted

func TestResolveScope_allowPathsPlusAllowRun(t *testing.T) {
	p := testProfile(profiles.ChainTypeTestnet)
	args := ScopeArgs{
		AllowPaths: []string{"gno.land/r/myorg/blog"},
		AllowRun:   true,
	}
	scope, _, err := ResolveScope(args, p)
	require.NoError(t, err)
	assert.True(t, scope.AllowRun, "scope.AllowRun should be true")
	assert.Len(t, scope.AllowPaths, 1)
}

// ---- AllowPaths-only with allow_run=false still works

func TestResolveScope_allowPathsOnlyWorks(t *testing.T) {
	p := testProfile(profiles.ChainTypeTestnet)
	args := ScopeArgs{
		AllowPaths: []string{"gno.land/r/myorg/blog"},
		AllowRun:   false,
	}
	scope, _, err := ResolveScope(args, p)
	require.NoError(t, err)
	assert.False(t, scope.AllowRun, "scope.AllowRun should be false")
	assert.Len(t, scope.AllowPaths, 1)
}

// ---- Unknown chain-type falls back to testnet limits

func TestResolveScope_unknownChainTypeFallback(t *testing.T) {
	p := testProfile("foobar")
	// testnet cap: MaxAllowPathsCount=10; supply 15
	var paths []string
	for i := 0; i < 15; i++ {
		paths = append(paths, "gno.land/r/myorg/p"+string(rune('0'+i)))
	}
	args := ScopeArgs{AllowPaths: paths}
	scope, warns, err := ResolveScope(args, p)
	require.NoError(t, err)
	assert.Len(t, scope.AllowPaths, 10, "AllowPaths count should use testnet fallback cap of 10")
	assert.NotEmpty(t, warns, "expected a warning for clamped allow_paths")
}
