package session

import (
	"strings"
	"testing"
	"time"

	"github.com/gnoverse/gno-mcp/internal/profiles"
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warns) != 0 {
		t.Errorf("unexpected warnings: %v", warns)
	}
	if scope.SpendLimit != "500ugnot" {
		t.Errorf("SpendLimit: got %q, want 500ugnot", scope.SpendLimit)
	}
	if scope.ExpiresIn != 30*time.Minute {
		t.Errorf("ExpiresIn: got %v, want 30m", scope.ExpiresIn)
	}
	if len(scope.AllowPaths) != 1 || scope.AllowPaths[0] != "gno.land/r/myorg/blog" {
		t.Errorf("AllowPaths: got %v", scope.AllowPaths)
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if scope.SpendLimit != "200000ugnot" {
		t.Errorf("SpendLimit: got %q, want 200000ugnot", scope.SpendLimit)
	}
	if scope.ExpiresIn != 2*time.Hour {
		t.Errorf("ExpiresIn: got %v, want 2h", scope.ExpiresIn)
	}
}

// ---- Layer 3: hardcoded fallback when profile and agent both omit values

func TestResolveScope_hardcodedFallback(t *testing.T) {
	p := testProfile(profiles.ChainTypeTestnet)
	args := ScopeArgs{AllowPaths: []string{"gno.land/r/myorg/blog"}}
	scope, _, err := ResolveScope(args, p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if scope.SpendLimit != "100000ugnot" {
		t.Errorf("SpendLimit: got %q, want hardcoded 100000ugnot", scope.SpendLimit)
	}
	if scope.ExpiresIn != time.Hour {
		t.Errorf("ExpiresIn: got %v, want hardcoded 1h", scope.ExpiresIn)
	}
}

// ---- Layer 4: hard-limit clamps

func TestResolveScope_clampsSpendLimit(t *testing.T) {
	// mainnet cap is 1000ugnot; agent asks for 5000ugnot
	p := testProfile(profiles.ChainTypeMainnet)
	args := ScopeArgs{
		SpendLimit: "5000ugnot",
		AllowPaths: []string{"gno.land/r/myorg/blog"},
	}
	scope, warns, err := ResolveScope(args, p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if scope.SpendLimit != "1000ugnot" {
		t.Errorf("SpendLimit: got %q, want 1000ugnot (clamped)", scope.SpendLimit)
	}
	if len(warns) == 0 {
		t.Error("expected a warning for clamped spend_limit")
	}
	if !strings.Contains(warns[0], "spend_limit") {
		t.Errorf("warning should mention spend_limit: %q", warns[0])
	}
}

func TestResolveScope_clampsExpiresIn(t *testing.T) {
	// mainnet cap is 1h; agent asks for 48h
	p := testProfile(profiles.ChainTypeMainnet)
	args := ScopeArgs{
		SpendLimit: "100ugnot",
		ExpiresIn:  "48h",
		AllowPaths: []string{"gno.land/r/myorg/blog"},
	}
	scope, warns, err := ResolveScope(args, p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if scope.ExpiresIn != time.Hour {
		t.Errorf("ExpiresIn: got %v, want 1h (clamped)", scope.ExpiresIn)
	}
	if scope.SpendPeriod != time.Hour {
		t.Errorf("SpendPeriod: got %v, want 1h (clamped)", scope.SpendPeriod)
	}
	if len(warns) == 0 {
		t.Error("expected a warning for clamped expires_in")
	}
	if !strings.Contains(warns[0], "expires_in") {
		t.Errorf("warning should mention expires_in: %q", warns[0])
	}
}

func TestResolveScope_clampsAllowPathsCount(t *testing.T) {
	// mainnet cap is 3; agent supplies 10
	p := testProfile(profiles.ChainTypeMainnet)
	var paths []string
	for i := 0; i < 10; i++ {
		paths = append(paths, "gno.land/r/myorg/p"+string(rune('0'+i)))
	}
	args := ScopeArgs{
		SpendLimit: "100ugnot",
		AllowPaths: paths,
	}
	scope, warns, err := ResolveScope(args, p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scope.AllowPaths) != 3 {
		t.Errorf("AllowPaths count: got %d, want 3 (clamped)", len(scope.AllowPaths))
	}
	if len(warns) == 0 {
		t.Error("expected a warning for clamped allow_paths")
	}
	if !strings.Contains(warns[0], "allow_paths") {
		t.Errorf("warning should mention allow_paths: %q", warns[0])
	}
}

// ---- BypassHardLimits skips layer 4

func TestResolveScope_bypassSkipsClamps(t *testing.T) {
	p := &profiles.Profile{
		ChainType:        profiles.ChainTypeMainnet,
		BypassHardLimits: true,
	}
	args := ScopeArgs{
		SpendLimit: "999999ugnot",
		ExpiresIn:  "720h",
		AllowPaths: []string{"gno.land/r/a", "gno.land/r/b", "gno.land/r/c", "gno.land/r/d"},
	}
	scope, warns, err := ResolveScope(args, p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warns) != 0 {
		t.Errorf("expected no warnings with bypass: %v", warns)
	}
	if scope.SpendLimit != "999999ugnot" {
		t.Errorf("SpendLimit: got %q, want unchanged 999999ugnot", scope.SpendLimit)
	}
	if scope.ExpiresIn != 720*time.Hour {
		t.Errorf("ExpiresIn: got %v, want unchanged 720h", scope.ExpiresIn)
	}
	if len(scope.AllowPaths) != 4 {
		t.Errorf("AllowPaths count: got %d, want unchanged 4", len(scope.AllowPaths))
	}
}

// ---- Error: empty allow_paths and allow_run=false

func TestResolveScope_emptyAllowPathsAndNoRunError(t *testing.T) {
	p := testProfile(profiles.ChainTypeTestnet)
	args := ScopeArgs{}
	_, _, err := ResolveScope(args, p)
	if err == nil {
		t.Fatal("expected error for empty allow_paths + allow_run=false")
	}
	if !strings.Contains(err.Error(), "allow_paths") || !strings.Contains(err.Error(), "allow_run") {
		t.Errorf("error should mention allow_paths and allow_run: %v", err)
	}
}

// ---- AllowRun-only is accepted (empty allow_paths is fine)

func TestResolveScope_allowRunOnlyOK(t *testing.T) {
	p := testProfile(profiles.ChainTypeTestnet)
	args := ScopeArgs{AllowRun: true}
	scope, _, err := ResolveScope(args, p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !scope.AllowRun {
		t.Error("scope.AllowRun should be true")
	}
	if len(scope.AllowPaths) != 0 {
		t.Errorf("expected empty AllowPaths, got %v", scope.AllowPaths)
	}
}

// ---- Both allow_paths and allow_run are accepted

func TestResolveScope_allowPathsPlusAllowRun(t *testing.T) {
	p := testProfile(profiles.ChainTypeTestnet)
	args := ScopeArgs{
		AllowPaths: []string{"gno.land/r/myorg/blog"},
		AllowRun:   true,
	}
	scope, _, err := ResolveScope(args, p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !scope.AllowRun {
		t.Error("scope.AllowRun should be true")
	}
	if len(scope.AllowPaths) != 1 {
		t.Errorf("AllowPaths: got %v", scope.AllowPaths)
	}
}

// ---- AllowPaths-only with allow_run=false still works

func TestResolveScope_allowPathsOnlyWorks(t *testing.T) {
	p := testProfile(profiles.ChainTypeTestnet)
	args := ScopeArgs{
		AllowPaths: []string{"gno.land/r/myorg/blog"},
		AllowRun:   false,
	}
	scope, _, err := ResolveScope(args, p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if scope.AllowRun {
		t.Error("scope.AllowRun should be false")
	}
	if len(scope.AllowPaths) != 1 {
		t.Errorf("AllowPaths: got %v", scope.AllowPaths)
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scope.AllowPaths) != 10 {
		t.Errorf("AllowPaths count: got %d, want 10 (testnet fallback cap)", len(scope.AllowPaths))
	}
	if len(warns) == 0 {
		t.Error("expected a warning for clamped allow_paths")
	}
}
