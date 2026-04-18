package mcp

import (
	"testing"

	"github.com/gnolang/gno-mcp/internal/client"
)

func TestNew(t *testing.T) {
	s := New(client.NewFake())
	if s == nil {
		t.Fatal("New returned nil")
	}
}
