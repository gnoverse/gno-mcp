package chain

import "testing"

func TestIsRealmPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"gno.land/r/demo/foo", true},
		{"gno.land/r/x", true},
		{"gno.land/p/demo/lib", false}, // pure package is not a realm
		{"std", false},
		{"", false},
		{"not a path", false},
	}
	for _, tc := range cases {
		if got := IsRealmPath(tc.path); got != tc.want {
			t.Errorf("IsRealmPath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

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
