package tools_test

import (
	"testing"

	"github.com/gnolang/gno-mcp/internal/client"
	"github.com/gnolang/gno-mcp/internal/mcp/testmcp"
)

func TestAddressInfo(t *testing.T) {
	h := testmcp.New(t)
	addr := "g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5"
	h.Client.Addresses[addr] = &client.AddressInfo{
		Address:  addr,
		Balance:  "1000000ugnot",
		Sequence: 5,
		Account:  42,
		Txs: []client.TxSummary{
			{Hash: "hash1", Height: 100, Result: "ok"},
			{Hash: "hash2", Height: 101, Result: "ok"},
		},
	}

	res := h.Call(t, "gno_address_info", map[string]any{
		"address": addr,
		"network": "gno.land",
	})
	if res.IsError {
		t.Fatalf("unexpected error: %v", res.Content)
	}

	var got client.AddressInfo
	testmcp.AsJSON(t, res, &got)
	if got.Address != addr {
		t.Errorf("wrong address: %s", got.Address)
	}
	if got.Balance != "1000000ugnot" {
		t.Errorf("wrong balance: %s", got.Balance)
	}
	if len(got.Txs) != 2 {
		t.Errorf("expected 2 txs, got %d", len(got.Txs))
	}
}

func TestAddressInfo_TxCap(t *testing.T) {
	h := testmcp.New(t)
	addr := "g1test"
	txs := make([]client.TxSummary, 30)
	for i := range txs {
		txs[i] = client.TxSummary{Hash: "hash", Height: int64(i), Result: "ok"}
	}
	h.Client.Addresses[addr] = &client.AddressInfo{
		Address: addr,
		Balance: "0ugnot",
		Txs:     txs,
	}

	res := h.Call(t, "gno_address_info", map[string]any{"address": addr})
	if res.IsError {
		t.Fatalf("unexpected error: %v", res.Content)
	}

	var got client.AddressInfo
	testmcp.AsJSON(t, res, &got)
	if len(got.Txs) != 20 {
		t.Errorf("expected 20 txs (capped), got %d", len(got.Txs))
	}
}
