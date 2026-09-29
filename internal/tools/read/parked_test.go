package read

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/server"
	"github.com/gnoverse/gno-mcp/internal/tools/parked"
)

const parkedRealm = "gno.land/r/g1x/tally"

// On an inert chain every read answers a parked package like one never
// deployed; each path-taking read tool must say which of the two it hit.
func TestReadTools_reportAParkedPackage(t *testing.T) {
	calls := map[string]struct {
		register func(*server.Server, chain.Resolver)
		tool     string
		args     map[string]any
	}{
		"render":       {RegisterRender, "gno_render", map[string]any{"realm": parkedRealm}},
		"eval":         {RegisterEval, "gno_eval", map[string]any{"path": parkedRealm, "expr": "Render(\"\")"}},
		"read-outline": {RegisterRead, "gno_read", map[string]any{"path": parkedRealm}},
		"read-full":    {RegisterRead, "gno_read", map[string]any{"path": parkedRealm, "full": true}},
		"read-file":    {RegisterRead, "gno_read", map[string]any{"path": parkedRealm, "file": "tally.gno"}},
	}
	for name, tc := range calls {
		t.Run(name, func(t *testing.T) {
			f := chain.NewFake()
			f.SetPackageMetaSequence(parkedRealm, chain.PackageMeta{Status: chain.PackageInert, Reason: "waiting for a package approver to enable it", Pending: true})
			s := newBaseTestServer(t)
			tc.register(s, constResolver(f))
			tc.args["profile"] = "testnet5"

			_, err := s.Registry().Call(context.Background(), tc.tool, tc.args)

			te, ok := errors.AsType[*server.ToolError](err)
			require.True(t, ok, "want a ToolError, got %T: %v", err, err)
			assert.Equal(t, parked.Code, te.Code)
			assert.Equal(t, parkedRealm, te.Extra["path"])
		})
	}
}

func TestReadTools_absentPackageIsNotReportedParked(t *testing.T) {
	f := chain.NewFake()
	s := newBaseTestServer(t)
	RegisterRender(s, constResolver(f))

	_, err := s.Registry().Call(context.Background(), "gno_render", map[string]any{"realm": parkedRealm, "profile": "testnet5"})

	require.Error(t, err)
	_, typed := errors.AsType[*server.ToolError](err)
	assert.False(t, typed, "an absent path keeps the chain's plain error: %v", err)
}
