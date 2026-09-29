package parked

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/server"
)

const path = "gno.land/r/g1x/tally"

var readErr = errors.New("vm/qrender: invalid package path")

func TestExplain_parkedPathBecomesPackageParked(t *testing.T) {
	f := chain.NewFake()
	f.SetPackageMetaSequence(path, chain.PackageMeta{Status: chain.PackageInert, Reason: "waiting for a package approver to enable it", Pending: true})

	err := Explain(context.Background(), f, path, readErr)

	te, ok := errors.AsType[*server.ToolError](err)
	require.True(t, ok, "want a ToolError, got %T: %v", err, err)
	assert.Equal(t, Code, te.Code)
	assert.Contains(t, te.Message, path)
	assert.Contains(t, te.Message, "[untrusted chain reason] waiting for a package approver to enable it",
		"error text is neutralized, not enveloped, so the chain-relayed reason carries a label instead")
	assert.Contains(t, te.Message, "For its deployer: "+NextSteps,
		"the recovery is addressed to the deployer: a reader of someone else's package cannot redeploy it")
	assert.Contains(t, te.Message, "Original error: "+readErr.Error(), "the underlying error stays visible")
	assert.Equal(t, path, te.Extra["path"])
	assert.Equal(t, "waiting for a package approver to enable it", te.Extra["reason"])
}

func TestExplain_leavesOtherErrorsAlone(t *testing.T) {
	cases := map[string]func(f *chain.Fake){
		"absent":       func(*chain.Fake) {},
		"live":         func(f *chain.Fake) { f.SetPackageMetaSequence(path, chain.PackageMeta{Status: chain.PackageLive}) },
		"meta-unknown": func(f *chain.Fake) { f.SetPackageMetaErr(path, errors.New("rpc down")) },
		"live-pending": func(f *chain.Fake) {
			f.SetPackageMetaSequence(path, chain.PackageMeta{Status: chain.PackageLive, Pending: true, Reason: "x"})
		},
	}
	for name, seed := range cases {
		t.Run(name, func(t *testing.T) {
			f := chain.NewFake()
			seed(f)
			err := Explain(context.Background(), f, path, readErr)
			assert.Same(t, readErr, err)
		})
	}
}

// A typed error already says what went wrong; asking the chain about the path
// would spend a query to relabel it.
func TestExplain_passesToolErrorsThroughWithoutAQuery(t *testing.T) {
	f := chain.NewFake()
	f.SetPackageMetaSequence(path, chain.PackageMeta{Status: chain.PackageInert, Pending: true})
	typed := &server.ToolError{Code: "scope_mismatch", Message: "no session covers this realm"}

	err := Explain(context.Background(), f, path, typed)

	assert.Same(t, typed, err)
	assert.Zero(t, f.PackageMetaCalls(path))
}

func TestExplain_nilIsNil(t *testing.T) {
	f := chain.NewFake()
	require.NoError(t, Explain(context.Background(), f, path, nil))
	assert.Zero(t, f.PackageMetaCalls(path))
}
