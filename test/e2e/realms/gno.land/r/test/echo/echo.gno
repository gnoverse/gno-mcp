package echo

// Echo returns msg unchanged. Used by the e2e protocol
// to verify gno_call args encoding round-trip (Check 7).
func Echo(cur realm, msg string) string {
	return msg
}
