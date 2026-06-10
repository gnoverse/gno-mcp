package chain

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gnolang/gno/tm2/pkg/amino"
	ctypes "github.com/gnolang/gno/tm2/pkg/bft/rpc/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// statusServer serves a JSON-RPC envelope whose result is the amino encoding
// of a ResultStatus reporting network — the wire shape tm2's Status() decodes.
func statusServer(t *testing.T, network string) *httptest.Server {
	t.Helper()
	var st ctypes.ResultStatus
	st.NodeInfo.Network = network
	return statusServerWith(t, st)
}

// statusServerWith serves the amino encoding of st as the /status result.
// The response echoes the request's id (the tm2 client rejects mismatches).
func statusServerWith(t *testing.T, st ctypes.ResultStatus) *httptest.Server {
	t.Helper()
	result, err := amino.MarshalJSON(&st)
	require.NoError(t, err, "amino-marshal ResultStatus")
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID json.RawMessage `json:"id"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req), "decode JSON-RPC request")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":` + string(req.ID) + `,"result":` + string(result) + `}`))
	}))
}

func TestQueryChainID_reportsNodeNetwork(t *testing.T) {
	srv := statusServer(t, "test5")
	defer srv.Close()

	got, err := QueryChainID(context.Background(), srv.URL)
	require.NoError(t, err)
	assert.Equal(t, "test5", got)
}

func TestQueryChainID_unreachable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := QueryChainID(ctx, "http://127.0.0.1:1")
	require.Error(t, err, "an unreachable node must surface an error")
}

func TestQueryChainID_respectsContextTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := QueryChainID(ctx, srv.URL)
	require.Error(t, err, "a hung node must time out via ctx")
	assert.Less(t, time.Since(start), 2*time.Second, "ctx timeout must bound the call")
}

func TestQueryChainID_emptyURL(t *testing.T) {
	_, err := QueryChainID(context.Background(), "")
	require.Error(t, err)
}

func TestRealStatus_reportsChainIDHeightTime(t *testing.T) {
	var st ctypes.ResultStatus
	st.NodeInfo.Network = "test5"
	st.SyncInfo.LatestBlockHeight = 4242
	st.SyncInfo.LatestBlockTime = time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	srv := statusServerWith(t, st)
	defer srv.Close()

	r, err := NewReal(srv.URL, "test5")
	require.NoError(t, err)

	got, err := r.Status(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "test5", got.ChainID)
	assert.Equal(t, int64(4242), got.Height)
	assert.True(t, st.SyncInfo.LatestBlockTime.Equal(got.BlockTime))
}

func TestFakeStatus_returnsSeeded(t *testing.T) {
	f := NewFake()
	f.SetStatus(NodeStatus{ChainID: "dev", Height: 7})

	got, err := f.Status(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "dev", got.ChainID)
	assert.Equal(t, int64(7), got.Height)
}

func TestFakeStatus_unseededErrors(t *testing.T) {
	f := NewFake()
	_, err := f.Status(context.Background())
	require.Error(t, err)
}

func TestFakeStatus_seededError(t *testing.T) {
	f := NewFake()
	f.SetStatusError(assert.AnError)
	_, err := f.Status(context.Background())
	require.ErrorIs(t, err, assert.AnError)
}
