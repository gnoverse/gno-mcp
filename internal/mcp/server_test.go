package mcp

import (
	"path/filepath"
	"testing"

	"github.com/gnolang/gno-mcp/internal/audit"
	"github.com/gnolang/gno-mcp/internal/client"
)

func TestNew(t *testing.T) {
	a, err := audit.Open(filepath.Join(t.TempDir(), "audit.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	s := New(client.NewFake(), a)
	if s == nil {
		t.Fatal("New returned nil")
	}
}
