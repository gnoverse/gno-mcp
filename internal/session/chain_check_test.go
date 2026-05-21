package session

import (
	"context"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/chain"
)

func TestQueryChain_active(t *testing.T) {
	fake := chain.NewFake()
	fake.SetSession("g1master", "g1session", chain.SessionStatus{
		Active:         true,
		AllowPaths:     []string{"gno.land/r/test/blog"},
		SpendLimit:     "1000000ugnot",
		SpendRemaining: "900000ugnot",
		ExpiresAt:      9999999999,
	})
	var resolver chain.Resolver = func(profile string) chain.Client { return fake }

	res, err := queryChain(context.Background(), resolver, "testnet", "g1master", "g1session")
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
	fake.SetSession("g1master", "g1expired", chain.SessionStatus{
		Active: false,
	})
	var resolver chain.Resolver = func(profile string) chain.Client { return fake }

	res, err := queryChain(context.Background(), resolver, "testnet", "g1master", "g1expired")
	if err != nil {
		t.Fatalf("queryChain: %v", err)
	}
	if res.Active {
		t.Fatal("expected Active=false, got true")
	}
}

func TestQueryChain_emptyMasterIsUnsupported(t *testing.T) {
	fake := chain.NewFake()
	var resolver chain.Resolver = func(profile string) chain.Client { return fake }

	res, err := queryChain(context.Background(), resolver, "testnet", "", "g1session")
	if err != nil {
		t.Fatalf("queryChain: %v", err)
	}
	if !res.Unsupported {
		t.Error("expected Unsupported=true when master is empty")
	}
	if res.Active {
		t.Error("expected Active=false when master is empty")
	}
}

func TestQueryChain_unknownProfileError(t *testing.T) {
	var resolver chain.Resolver = func(profile string) chain.Client { return nil }

	_, err := queryChain(context.Background(), resolver, "missing-profile", "g1master", "g1session")
	if err == nil {
		t.Fatal("expected error for nil client, got nil")
	}
}
