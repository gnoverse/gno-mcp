package client

import (
	"context"
	"testing"
)

func TestFakeNetworkInfo(t *testing.T) {
	f := NewFake()
	ni, err := f.NetworkInfo(context.Background(), "gno.land")
	if err != nil {
		t.Fatal(err)
	}
	if ni.Chain != "test" {
		t.Errorf("want chain=test, got %s", ni.Chain)
	}
}

func TestFakeFaucetThenAddressInfo(t *testing.T) {
	f := NewFake()
	if err := f.FaucetRequest(context.Background(), "", "g1abc"); err != nil {
		t.Fatal(err)
	}
	a, err := f.AddressInfo(context.Background(), "", "g1abc")
	if err != nil {
		t.Fatal(err)
	}
	if a.Balance == "0ugnot" {
		t.Error("faucet did not credit address")
	}
}
