package testmcp

import "testing"

func TestHarnessCreation(t *testing.T) {
	h := New(t)
	if h == nil {
		t.Fatal("New returned nil harness")
	}
}
