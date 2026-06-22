package read

import (
	"strings"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/budget"
	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBudgetResourceBody_TruncatesLarge(t *testing.T) {
	big := strings.Repeat("x", 5000) // > DefaultBudget (4096)
	body, truncated := budgetBody(big, "https://test13.testnets.gno.land/r/foo", budget.DefaultBudget)
	require.True(t, truncated, "expected truncation for >4KB body")
	assert.NotEqual(t, big, body, "body should have been replaced by a summary")
}

func TestBudgetResourceBody_KeepsSmall(t *testing.T) {
	body, truncated := budgetBody("small", "https://x", budget.DefaultBudget)
	assert.False(t, truncated, "small body should not be truncated")
	assert.Equal(t, "small", body, "small body should pass through unchanged")
}

func TestGnowebURLFor(t *testing.T) {
	cases := []struct {
		name    string
		profile profiles.Profile
		realm   string
		path    string
		want    string
	}{
		{"derived from rpc", profiles.Profile{RPCURL: "https://rpc.test13.testnets.gno.land:443"}, "gno.land/r/gnoland/home", "", "https://test13.testnets.gno.land/r/gnoland/home"},
		{"derived with sub-path", profiles.Profile{RPCURL: "https://rpc.test13.testnets.gno.land:443"}, "gno.land/r/demo/boards", "post/1", "https://test13.testnets.gno.land/r/demo/boards/post/1"},
		{"local not derivable", profiles.Profile{RPCURL: "http://127.0.0.1:26657"}, "gno.land/r/x", "", ""},
		{"configured gnoweb-url honored", profiles.Profile{GnowebURL: "https://custom.example.com", RPCURL: "https://rpc.test13.testnets.gno.land:443"}, "gno.land/r/x", "", "https://custom.example.com/r/x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, gnowebURLFor(tc.profile, tc.realm, tc.path))
		})
	}
}
