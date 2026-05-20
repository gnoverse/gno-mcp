//go:build integration
// +build integration

package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/chain"
)

func TestSmoke_renderGnolandHome(t *testing.T) {
	c, err := chain.NewReal("https://rpc.test11.testnets.gno.land:443", "test11")
	if err != nil {
		t.Fatalf("NewReal: %v", err)
	}
	out, err := c.Render(context.Background(), "gno.land/r/gnoland/home", "")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("expected non-empty render output")
	}
	if !strings.Contains(strings.ToLower(out), "gno") {
		t.Errorf("expected 'gno' somewhere in homepage output, got first 200 chars: %s", firstN(out, 200))
	}
}

func TestSmoke_inspectGrc20(t *testing.T) {
	c, err := chain.NewReal("https://rpc.test11.testnets.gno.land:443", "test11")
	if err != nil {
		t.Fatalf("NewReal: %v", err)
	}
	doc, err := c.Doc(context.Background(), "gno.land/p/demo/tokens/grc20")
	if err != nil {
		t.Fatalf("Doc: %v", err)
	}
	if !strings.Contains(doc, "Transfer") {
		t.Errorf("expected grc20 to mention Transfer, got: %s", firstN(doc, 500))
	}
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
