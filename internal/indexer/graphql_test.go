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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		assert.True(t, strings.Contains(string(body), "transactions"), "expected transactions query in body, got %s", body)
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
	require.NoError(t, err, "History")
	require.Len(t, events, 1, "expected 1 event, got %d: %+v", len(events), events)
	assert.Equal(t, "0xabc", events[0].Hash)
	assert.Equal(t, int64(100), events[0].Height)
	assert.Equal(t, "MsgAddPackage", events[0].Kind)
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
	require.NoError(t, err, "History")
	require.Len(t, events, 1)
	ev := events[0]
	assert.Equal(t, "MsgCall", ev.Kind)
	assert.Equal(t, "g1caller", ev.Caller)
	assert.Equal(t, "Transfer", ev.Func)
	require.Len(t, ev.Args, 2)
	assert.Equal(t, "g1dest", ev.Args[0])
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
	require.NoError(t, err, "Activity")
	require.Len(t, events, 2, "expected 2 events (MsgCall + MsgRun), got %d: %+v", len(events), events)
	kinds := map[string]bool{}
	for _, e := range events {
		kinds[e.Kind] = true
	}
	assert.True(t, kinds["MsgCall"] && kinds["MsgRun"], "expected MsgCall and MsgRun, got kinds %v", kinds)
	assert.False(t, kinds["MsgAddPackage"], "MsgAddPackage should have been filtered out")
}

func TestGraphQL_Activity_rejectsSince(t *testing.T) {
	c := NewGraphQL("http://127.0.0.1:1") // URL won't be called
	since := time.Now().Add(-time.Hour)
	_, err := c.Activity(context.Background(), "gno.land/r/foo", &since, nil)
	require.Error(t, err, "expected error when since is non-nil")
	assert.True(t, strings.Contains(err.Error(), "error_unavailable"), "error should contain 'error_unavailable', got: %v", err)
	assert.True(t, strings.Contains(err.Error(), "time filtering"), "error should contain 'time filtering', got: %v", err)
}

func TestGraphQL_Activity_rejectsUntil(t *testing.T) {
	c := NewGraphQL("http://127.0.0.1:1") // URL won't be called
	until := time.Now()
	_, err := c.Activity(context.Background(), "gno.land/r/foo", nil, &until)
	require.Error(t, err, "expected error when until is non-nil")
	assert.True(t, strings.Contains(err.Error(), "error_unavailable"), "error should contain 'error_unavailable', got: %v", err)
	assert.True(t, strings.Contains(err.Error(), "time filtering"), "error should contain 'time filtering', got: %v", err)
}

func TestGraphQL_List_notSupported(t *testing.T) {
	c := NewGraphQL("http://127.0.0.1:1") // URL won't be called
	_, err := c.List(context.Background(), ListFilter{})
	require.Error(t, err, "expected error for unsupported List")
	assert.True(t, strings.Contains(err.Error(), "not supported"), "error should mention 'not supported', got: %v", err)
}

func TestGraphQL_unreachable(t *testing.T) {
	c := NewGraphQL("http://127.0.0.1:1")
	_, err := c.History(context.Background(), "gno.land/r/x")
	require.Error(t, err, "expected error for unreachable indexer")
	assert.True(t, strings.Contains(err.Error(), "error_unavailable"), "error should mention 'error_unavailable', got: %v", err)
}

func TestGraphQL_nonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := NewGraphQL(srv.URL)
	_, err := c.History(context.Background(), "gno.land/r/x")
	require.Error(t, err, "expected error for non-200 status")
	assert.True(t, strings.Contains(err.Error(), "503"), "error should mention status code, got: %v", err)
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
	require.Error(t, err, "expected error from GraphQL errors envelope")
	assert.True(t, strings.Contains(err.Error(), "some graphql error"), "error should mention graphql error message, got: %v", err)
}
