package testmcp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/gnolang/gno-mcp/internal/audit"
	"github.com/gnolang/gno-mcp/internal/client"
	gnomcp "github.com/gnolang/gno-mcp/internal/mcp"
	"github.com/gnolang/gno-mcp/internal/session"
	"github.com/mark3labs/mcp-go/mcp"
)

type Harness struct {
	Srv     *gnomcp.Server
	Client  *client.Fake
	Audit   *audit.Log
	Session *session.Manager
}

func New(t *testing.T) *Harness {
	t.Helper()
	f := client.NewFake()
	a, err := audit.Open(filepath.Join(t.TempDir(), "audit.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	// Tests get a session whose balance fetcher reads from the fake's
	// Addresses map. That lets a test seed a credited address and call
	// Refresh to flip the session to authenticated.
	sess := session.New(session.Options{
		Network: "staging.gno.land",
		Balance: func(ctx context.Context, network, addr string) (int64, error) {
			info, err := f.AddressInfo(ctx, network, addr)
			if err != nil {
				return 0, err
			}
			// Parse "1000000ugnot" → 1000000. Tolerate trailing units.
			var ugnot int64
			for _, c := range info.Balance {
				if c < '0' || c > '9' {
					break
				}
				ugnot = ugnot*10 + int64(c-'0')
			}
			return ugnot, nil
		},
	})
	return &Harness{
		Srv:     gnomcp.NewWithSession(f, a, sess),
		Client:  f,
		Audit:   a,
		Session: sess,
	}
}

// ListTools returns the names of all tools registered in the server.
func (h *Harness) ListTools(t *testing.T) []string {
	t.Helper()
	return h.Srv.ListToolNames()
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

// TextContent returns the first text content from a result as a string.
func TextContent(t *testing.T, r *mcp.CallToolResult) string {
	t.Helper()
	if len(r.Content) == 0 {
		t.Fatal("empty content")
	}
	tc, ok := r.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("first content not text: %T", r.Content[0])
	}
	return tc.Text
}
