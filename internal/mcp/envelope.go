package mcp

import "strings"

// UntrustedEnvelope wraps external data so the LLM treats it as data, not instructions.
func UntrustedEnvelope(kind, source, content string) string {
	var b strings.Builder
	b.WriteString("<untrusted_content kind=\"")
	b.WriteString(kind)
	b.WriteString("\" source=\"")
	b.WriteString(source)
	b.WriteString("\">\n")
	b.WriteString(content)
	b.WriteString("\n</untrusted_content>\n")
	return b.String()
}
