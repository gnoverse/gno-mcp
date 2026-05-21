package echo

// Echo returns msg unchanged. Used by the Milestone B e2e protocol
// to verify gno_call args encoding round-trip (Check 7).
func Echo(msg string) string {
	return msg
}
