package tools_test

import (
	"testing"
	"time"

	"github.com/gnolang/gno-mcp/internal/audit"
	"github.com/gnolang/gno-mcp/internal/mcp/testmcp"
)

func TestAuditTail(t *testing.T) {
	h := testmcp.New(t)

	// Append 3 entries
	for i, tool := range []string{"gno_get", "gno_eval", "gno_inspect"} {
		if err := h.Audit.Append(audit.Entry{
			Time:    time.Now(),
			Tool:    tool,
			Network: "gno.land",
			Result:  "ok",
			Args:    map[string]any{"i": i},
		}); err != nil {
			t.Fatalf("append: %v", err)
		}
	}

	// Request limit=2, should get 2 most recent
	res := h.Call(t, "gno_audit_tail", map[string]any{"limit": 2})
	if res.IsError {
		t.Fatalf("unexpected error: %v", res.Content)
	}

	var entries []audit.Entry
	testmcp.AsJSON(t, res, &entries)
	if len(entries) != 2 {
		t.Errorf("expected 2 entries with limit=2, got %d", len(entries))
	}
	// Most recent 2 should be gno_eval and gno_inspect
	if entries[0].Tool != "gno_eval" {
		t.Errorf("expected gno_eval, got %s", entries[0].Tool)
	}
	if entries[1].Tool != "gno_inspect" {
		t.Errorf("expected gno_inspect, got %s", entries[1].Tool)
	}
}

func TestAuditTail_Empty(t *testing.T) {
	h := testmcp.New(t)
	res := h.Call(t, "gno_audit_tail", map[string]any{})
	if res.IsError {
		t.Fatalf("unexpected error: %v", res.Content)
	}
	// Should return null or [] for empty log
	text := testmcp.TextContent(t, res)
	if text != "null" && text != "[]" {
		// Both are acceptable for empty
		var entries []audit.Entry
		testmcp.AsJSON(t, res, &entries)
		if len(entries) != 0 {
			t.Errorf("expected empty entries, got: %s", text)
		}
	}
}
