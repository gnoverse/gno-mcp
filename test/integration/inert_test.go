//go:build integration

package integration_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/gnolang/gno/gno.land/pkg/gnoland"
	"github.com/gnolang/gno/gno.land/pkg/integration"
	"github.com/gnolang/gno/gno.land/pkg/sdk/vm"
	"github.com/gnolang/gno/tm2/pkg/crypto"
	"github.com/gnolang/gno/tm2/pkg/std"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/chain"
)

// newInertNode boots a node running the inert code-submission policy, with
// test1 as the only package approver and nobody enabling anything.
func newInertNode(t *testing.T) *chain.Real {
	t.Helper()
	cfg := minimalNodeConfig()
	state := cfg.Genesis.AppState.(gnoland.GnoGenesisState)
	state.VM.Params.CodeSubmissionPolicy = vm.CodeSubmissionPolicyInert
	state.VM.Params.PkgApprovers = []crypto.Address{crypto.MustAddressFromString(integration.DefaultAccount_Address)}
	cfg.Genesis.AppState = state

	node, remoteAddr := integration.TestingInMemoryNode(t, slog.Default(), cfg)
	t.Cleanup(func() { _ = node.Stop() })
	c, err := chain.NewReal(remoteAddr, nodeChainID)
	require.NoError(t, err)
	return c
}

func TestIntegration_inertDeployParks(t *testing.T) {
	c := newInertNode(t)
	ctx := context.Background()

	policy, err := c.SubmissionPolicy(ctx)
	require.NoError(t, err)
	assert.Equal(t, chain.SubmissionPolicyInert, policy)

	runners, err := c.RunSubmitters(ctx)
	require.NoError(t, err)
	assert.Empty(t, runners, "no run_submitters in genesis: the allowlist is off")

	const path = "gno.land/r/test/parkme"
	files := []*std.MemFile{
		{Name: "gnomod.toml", Body: "module = \"" + path + "\"\ngno = \"0.9\"\n"},
		{Name: "parkme.gno", Body: "package parkme\n\nfunc Render(_ string) string { return \"parked\" }\n"},
	}
	_, err = c.AddPackage(ctx, test1Signer(t), path, files, false)
	require.NoError(t, err, "an inert chain accepts the deploy and parks it")

	meta, err := c.PackageMeta(ctx, path)
	require.NoError(t, err)
	assert.Equal(t, chain.PackageInert, meta.Status)
	assert.True(t, meta.Pending)
	assert.NotEmpty(t, meta.Reason, "the chain says why the package is not live")
	assert.False(t, meta.Live())

	_, err = c.Render(ctx, path, "")
	require.Error(t, err, "a parked package answers reads like an absent one")

	gone, err := c.PackageMeta(ctx, "gno.land/r/test/never")
	require.NoError(t, err)
	assert.Equal(t, chain.PackageAbsent, gone.Status)
}

func TestIntegration_permissionlessPolicyAndLivePackage(t *testing.T) {
	c := newNodeBackedReal(t)
	ctx := context.Background()

	policy, err := c.SubmissionPolicy(ctx)
	require.NoError(t, err)
	assert.NotEqual(t, chain.SubmissionPolicyInert, policy)

	meta, err := c.PackageMeta(ctx, "gno.land/r/test/counter")
	require.NoError(t, err)
	assert.True(t, meta.Live(), "a genesis realm is live, got %+v", meta)
}
