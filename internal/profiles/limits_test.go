package profiles

import (
	"strings"
	"testing"
	"time"
)

func TestHardLimits(t *testing.T) {
	cases := []struct {
		name        string
		profile     Profile
		wantSpend   string
		wantExpires time.Duration
		wantPaths   int
	}{
		{
			name:        "local",
			profile:     Profile{ChainType: ChainTypeLocal},
			wantSpend:   "100000000ugnot",
			wantExpires: 30 * 24 * time.Hour,
			wantPaths:   20,
		},
		{
			name:        "testnet",
			profile:     Profile{ChainType: ChainTypeTestnet},
			wantSpend:   "100000000ugnot",
			wantExpires: 7 * 24 * time.Hour,
			wantPaths:   10,
		},
		{
			name:        "unknown falls back to testnet",
			profile:     Profile{ChainType: "foobar"},
			wantSpend:   "100000000ugnot",
			wantExpires: 7 * 24 * time.Hour,
			wantPaths:   10,
		},
		{
			name:        "bypass returns unlimited sentinel",
			profile:     Profile{ChainType: ChainTypeTestnet, BypassHardLimits: true},
			wantSpend:   "",
			wantExpires: 0,
			wantPaths:   0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hl := tc.profile.HardLimits()
			if hl.MaxSpendLimit != tc.wantSpend {
				t.Errorf("MaxSpendLimit: got %q, want %q", hl.MaxSpendLimit, tc.wantSpend)
			}
			if hl.MaxExpiresIn != tc.wantExpires {
				t.Errorf("MaxExpiresIn: got %v, want %v", hl.MaxExpiresIn, tc.wantExpires)
			}
			if hl.MaxAllowPathsCount != tc.wantPaths {
				t.Errorf("MaxAllowPathsCount: got %d, want %d", hl.MaxAllowPathsCount, tc.wantPaths)
			}
		})
	}
}

func TestHardLimits_NoMainnetType(t *testing.T) {
	// A testnet profile gets the moderate (default) limits.
	p := Profile{ChainType: ChainTypeTestnet}
	if got := p.HardLimits().MaxSpendLimit; got != "100000000ugnot" {
		t.Errorf("testnet MaxSpendLimit = %q, want 100000000ugnot", got)
	}
}

func TestEffectiveDefaults_profileSetWins(t *testing.T) {
	p := Profile{
		DefaultSpendLimit: "500000ugnot",
		DefaultExpiresIn:  "2h",
	}
	spend, expires, err := p.EffectiveDefaults()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spend != "500000ugnot" {
		t.Errorf("spend: got %q, want 500000ugnot", spend)
	}
	if expires != 2*time.Hour {
		t.Errorf("expires: got %v, want 2h", expires)
	}
}

func TestEffectiveDefaults_fallbackToHardcoded(t *testing.T) {
	p := Profile{}
	spend, expires, err := p.EffectiveDefaults()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spend != "100000000ugnot" {
		t.Errorf("spend: got %q, want hardcoded 100000000ugnot", spend)
	}
	if expires != time.Hour {
		t.Errorf("expires: got %v, want hardcoded 1h", expires)
	}
}

func TestEffectiveDefaults_mixedFallback(t *testing.T) {
	p := Profile{DefaultSpendLimit: "200000ugnot"}
	spend, expires, err := p.EffectiveDefaults()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spend != "200000ugnot" {
		t.Errorf("spend: got %q, want 200000ugnot", spend)
	}
	if expires != time.Hour {
		t.Errorf("expires: got %v, want hardcoded 1h", expires)
	}
}

func TestEffectiveDefaults_invalidExpiresInReturnsError(t *testing.T) {
	p := Profile{DefaultExpiresIn: "garbage"}
	_, _, err := p.EffectiveDefaults()
	if err == nil {
		t.Fatal("expected error for unparseable default-expires-in")
	}
	if !strings.Contains(err.Error(), "default-expires-in") {
		t.Errorf("error should mention field name: %v", err)
	}
}
