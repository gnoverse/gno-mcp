package tools_test

import (
	"testing"

	"github.com/gnolang/gno-mcp/internal/client"
	"github.com/gnolang/gno-mcp/internal/mcp/testmcp"
)

func TestInspect(t *testing.T) {
	h := testmcp.New(t)
	h.Client.Realms["gno.land/r/demo/boards"] = &client.RealmInspection{
		Path:      "gno.land/r/demo/boards",
		Files:     []string{"board.gno", "post.gno"},
		GnowebURL: "https://gno.land/r/demo/boards",
		Functions: []client.FuncSig{
			{Name: "NewBoard", Public: true, Params: []string{"name string"}, Return: []string{"int64"}},
		},
	}

	res := h.Call(t, "gno_inspect", map[string]any{
		"path": "gno.land/r/demo/boards",
	})
	if res.IsError {
		t.Fatalf("unexpected error: %v", res.Content)
	}

	var got client.RealmInspection
	testmcp.AsJSON(t, res, &got)
	if got.Path != "gno.land/r/demo/boards" {
		t.Errorf("wrong path: %s", got.Path)
	}
	if len(got.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(got.Files))
	}
	if len(got.Functions) != 1 || got.Functions[0].Name != "NewBoard" {
		t.Errorf("expected NewBoard function, got: %+v", got.Functions)
	}
}

func TestInspect_NotFound(t *testing.T) {
	h := testmcp.New(t)
	res := h.Call(t, "gno_inspect", map[string]any{
		"path": "gno.land/r/nonexistent",
	})
	if !res.IsError {
		t.Error("expected error for nonexistent realm")
	}
}
