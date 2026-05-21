package other

// Ping returns a fixed string. Used by the Milestone B e2e protocol
// to exercise scope-mismatch errors (Check 8) and multi-session coexistence
// (Check 9) — a second realm outside the initial session's AllowPaths.
func Ping(cur realm) string {
	return "pong"
}
