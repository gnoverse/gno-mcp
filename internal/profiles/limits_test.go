package profiles

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			profile:     Profile{ChainID: "dev"},
			wantSpend:   "100000000ugnot",
			wantExpires: 30 * 24 * time.Hour,
			wantPaths:   20,
		},
		{
			name:        "testnet",
			profile:     Profile{ChainID: "test5"},
			wantSpend:   "100000000ugnot",
			wantExpires: 7 * 24 * time.Hour,
			wantPaths:   10,
		},
		{
			name:        "empty chain-id derives testnet limits",
			profile:     Profile{},
			wantSpend:   "100000000ugnot",
			wantExpires: 7 * 24 * time.Hour,
			wantPaths:   10,
		},
		{
			name:        "bypass returns unlimited sentinel",
			profile:     Profile{ChainID: "test5", BypassHardLimits: true},
			wantSpend:   "",
			wantExpires: 0,
			wantPaths:   0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hl := tc.profile.HardLimits()
			assert.Equal(t, tc.wantSpend, hl.MaxSpendLimit, "MaxSpendLimit")
			assert.Equal(t, tc.wantExpires, hl.MaxExpiresIn, "MaxExpiresIn")
			assert.Equal(t, tc.wantPaths, hl.MaxAllowPathsCount, "MaxAllowPathsCount")
		})
	}
}

func TestHardLimits_NoMainnetType(t *testing.T) {
	p := Profile{ChainID: "test5"}
	assert.Equal(t, "100000000ugnot", p.HardLimits().MaxSpendLimit)
}

func TestEffectiveDefaults_profileSetWins(t *testing.T) {
	p := Profile{
		DefaultSpendLimit: "500000ugnot",
		DefaultExpiresIn:  "2h",
	}
	spend, expires, err := p.EffectiveDefaults()
	require.NoError(t, err)
	assert.Equal(t, "500000ugnot", spend)
	assert.Equal(t, 2*time.Hour, expires)
}

func TestEffectiveDefaults_fallbackToHardcoded(t *testing.T) {
	p := Profile{}
	spend, expires, err := p.EffectiveDefaults()
	require.NoError(t, err)
	assert.Equal(t, "100000000ugnot", spend)
	assert.Equal(t, time.Hour, expires)
}

func TestEffectiveDefaults_mixedFallback(t *testing.T) {
	p := Profile{DefaultSpendLimit: "200000ugnot"}
	spend, expires, err := p.EffectiveDefaults()
	require.NoError(t, err)
	assert.Equal(t, "200000ugnot", spend)
	assert.Equal(t, time.Hour, expires)
}

func TestEffectiveDefaults_invalidExpiresInReturnsError(t *testing.T) {
	p := Profile{DefaultExpiresIn: "garbage"}
	_, _, err := p.EffectiveDefaults()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "default-expires-in")
}
