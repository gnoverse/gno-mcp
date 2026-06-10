// Package untrusted wraps chain-derived bytes in an envelope that marks them as
// data, never instructions, for an LLM consuming a tool result. The gno chain is
// open — any realm's content is attacker-influenceable — so all chain bytes are
// treated as untrusted regardless of which realm produced them.
package untrusted

import (
	"fmt"
	"regexp"
)

const closeTag = "</untrusted_content>"

// envTagRE matches the start of an untrusted_content tag — opening OR closing —
// in any case and tolerating inner whitespace. Both are neutralized in chain
// content: a forged closing tag would end the envelope early, and a forged
// opening tag (e.g. <untrusted_content kind="system">) could trick an LLM into
// treating the following text as a fresh, more-trusted envelope.
var envTagRE = regexp.MustCompile(`(?i)<(\s*/?\s*untrusted_content)`)

// Neutralize escapes every envelope tag in s (its opening '<' is HTML-escaped)
// without adding an envelope. Use it on mixed-trust text that may embed
// chain-derived bytes — e.g. error messages carrying realm panic strings — so
// that text cannot forge an envelope or close a real one.
func Neutralize(s string) string {
	return envTagRE.ReplaceAllString(s, "&lt;$1")
}

// Wrap returns body inside an <untrusted_content> envelope tagged with kind and
// source. Any envelope tag embedded in body is neutralized first so chain
// content cannot escape or forge the envelope.
func Wrap(body, kind, source string) string {
	return fmt.Sprintf("<untrusted_content kind=%q source=%q>\n%s\n%s", kind, source, Neutralize(body), closeTag)
}
