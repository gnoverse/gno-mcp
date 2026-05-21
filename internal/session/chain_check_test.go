package session

import (
	"context"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/chain"
)

func TestQueryChain_active(t *testing.T) {
	fake := chain.NewFake()
	fake.SetSession("gpub1active", chain.SessionStatus{
		Active:         true,
		AllowPaths:     []string{"gno.land/r/test/blog"},
		SpendLimit:     "1000000ugnot",
		SpendRemaining: "900000ugnot",
		ExpiresAt:      9999999999,
	})
	var resolver chain.Resolver = func(profile string) chain.Client { return fake }

	res, err := queryChain(context.Background(), resolver, "testnet", "gpub1active")
	if err != nil {
		t.Fatalf("queryChain: %v", err)
	}
	if !res.Active {
		t.Fatal("expected Active=true, got false")
	}
	if res.Status.SpendRemaining != "900000ugnot" {
		t.Errorf("SpendRemaining = %q, want \"900000ugnot\"", res.Status.SpendRemaining)
	}
}

func TestQueryChain_inactive(t *testing.T) {
	fake := chain.NewFake()
	fake.SetSession("gpub1expired", chain.SessionStatus{
		Active: false,
	})
	var resolver chain.Resolver = func(profile string) chain.Client { return fake }

	res, err := queryChain(context.Background(), resolver, "testnet", "gpub1expired")
	if err != nil {
		t.Fatalf("queryChain: %v", err)
	}
	if res.Active {
		t.Fatal("expected Active=false, got true")
	}
}

func TestQueryChain_unknownProfileError(t *testing.T) {
	var resolver chain.Resolver = func(profile string) chain.Client { return nil }

	_, err := queryChain(context.Background(), resolver, "missing-profile", "gpub1anything")
	if err == nil {
		t.Fatal("expected error for nil client, got nil")
	}
}
