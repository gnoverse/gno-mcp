package chain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGasWantedFor pins the sizing of a broadcast's GasWanted from the gas a
// dry-run measured: measured + margin, floored at DefaultGasWanted (so a light
// tx is byte-for-byte unchanged) and capped at the estimate ceiling.
func TestGasWantedFor(t *testing.T) {
	tests := []struct {
		name     string
		measured int64
		want     int64
	}{
		{"light tx floors to default", 2_000_000, DefaultGasWanted},
		{"below floor after margin stays floored", 6_000_000, DefaultGasWanted}, // 6M*3/2=9M < 10M
		{"heavy tx right-sized with margin", 15_000_000, 22_500_000},            // 15M*3/2 — the CLA Sign case
		{"zero measured falls back to floor", 0, DefaultGasWanted},
		{"clamped to estimate ceiling", gasEstimateCeiling, gasEstimateCeiling}, // 1e9*3/2 -> clamp
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, gasWantedFor(tt.measured))
		})
	}
}
