package chain

import "testing"

func TestNewReal_validRPCURL(t *testing.T) {
	r, err := NewReal("https://rpc.test5.gno.land:443", "test5")
	if err != nil {
		t.Fatalf("NewReal: %v", err)
	}
	if r == nil {
		t.Fatal("NewReal returned nil")
	}
}

func TestNewReal_emptyRPCURL(t *testing.T) {
	_, err := NewReal("", "test5")
	if err == nil {
		t.Fatal("expected error for empty rpc-url")
	}
}
