package mcp

import (
	"strings"
	"testing"
)

func TestUntrustedEnvelope(t *testing.T) {
	got := UntrustedEnvelope("render", "gno.land/r/demo/x", "hello")
	for _, want := range []string{"<untrusted_content", "kind=\"render\"", "hello", "</untrusted_content>"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %s", want, got)
		}
	}
}
