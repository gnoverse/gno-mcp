package session

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testProfile(chainID string) *profiles.Profile {
	return &profiles.Profile{ChainID: chainID}
}

// testFee is a per-write GasFee small enough that the existing scope fixtures
// stay above the spend-limit-must-cover-a-write guard.
const testFee int64 = 100

// ---- Layer 1: agent-explicit values win

func TestResolveScope_agentExplicitWins(t *testing.T) {
	args := ScopeArgs{
		SpendLimit: "500ugnot",
		ExpiresIn:  "30m",
		AllowPaths: []string{"gno.land/r/myorg/blog"},
	}
	p := testProfile("test5")
	scope, warns, err := ResolveScope(args, p, testFee)
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
		DefaultSpendLimit: "200000ugnot",
		DefaultExpiresIn:  "2h",
	}
	args := ScopeArgs{AllowPaths: []string{"gno.land/r/myorg/blog"}}
	scope, _, err := ResolveScope(args, p, testFee)
	require.NoError(t, err)
	assert.Equal(t, "200000ugnot", scope.SpendLimit)
	assert.Equal(t, 2*time.Hour, scope.ExpiresIn)
}

// ---- Layer 3: fee-derived fallback when profile and agent both omit values

func TestResolveScope_feeDerivedDefault(t *testing.T) {
	// No agent value, no profile default: the spend limit derives from the
	// live per-write fee (10 writes' worth), so a default session is never
	// dead on arrival on a chain with a high gas price.
	p := testProfile("test5")
	args := ScopeArgs{AllowPaths: []string{"gno.land/r/myorg/blog"}}
	scope, warns, err := ResolveScope(args, p, 4_000_000)
	require.NoError(t, err)
	assert.Empty(t, warns)
	assert.Equal(t, "40000000ugnot", scope.SpendLimit)
	assert.Equal(t, time.Hour, scope.ExpiresIn)
}

func TestResolveScope_feeDerivedDefaultCappedByHardLimit(t *testing.T) {
	// 10×fee exceeds the testnet cap of 100000000ugnot: the derived default
	// caps silently — no "requested ... exceeds cap" warning for a value the
	// agent never requested.
	p := testProfile("test5")
	args := ScopeArgs{AllowPaths: []string{"gno.land/r/myorg/blog"}}
	scope, warns, err := ResolveScope(args, p, 15_000_000)
	require.NoError(t, err)
	assert.Empty(t, warns)
	assert.Equal(t, "100000000ugnot", scope.SpendLimit)
}

// ---- Guard: the effective spend limit must cover one write's GasFee

func TestResolveScope_explicitSpendLimitBelowFeeErrors(t *testing.T) {
	p := testProfile("test5")
	args := ScopeArgs{
		SpendLimit: "1000000ugnot",
		AllowPaths: []string{"gno.land/r/myorg/blog"},
	}
	_, _, err := ResolveScope(args, p, 4_000_000)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "1000000ugnot", "error should name the offending limit")
	assert.Contains(t, err.Error(), "4000000", "error should name the per-write fee")
}

func TestResolveScope_profileDefaultBelowFeeErrors(t *testing.T) {
	p := &profiles.Profile{DefaultSpendLimit: "200000ugnot"}
	args := ScopeArgs{AllowPaths: []string{"gno.land/r/myorg/blog"}}
	_, _, err := ResolveScope(args, p, 4_000_000)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "200000ugnot")
	assert.Contains(t, err.Error(), "4000000")
}

func TestResolveScope_bypassStillGuardsFee(t *testing.T) {
	// BypassHardLimits skips policy clamps, not the DOA guard: a session that
	// cannot pay a single write's fee is useless on any profile.
	p := &profiles.Profile{BypassHardLimits: true}
	args := ScopeArgs{
		SpendLimit: "999ugnot",
		AllowPaths: []string{"gno.land/r/myorg/blog"},
	}
	_, _, err := ResolveScope(args, p, 4_000_000)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "999ugnot")
}

func TestResolveScope_nonUgnotSpendLimitErrors(t *testing.T) {
	// Gas fees are billed in ugnot; a spend limit in another denom can never
	// cover them, so the session would be rejected by the chain's ante on its
	// first write. Only the BypassHardLimits path can reach the guard with a
	// non-ugnot denom (clamping errors on it first otherwise).
	p := &profiles.Profile{BypassHardLimits: true}
	args := ScopeArgs{
		SpendLimit: "5000000foo",
		AllowPaths: []string{"gno.land/r/myorg/blog"},
	}
	_, _, err := ResolveScope(args, p, 100)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ugnot")
}

