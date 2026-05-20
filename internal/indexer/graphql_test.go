package indexer

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// txResponse builds a minimal valid GraphQL envelope for a transactions list.
func txResponse(txs []any) map[string]any {
	return map[string]any{
		"data": map[string]any{
			"transactions": txs,
		},
	}
}

func TestGraphQL_History(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "transactions") {
			t.Errorf("expected transactions query in body, got %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(txResponse([]any{
			map[string]any{
				"hash":         "0xabc",
				"block_height": 100,
				"messages": []any{
					map[string]any{
						"typeUrl": "add_package",
						"route":   "vm",
						"value": map[string]any{
							"__typename": "MsgAddPackage",
							"creator":    "g1deployer",
							"package":    map[string]any{"path": "gno.land/r/foo"},
						},
					},
				},
			},
		}))
	}))
	defer srv.Close()

	c := NewGraphQL(srv.URL)
	events, err := c.History(context.Background(), "gno.land/r/foo")
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d: %+v", len(events), events)
	}
	if events[0].Hash != "0xabc" {
		t.Errorf("Hash = %q, want %q", events[0].Hash, "0xabc")
	}
	if events[0].Height != 100 {
		t.Errorf("Height = %d, want 100", events[0].Height)
	}
	if events[0].Kind != "MsgAddPackage" {
		t.Errorf("Kind = %q, want MsgAddPackage", events[0].Kind)
	}
}

func TestGraphQL_History_MsgCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(txResponse([]any{
			map[string]any{
				"hash":         "0xdef",
				"block_height": 200,
				"messages": []any{
					map[string]any{
						"typeUrl": "exec",
						"route":   "vm",
						"value": map[string]any{
							"__typename": "MsgCall",
							"caller":     "g1caller",
							"pkg_path":   "gno.land/r/foo",
							"func":       "Transfer",
							"args":       []any{"g1dest", "100"},
						},
					},
				},
			},
		}))
	}))
	defer srv.Close()

	c := NewGraphQL(srv.URL)
	events, err := c.History(context.Background(), "gno.land/r/foo")
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	ev := events[0]
	if ev.Kind != "MsgCall" {
		t.Errorf("Kind = %q, want MsgCall", ev.Kind)
	}
	if ev.Caller != "g1caller" {
		t.Errorf("Caller = %q, want g1caller", ev.Caller)
	}
	if ev.Func != "Transfer" {
		t.Errorf("Func = %q, want Transfer", ev.Func)
	}
	if len(ev.Args) != 2 || ev.Args[0] != "g1dest" {
		t.Errorf("Args = %v, want [g1dest 100]", ev.Args)
	}
}

func TestGraphQL_Activity_filtersClientSide(t *testing.T) {
	now := time.Now()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return two events; the client should filter by time.
		// We embed time via block_height for sorting, but actually Activity
		// stores time in a separate field we set in the test server response.
		// Since Transaction has no time field in the schema, we set TxEvent.Time
		// via our internal logic; the test just checks we get back all events
		// (client-side filtering happens on TxEvent.Time which defaults to zero).
		_ = json.NewEncoder(w).Encode(txResponse([]any{
			map[string]any{
				"hash":         "old",
				"block_height": 1,
				"messages":     []any{},
			},
			map[string]any{
				"hash":         "new",
				"block_height": 2,
				"messages":     []any{},
			},
		}))
	}))
	defer srv.Close()

	c := NewGraphQL(srv.URL)
	// With no time bounds both should come through (TxEvent.Time is zero).
	events, err := c.Activity(context.Background(), "gno.land/r/foo", nil, nil)
	if err != nil {
		t.Fatalf("Activity: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d: %+v", len(events), events)
	}

	// With a since bound in the future, zero-time events are filtered out.
	future := now.Add(time.Hour)
	events, err = c.Activity(context.Background(), "gno.land/r/foo", &future, nil)
	if err != nil {
		t.Fatalf("Activity with future since: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events with future since, got %d", len(events))
	}
}

func TestGraphQL_List_notSupported(t *testing.T) {
	c := NewGraphQL("http://127.0.0.1:1") // URL won't be called
	_, err := c.List(context.Background(), ListFilter{})
	if err == nil {
		t.Fatal("expected error for unsupported List")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Errorf("error should mention 'not supported', got: %v", err)
	}
}

func TestGraphQL_unreachable(t *testing.T) {
	c := NewGraphQL("http://127.0.0.1:1")
	_, err := c.History(context.Background(), "gno.land/r/x")
	if err == nil {
		t.Fatal("expected error for unreachable indexer")
	}
	if !strings.Contains(err.Error(), "error_unavailable") {
		t.Errorf("error should mention 'error_unavailable', got: %v", err)
	}
}

func TestGraphQL_nonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := NewGraphQL(srv.URL)
	_, err := c.History(context.Background(), "gno.land/r/x")
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("error should mention status code, got: %v", err)
	}
}

func TestGraphQL_graphqlError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []any{
				map[string]any{"message": "some graphql error"},
			},
		})
	}))
	defer srv.Close()

	c := NewGraphQL(srv.URL)
	_, err := c.History(context.Background(), "gno.land/r/x")
	if err == nil {
		t.Fatal("expected error from GraphQL errors envelope")
	}
	if !strings.Contains(err.Error(), "some graphql error") {
		t.Errorf("error should mention graphql error message, got: %v", err)
	}
}
