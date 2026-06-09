package chain

import "testing"

func TestIsReadablePackagePath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"gno.land/r/demo/foo", true},       // realm
		{"gno.land/p/demo/lib", true},       // pure
		{"gno.land/r/x", true},              // minimal realm
		{"gno.land/r/demo/foo_test", false}, // _test excluded
		{"gno.land/e/g1abc/run", false},     // ephemeral
		{"std", false},                      // stdlib
		{"", false},
		{"not a path", false},
	}
	for _, tc := range cases {
		if got := IsReadablePackagePath(tc.path); got != tc.want {
			t.Errorf("IsReadablePackagePath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}
