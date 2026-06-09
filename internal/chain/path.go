package chain

import "github.com/gnolang/gno/gnovm/pkg/gnolang"

// IsRealmPath reports whether pkgPath is a realm (/r/) user package — the kind
// gno_render and gno_eval operate on (Render and stateful expression evaluation
// are realm concepts). Returns false for pure packages, stdlib, ephemeral, run,
// and _test paths.
func IsRealmPath(pkgPath string) bool {
	return gnolang.IsRealmPath(pkgPath)
}

// IsReadablePackagePath reports whether pkgPath is a realm (/r/) or pure (/p/)
// user package — the kinds gno_read and gno_inspect can fetch. Mirrors the
// chain's own predicates (single source of truth); returns false for stdlib,
// ephemeral, run, and _test paths.
func IsReadablePackagePath(pkgPath string) bool {
	return gnolang.IsRealmPath(pkgPath) || gnolang.IsPPackagePath(pkgPath)
}
