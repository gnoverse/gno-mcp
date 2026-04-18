package testmcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gnolang/gno-mcp/internal/client"
	gnomcp "github.com/gnolang/gno-mcp/internal/mcp"
	"github.com/mark3labs/mcp-go/mcp"
)

type Harness struct {
	Srv    *gnomcp.Server
	Client *client.Fake
}

func New(t *testing.T) *Harness {
	t.Helper()
	f := client.NewFake()
	return &Harness{Srv: gnomcp.New(f), Client: f}
}

func (h *Harness) Call(t *testing.T, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	res, err := h.Srv.Dispatch(context.Background(), req)
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	return res
}

func AsJSON(t *testing.T, r *mcp.CallToolResult, out any) {
	t.Helper()
	if len(r.Content) == 0 {
		t.Fatal("empty content")
	}
	tc, ok := r.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("first content not text: %T", r.Content[0])
	}
	if err := json.Unmarshal([]byte(tc.Text), out); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, tc.Text)
	}
}
