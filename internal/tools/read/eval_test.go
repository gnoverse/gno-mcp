package read

import (
	"context"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/gnoverse/gno-mcp/internal/server"
)

// newEvalTestServer builds a server with both Render and Eval registered,
// using the given client for all profiles.
func newEvalTestServer(t *testing.T, c chain.Client) *server.Server {
	t.Helper()
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"testnet5": {ChainType: "testnet", RPCURL: "x", ChainID: "test5"},
	}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	s := server.NewServer(cfg, "")
	RegisterEval(s, func(profile string) chain.Client { return c })
	return s
}

func TestEval_returnsText(t *testing.T) {
	f := chain.NewFake()
	f.SetEval("gno.land/r/foo", "Bar()", "(42 int)")

	s := newEvalTestServer(t, f)
	res, err := s.Registry().Call(context.Background(), "gno_eval", map[string]any{
		"realm":   "gno.land/r/foo",
		"expr":    "Bar()",
		"profile": "testnet5",
	})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if res.Text != "(42 int)" {
		t.Errorf("Text = %q, want %q", res.Text, "(42 int)")
	}
}

func TestEval_requiresExpr(t *testing.T) {
	s := newEvalTestServer(t, chain.NewFake())
	_, err := s.Registry().Call(context.Background(), "gno_eval", map[string]any{
		"realm":   "gno.land/r/foo",
		"profile": "testnet5",
	})
	if err == nil {
		t.Fatal("expected error when expr is missing")
	}
}

func TestEval_requiresRealm(t *testing.T) {
	s := newEvalTestServer(t, chain.NewFake())
	_, err := s.Registry().Call(context.Background(), "gno_eval", map[string]any{
		"expr":    "Bar()",
		"profile": "testnet5",
	})
	if err == nil {
		t.Fatal("expected error when realm is missing")
	}
}

func TestEval_rejectsNonStringExpr(t *testing.T) {
	s := newEvalTestServer(t, chain.NewFake())
	_, err := s.Registry().Call(context.Background(), "gno_eval", map[string]any{
		"realm":   "gno.land/r/foo",
		"expr":    42,
		"profile": "testnet5",
	})
	if err == nil {
		t.Fatal("expected type error when expr is not a string")
	}
	// stringArg returns "expr: expected string, got int"
	// just check it errors; message format is tested implicitly.
}

func TestEval_unknownProfileReturnsError(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"testnet5": {ChainType: "testnet", RPCURL: "x", ChainID: "test5"},
	}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	s := server.NewServer(cfg, "")
	RegisterEval(s, func(profile string) chain.Client {
		if profile == "testnet5" {
			return chain.NewFake()
		}
		return nil
	})
	_, err := s.Registry().Call(context.Background(), "gno_eval", map[string]any{
		"realm":   "gno.land/r/foo",
		"expr":    "Bar()",
		"profile": "ghost",
	})
	if err == nil {
		t.Fatal("expected error when resolver returns nil for unknown profile")
	}
}
