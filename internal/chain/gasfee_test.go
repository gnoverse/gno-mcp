package chain

import "testing"

// estimateGasWanted scales a simulation's GasUsed up by the gasMargin (1.2×),
// rounding the limit up so headroom is never lost to integer truncation. A
// degenerate (non-positive) GasUsed falls back to the DefaultGasWanted ceiling.
func TestEstimateGasWanted(t *testing.T) {
	for _, tc := range []struct {
		name    string
		gasUsed int64
		want    int64
	}{
		{"typical_addpkg", 5_000_000, 6_000_000},      // 5M × 1.2
		{"namereg_over_10M", 12_000_000, 14_400_000},  // 12M × 1.2
		{"ceil_rounds_up", 1, 2},                      // ceil(1.2) = 2, not 1
		{"ceil_rounds_up_small", 10, 12},              // 10 × 1.2 = 12 exact
		{"ceil_rounds_up_nonexact", 7, 9},             // ceil(8.4) = 9
		{"zero_falls_back", 0, DefaultGasWanted},      // degenerate simulation
		{"negative_falls_back", -1, DefaultGasWanted}, // never happens, but guard it
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := estimateGasWanted(tc.gasUsed); got != tc.want {
				t.Errorf("estimateGasWanted(%d) = %d, want %d", tc.gasUsed, got, tc.want)
			}
		})
	}
}

// The estimated GasWanted must never exceed the DefaultGasWanted ceiling for a
// simulation that ran to completion: the simulate ante caps GasUsed at the
// ceiling, so GasUsed ≤ DefaultGasWanted, and even at the cap the +20% margin is
// the only thing that can push past it — that overshoot is acceptable headroom,
// but a tx using the full ceiling is already pathological. Document the boundary.
func TestEstimateGasWanted_atCeiling(t *testing.T) {
	got := estimateGasWanted(DefaultGasWanted)
	want := DefaultGasWanted * gasMarginNum / gasMarginDen
	if got != want {
		t.Errorf("estimateGasWanted(ceiling) = %d, want %d", got, want)
	}
}
