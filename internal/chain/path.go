package chain

import "github.com/gnolang/gno/gnovm/pkg/gnolang"

// IsReadablePackagePath reports whether pkgPath is a realm (/r/) or pure (/p/)
// user package — the kinds gno_read and gno_inspect can fetch. Mirrors the
// chain's own predicates (single source of truth); returns false for stdlib,
// ephemeral, run, and _test paths.
func IsReadablePackagePath(pkgPath string) bool {
	return gnolang.IsRealmPath(pkgPath) || gnolang.IsPPackagePath(pkgPath)
}
