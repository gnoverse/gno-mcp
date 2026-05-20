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

func TestGraphQL_Activity_filtersByKind(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(txResponse([]any{
			map[string]any{
				"hash":         "0xcall",
				"block_height": 1,
				"messages": []any{
					map[string]any{
						"typeUrl": "exec",
						"route":   "vm",
						"value": map[string]any{
							"__typename": "MsgCall",
							"caller":     "g1c",
							"pkg_path":   "gno.land/r/foo",
							"func":       "Transfer",
							"args":       []any{},
						},
					},
				},
			},
			map[string]any{
				"hash":         "0xaddpkg",
				"block_height": 2,
				"messages": []any{
					map[string]any{
						"typeUrl": "add_package",
						"route":   "vm",
						"value": map[string]any{
							"__typename": "MsgAddPackage",
							"creator":    "g1d",
							"package":    map[string]any{"path": "gno.land/r/foo"},
						},
					},
				},
			},
			map[string]any{
				"hash":         "0xrun",
				"block_height": 3,
				"messages": []any{
					map[string]any{
						"typeUrl": "run",
						"route":   "vm",
						"value": map[string]any{
							"__typename": "MsgRun",
							"caller":     "g1r",
						},
					},
				},
			},
		}))
	}))
	defer srv.Close()

	c := NewGraphQL(srv.URL)
	events, err := c.Activity(context.Background(), "gno.land/r/foo", nil, nil)
	if err != nil {
		t.Fatalf("Activity: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events (MsgCall + MsgRun), got %d: %+v", len(events), events)
	}
	kinds := map[string]bool{}
	for _, e := range events {
		kinds[e.Kind] = true
	}
	if !kinds["MsgCall"] || !kinds["MsgRun"] {
		t.Errorf("expected MsgCall and MsgRun, got kinds %v", kinds)
	}
	if kinds["MsgAddPackage"] {
		t.Error("MsgAddPackage should have been filtered out")
	}
}

func TestGraphQL_Activity_rejectsSince(t *testing.T) {
	c := NewGraphQL("http://127.0.0.1:1") // URL won't be called
	since := time.Now().Add(-time.Hour)
	_, err := c.Activity(context.Background(), "gno.land/r/foo", &since, nil)
	if err == nil {
		t.Fatal("expected error when since is non-nil")
	}
	if !strings.Contains(err.Error(), "error_unavailable") {
		t.Errorf("error should contain 'error_unavailable', got: %v", err)
	}
	if !strings.Contains(err.Error(), "time filtering") {
		t.Errorf("error should contain 'time filtering', got: %v", err)
	}
}

func TestGraphQL_Activity_rejectsUntil(t *testing.T) {
	c := NewGraphQL("http://127.0.0.1:1") // URL won't be called
	until := time.Now()
	_, err := c.Activity(context.Background(), "gno.land/r/foo", nil, &until)
	if err == nil {
		t.Fatal("expected error when until is non-nil")
	}
	if !strings.Contains(err.Error(), "error_unavailable") {
		t.Errorf("error should contain 'error_unavailable', got: %v", err)
	}
	if !strings.Contains(err.Error(), "time filtering") {
		t.Errorf("error should contain 'time filtering', got: %v", err)
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
