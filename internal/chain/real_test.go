package chain

import (
	"context"
	"strings"
	"testing"
)

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

func TestReal_File_rejectsEmptyFile(t *testing.T) {
	r, err := NewReal("https://rpc.test5.gno.land:443", "test5")
	if err != nil {
		t.Fatalf("NewReal: %v", err)
	}
	_, err = r.File(context.Background(), "gno.land/r/foo", "")
	if err == nil {
		t.Fatal("expected error for empty file name (use ListFiles instead)")
	}
	if !strings.Contains(err.Error(), "ListFiles") {
		t.Errorf("error should steer caller to ListFiles, got %q", err)
	}
}