// ---- WritesAtFee: the "how many writes does this limit buy" math

func TestScope_WritesAtFee(t *testing.T) {
	cases := map[string]struct {
		limit  string
		fee    int64
		want   int64
		wantOK bool
	}{
		"exact multiple":  {limit: "40000000ugnot", fee: 4_000_000, want: 10, wantOK: true},
		"rounds down":     {limit: "9000000ugnot", fee: 4_000_000, want: 2, wantOK: true},
		"below fee":       {limit: "1000000ugnot", fee: 4_000_000, want: 0, wantOK: true},
		"zero fee":        {limit: "1000000ugnot", fee: 0, wantOK: false},
		"non-ugnot denom": {limit: "1000000foo", fee: 100, wantOK: false},
		"malformed limit": {limit: "junk", fee: 100, wantOK: false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, ok := Scope{SpendLimit: tc.limit}.WritesAtFee(tc.fee)
			require.Equal(t, tc.wantOK, ok)
			if tc.wantOK {
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

// ---- Layer 4: hard-limit clamps

func TestResolveScope_clampsSpendLimit(t *testing.T) {
	// testnet cap is 100000000ugnot; agent asks for 500000000ugnot
	p := testProfile("test5")
	args := ScopeArgs{
		SpendLimit: "500000000ugnot",
		AllowPaths: []string{"gno.land/r/myorg/blog"},
	}
	scope, warns, err := ResolveScope(args, p, testFee)
	require.NoError(t, err)
	assert.Equal(t, "100000000ugnot", scope.SpendLimit, "SpendLimit should be clamped")
	require.NotEmpty(t, warns, "expected a warning for clamped spend_limit")
	assert.Contains(t, warns[0], "spend_limit")
}

func TestResolveScope_clampsExpiresIn(t *testing.T) {
	// testnet cap is 7*24h; agent asks for 30*24h
	p := testProfile("test5")
	args := ScopeArgs{
		SpendLimit: "100ugnot",
		ExpiresIn:  "720h",
		AllowPaths: []string{"gno.land/r/myorg/blog"},
	}
	scope, warns, err := ResolveScope(args, p, testFee)
	require.NoError(t, err)
	assert.Equal(t, 7*24*time.Hour, scope.ExpiresIn, "ExpiresIn should be clamped")
	assert.Equal(t, 7*24*time.Hour, scope.SpendPeriod, "SpendPeriod should be clamped")
	require.NotEmpty(t, warns, "expected a warning for clamped expires_in")
	assert.Contains(t, warns[0], "expires_in")
}

func TestResolveScope_clampsAllowPathsCount(t *testing.T) {
	// testnet cap is 10; agent supplies 15
	p := testProfile("test5")
	var paths []string
	for i := 0; i < 15; i++ {
		paths = append(paths, fmt.Sprintf("gno.land/r/myorg/p%d", i))
	}
	args := ScopeArgs{
		SpendLimit: "100ugnot",
		AllowPaths: paths,
	}
	scope, warns, err := ResolveScope(args, p, testFee)
	require.NoError(t, err)
	assert.Len(t, scope.AllowPaths, 10, "AllowPaths count should be clamped to 10")
	require.NotEmpty(t, warns, "expected a warning for clamped allow_paths")
	assert.Contains(t, warns[0], "allow_paths")
}

// ---- BypassHardLimits skips layer 4

func TestResolveScope_bypassSkipsClamps(t *testing.T) {
	p := &profiles.Profile{
		BypassHardLimits: true,
	}
	args := ScopeArgs{
		SpendLimit: "999999ugnot",
		ExpiresIn:  "720h",
		AllowPaths: []string{"gno.land/r/a", "gno.land/r/b", "gno.land/r/c", "gno.land/r/d"},
	}
	scope, warns, err := ResolveScope(args, p, testFee)
	require.NoError(t, err)
	assert.Empty(t, warns, "expected no warnings with bypass")
	assert.Equal(t, "999999ugnot", scope.SpendLimit)
	assert.Equal(t, 720*time.Hour, scope.ExpiresIn)
	assert.Len(t, scope.AllowPaths, 4)
}

// ---- Error: empty allow_paths and allow_run=false

func TestResolveScope_emptyAllowPathsAndNoRunError(t *testing.T) {
	p := testProfile("test5")
	args := ScopeArgs{}
	_, _, err := ResolveScope(args, p, testFee)
	require.Error(t, err, "expected error for empty allow_paths + allow_run=false")
	assert.True(t, strings.Contains(err.Error(), "allow_paths") && strings.Contains(err.Error(), "allow_run"),
		"error should mention allow_paths and allow_run: %v", err)
}

// ---- AllowRun-only is accepted (empty allow_paths is fine)

func TestResolveScope_allowRunOnlyOK(t *testing.T) {
	p := testProfile("test5")
	args := ScopeArgs{AllowRun: true}
	scope, _, err := ResolveScope(args, p, testFee)
	require.NoError(t, err)
	assert.True(t, scope.AllowRun, "scope.AllowRun should be true")
	assert.Empty(t, scope.AllowPaths)
}

// ---- Both allow_paths and allow_run are accepted

func TestResolveScope_allowPathsPlusAllowRun(t *testing.T) {
	p := testProfile("test5")
	args := ScopeArgs{
		AllowPaths: []string{"gno.land/r/myorg/blog"},
		AllowRun:   true,
	}
	scope, _, err := ResolveScope(args, p, testFee)
	require.NoError(t, err)
	assert.True(t, scope.AllowRun, "scope.AllowRun should be true")
	assert.Len(t, scope.AllowPaths, 1)
}

// ---- AllowPaths-only with allow_run=false still works

func TestResolveScope_allowPathsOnlyWorks(t *testing.T) {
	p := testProfile("test5")
	args := ScopeArgs{
		AllowPaths: []string{"gno.land/r/myorg/blog"},
		AllowRun:   false,
	}
	scope, _, err := ResolveScope(args, p, testFee)
	require.NoError(t, err)
	assert.False(t, scope.AllowRun, "scope.AllowRun should be false")
	assert.Len(t, scope.AllowPaths, 1)
}

// ---- Injection: allow_paths and spend_limit feed a pasted gnokey command

func TestResolveScope_rejectsInjectionInAllowPaths(t *testing.T) {
	p := testProfile("test5")
	for _, bad := range []string{
		"gno.land/r/foo\n  --allow-paths vm/run",
		"gno.land/r/foo; rm -rf /",
		"gno.land/r/foo$(whoami)",
		"gno.land/r/foo `id`",
		"gno.land/r/foo && echo pwned",
		"gno.land/r/foo ",
	} {
		_, _, err := ResolveScope(ScopeArgs{AllowPaths: []string{bad}}, p, testFee)
		require.Error(t, err, "expected rejection of allow_paths %q", bad)
	}
}

func TestResolveScope_rejectsNonRealmAllowPath(t *testing.T) {
	p := testProfile("test5")
	for _, bad := range []string{
		"gno.land/p/demo/avl", // pure package, not a realm
		"not-a-path",
		"gno.land/r/Foo", // uppercase is not a valid realm name
	} {
		_, _, err := ResolveScope(ScopeArgs{AllowPaths: []string{bad}}, p, testFee)
		require.Error(t, err, "expected rejection of non-realm allow_paths %q", bad)
	}
}

func TestResolveScope_rejectsMalformedSpendLimit(t *testing.T) {
	// Covers both the hard-limit path and the BypassHardLimits early return,
	// where clampCoin (the only prior parse) never runs.
	profs := []*profiles.Profile{
		testProfile("test5"),
		{BypassHardLimits: true},
	}
	for _, prof := range profs {
		_, _, err := ResolveScope(ScopeArgs{
			SpendLimit: "100ugnot; rm -rf /",
			AllowPaths: []string{"gno.land/r/myorg/blog"},
		}, prof, testFee)
		require.Error(t, err, "expected rejection of malformed spend_limit (bypass=%v)", prof.BypassHardLimits)
	}
}

// ---- Any non-dev chain-id derives testnet limits (the safe middle)

func TestResolveScope_nonDevChainIDGetsTestnetLimits(t *testing.T) {
	p := testProfile("foobar")
	// testnet cap: MaxAllowPathsCount=10; supply 15
	var paths []string
	for i := 0; i < 15; i++ {
		paths = append(paths, fmt.Sprintf("gno.land/r/myorg/p%d", i))
	}
	args := ScopeArgs{AllowPaths: paths}
	scope, warns, err := ResolveScope(args, p, testFee)
	require.NoError(t, err)
	assert.Len(t, scope.AllowPaths, 10, "AllowPaths count should use testnet fallback cap of 10")
	assert.NotEmpty(t, warns, "expected a warning for clamped allow_paths")
}
