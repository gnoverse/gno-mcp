package read

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBudgetResourceBody_TruncatesLarge(t *testing.T) {
	big := strings.Repeat("x", 5000) // > DefaultBudget (4096)
	body, truncated := budgetBody(big, "https://test11.testnets.gno.land/r/foo")
	require.True(t, truncated, "expected truncation for >4KB body")
	assert.NotEqual(t, big, body, "body should have been replaced by a summary")
}

func TestBudgetResourceBody_KeepsSmall(t *testing.T) {
	body, truncated := budgetBody("small", "https://x")
	assert.False(t, truncated, "small body should not be truncated")
	assert.Equal(t, "small", body, "small body should pass through unchanged")
}

func TestGnowebURLFor(t *testing.T) {
	cases := []struct{ rpc, realm, path, want string }{
		{"https://rpc.test11.testnets.gno.land:443", "gno.land/r/gnoland/home", "", "https://test11.testnets.gno.land/r/gnoland/home"},
		{"https://rpc.test11.testnets.gno.land:443", "gno.land/r/demo/boards", "post/1", "https://test11.testnets.gno.land/r/demo/boards/post/1"},
		{"http://127.0.0.1:26657", "gno.land/r/x", "", ""}, // local → not derivable
	}
	for _, tc := range cases {
		got := gnowebURLFor(tc.rpc, tc.realm, tc.path)
		assert.Equal(t, tc.want, got, "gnowebURLFor(%q,%q,%q)", tc.rpc, tc.realm, tc.path)
	}
}
