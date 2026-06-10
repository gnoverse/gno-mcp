package read

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/chain"
)

func TestStatus_reportsConfigAndLiveTip(t *testing.T) {
	f := chain.NewFake()
	f.SetStatus(chain.NodeStatus{
		ChainID:   "test5",
		Height:    4242,
		BlockTime: time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC),
	})

	s := newBaseTestServer(t)
	RegisterStatus(s, constResolver(f))
	res, err := s.Registry().Call(context.Background(), "gno_status", map[string]any{
		"profile": "testnet5",
	})
	require.NoError(t, err)
	assert.Contains(t, res.Text, "4242")
	assert.Contains(t, res.Text, `<untrusted_content kind="status"`, "node-sourced text must be wrapped")
	assert.NotContains(t, res.Text, "mismatch")
	require.NotNil(t, res.StructuredContent)
	assert.Equal(t, "testnet5", res.StructuredContent["profile"])
	assert.Equal(t, "test5", res.StructuredContent["chain_id"])
	assert.Equal(t, "http://127.0.0.1:26657", res.StructuredContent["rpc_url"])
	assert.Equal(t, int64(4242), res.StructuredContent["height"])
	assert.Equal(t, "2026-06-10T12:00:00Z", res.StructuredContent["block_time"])
}

func TestStatus_flagsChainIDMismatch(t *testing.T) {
	f := chain.NewFake()
	f.SetStatus(chain.NodeStatus{ChainID: "dev", Height: 7})

	s := newBaseTestServer(t)
	RegisterStatus(s, constResolver(f))
	res, err := s.Registry().Call(context.Background(), "gno_status", map[string]any{
		"profile": "testnet5",
	})
	require.NoError(t, err)
	assert.Contains(t, res.Text, "mismatch", "node reporting a different chain-id than the profile declares must be flagged")
	assert.Equal(t, "dev", res.StructuredContent["node_chain_id"])
}

func TestStatus_nodeUnreachableStillReportsConfig(t *testing.T) {
	f := chain.NewFake()
	f.SetStatusError(assert.AnError)

	s := newBaseTestServer(t)
	RegisterStatus(s, constResolver(f))
	res, err := s.Registry().Call(context.Background(), "gno_status", map[string]any{
		"profile": "testnet5",
	})
	require.NoError(t, err, "a dead node is a finding, not a tool failure — config info must still flow")
	assert.Contains(t, res.Text, "http://127.0.0.1:26657")
	assert.Equal(t, "test5", res.StructuredContent["chain_id"])
	assert.NotEmpty(t, res.StructuredContent["height_error"])
	assert.NotContains(t, res.StructuredContent, "height")
}

func TestStatus_unknownProfile(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterStatus(s, constResolver(chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_status", map[string]any{
		"profile": "nope",
	})
	require.Error(t, err)
}
