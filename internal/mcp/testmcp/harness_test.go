package testmcp

import "testing"

func TestHelloRoundtrip(t *testing.T) {
	h := New(t)
	res := h.Call(t, "gno_hello", nil)
	if len(res.Content) == 0 {
		t.Fatal("no content")
	}
}
