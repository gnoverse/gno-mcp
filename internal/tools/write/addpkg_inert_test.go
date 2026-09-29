package write

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/audit"
	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/keystore"
	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/gnoverse/gno-mcp/internal/server"
	"github.com/gnoverse/gno-mcp/internal/tools/parked"
)

const inertPath = "gno.land/r/test/tally"

var (
	// testParkWait expires quickly, for outcomes reached only at the deadline.
	testParkWait = parkWait{timeout: 60 * time.Millisecond, interval: 5 * time.Millisecond}
	// liveWait never expires in a test: a package that goes live returns early.
	liveWait   = parkWait{timeout: time.Minute, interval: 5 * time.Millisecond}
	parkedMeta = chain.PackageMeta{Status: chain.PackageInert, Reason: "waiting for a package approver to enable it", Pending: true}
	liveMeta   = chain.PackageMeta{Status: chain.PackageLive}
)

// deployTally runs gno_addpkg for inertPath on a funded testnet agent key, with
// a gnoweb host so a View link can appear.
func deployTally(t *testing.T, fake *chain.Fake, simulate bool, wait parkWait) (server.Result, error, []audit.Entry) {
	t.Helper()
	p := profiles.Profile{RPCURL: "http://127.0.0.1:26657", ChainID: "test9999", GnowebURL: "https://test9999.gno.land"}
	s := newTestnetServerFromProfiles(t, map[string]profiles.Profile{"testnet9999": p})
	ks := keystore.New(t.TempDir(), "", 5)
	addr, err := ks.GenerateForProfile("testnet9999", "", p)
	require.NoError(t, err)
	fake.SetBalance(addr, 10_000_000)
	fake.SetAddPackage(inertPath, chain.AddPackageResult{TxHash: "0xtally", Height: 7, GasUsed: 2_600_000, Simulated: simulate})
	var auditBuf bytes.Buffer
	registerAddPkg(s, ks, constChainResolver(fake), audit.NewLog(&auditBuf), wait)

	res, err := s.Registry().Call(context.Background(), "gno_addpkg", map[string]any{
		"profile":     "testnet9999",
		"deploy_path": inertPath,
		"simulate":    simulate,
		"files":       []any{map[string]any{"name": "tally.gno", "body": "package tally\n"}},
	})
	return res, err, parseAuditEntries(t, &auditBuf)
}

// A chain that runs what it accepts parks nothing, so a landed deploy is live.
func TestAddPkg_nonInertChainReportsLive(t *testing.T) {
	fake := chain.NewFake()
	fake.SetSubmissionPolicy("permissionless")

	res, err, entries := deployTally(t, fake, false, liveWait)

	require.NoError(t, err)
	assert.Contains(t, res.Text, "AddPackage succeeded")
	assert.Contains(t, res.Text, "Package: live")
	assert.NotContains(t, res.Text, "package approver", "no approver enabled it")
	assert.Equal(t, chain.PackageLive, res.StructuredContent["package_status"])
	assert.NotContains(t, res.StructuredContent, "code_submission_policy")
	assert.NotContains(t, res.StructuredContent, "next_steps")
	assert.Zero(t, fake.PackageMetaCalls(inertPath), "a chain that runs what it accepts needs no status poll")
	assert.Equal(t, "ok", entries[0].Result)
}

func TestAddPkg_inertReportsLiveOnceEnabled(t *testing.T) {
	fake := chain.NewFake()
	fake.SetSubmissionPolicy(chain.SubmissionPolicyInert)
	fake.SetPackageMetaSequence(inertPath, parkedMeta, parkedMeta, liveMeta)

	res, err, entries := deployTally(t, fake, false, liveWait)

	require.NoError(t, err)
	assert.Contains(t, res.Text, "AddPackage succeeded")
	assert.Contains(t, res.Text, "https://test9999.gno.land/r/test/tally", "a live realm gets its view link")
	assert.Equal(t, chain.PackageLive, res.StructuredContent["package_status"])
	assert.Equal(t, chain.SubmissionPolicyInert, res.StructuredContent["code_submission_policy"])
	assert.NotContains(t, res.StructuredContent, "next_steps", "a live package needs no recovery")
	assert.Equal(t, "ok", entries[0].Result)
}

func TestAddPkg_inertReportsParkedWhenNotEnabledInTime(t *testing.T) {
	fake := chain.NewFake()
	fake.SetSubmissionPolicy(chain.SubmissionPolicyInert)
	fake.SetPackageMetaSequence(inertPath, parkedMeta)

	res, err, entries := deployTally(t, fake, false, testParkWait)

	require.NoError(t, err, "the deploy landed; being parked is its state, not a failure of the call")
	assert.Contains(t, res.Text, "PARKED")
	assert.NotContains(t, res.Text, "AddPackage succeeded")
	assert.Contains(t, res.Text, "0xtally", "the tx that parked it stays traceable")
	assert.Contains(t, res.Text, `<untrusted_content kind="package_reason" source="`+inertPath+`">`,
		"the reason is chain-authored text")
	assert.Contains(t, res.Text, parkedMeta.Reason)
	assert.Contains(t, res.Text, parked.NextSteps)
	assert.NotContains(t, res.Text, "https://test9999.gno.land/r/test/tally", "no view link to a page that is not there")
	assert.NotContains(t, res.StructuredContent, "gnoweb_url")
	assert.Equal(t, chain.PackageInert, res.StructuredContent["package_status"])
	assert.Equal(t, parkedMeta.Reason, res.StructuredContent["package_reason"])
	// A client may hand the model the structured content alone, so the
	// recovery cannot live only in the text.
	assert.Contains(t, res.StructuredContent["next_steps"], parked.NextSteps)
	assert.Equal(t, "parked", entries[0].Result)
}

func TestAddPkg_inertStatusUnknownIsNeverReportedLive(t *testing.T) {
	fake := chain.NewFake()
	fake.SetSubmissionPolicy(chain.SubmissionPolicyInert)
	fake.SetPackageMetaErr(inertPath, errors.New("rpc down"))

	res, err, entries := deployTally(t, fake, false, testParkWait)

	require.NoError(t, err)
	assert.NotContains(t, res.Text, "AddPackage succeeded")
	assert.Contains(t, res.Text, "status unknown")
	assert.NotContains(t, res.StructuredContent, "gnoweb_url")
	assert.Equal(t, "unknown", res.StructuredContent["package_status"])
	assert.Contains(t, res.StructuredContent["next_steps"], "gno_read")
	assert.Equal(t, "status_unknown", entries[0].Result)
}

// A redeploy parked over a live private realm leaves the previous version
// serving reads and calls; saying the path reads as absent would be false.
func TestAddPkg_inertRedeployParkedOverLiveKeepsThePreviousVersion(t *testing.T) {
	fake := chain.NewFake()
	fake.SetSubmissionPolicy(chain.SubmissionPolicyInert)
	fake.SetPackageMetaSequence(inertPath, chain.PackageMeta{Status: chain.PackageLive, Pending: true})

	res, err, entries := deployTally(t, fake, false, testParkWait)

	require.NoError(t, err)
	assert.Contains(t, res.Text, "redeploy PARKED")
	assert.Contains(t, res.Text, "previous version")
	assert.NotContains(t, res.Text, "never deployed", "reads still reach the previous version")
	assert.NotContains(t, res.Text, "<untrusted_content", "no envelope around an empty reason")
	assert.NotContains(t, res.StructuredContent, "package_reason")
	assert.Equal(t, packageStatusRedeployParked, res.StructuredContent["package_status"])
	assert.Contains(t, res.StructuredContent["next_steps"], "previous version")
	assert.Contains(t, res.StructuredContent["next_steps"], parked.NextSteps)
	assert.Equal(t, "parked", entries[0].Result)
}

func TestAddPkg_inertRedeployParkedEnvelopesTheChainsReason(t *testing.T) {
	const reason = "a live private package at this path was deployed by a different address, so this submission can never be enabled"
	fake := chain.NewFake()
	fake.SetSubmissionPolicy(chain.SubmissionPolicyInert)
	fake.SetPackageMetaSequence(inertPath, chain.PackageMeta{Status: chain.PackageLive, Pending: true, Reason: reason})

	res, err, _ := deployTally(t, fake, false, testParkWait)

	require.NoError(t, err)
	assert.Contains(t, res.Text, `<untrusted_content kind="package_reason" source="`+inertPath+`">`)
	assert.Contains(t, res.Text, reason)
	assert.Equal(t, reason, res.StructuredContent["package_reason"])
}

// The chain answering "absent" for a deploy that landed is an answer, but not
// live or parked: it stays unknown and is never reported live.
func TestAddPkg_inertAbsentAfterTheDeployIsUnknown(t *testing.T) {
	fake := chain.NewFake()
	fake.SetSubmissionPolicy(chain.SubmissionPolicyInert)

	res, err, _ := deployTally(t, fake, false, testParkWait)

	require.NoError(t, err)
	assert.Contains(t, res.Text, "did not report the package as live or parked")
	assert.Equal(t, packageStatusUnknown, res.StructuredContent["package_status"])
}

// An inert chain parks what it is sent without type-checking it, so a clean
// dry run says nothing about the code.
func TestAddPkg_inertSimulateSaysTheCodeWasNotTypeChecked(t *testing.T) {
	fake := chain.NewFake()
	fake.SetSubmissionPolicy(chain.SubmissionPolicyInert)

	res, err, _ := deployTally(t, fake, true, liveWait)

	require.NoError(t, err)
	assert.Contains(t, res.Text, "did not type-check")
	assert.Equal(t, chain.SubmissionPolicyInert, res.StructuredContent["code_submission_policy"])
	assert.Contains(t, res.StructuredContent["next_steps"], "did not type-check")
	assert.NotContains(t, res.StructuredContent, "package_status", "nothing was deployed")
	assert.Zero(t, fake.PackageMetaCalls(inertPath), "nothing was deployed, so nothing to poll")
}

func TestAddPkg_policyReadFailureStopsBeforeSigning(t *testing.T) {
	fake := chain.NewFake()
	fake.SetSubmissionPolicyErr(errors.New("rpc down"))

	_, err, entries := deployTally(t, fake, false, liveWait)

	require.Error(t, err)
	assert.Nil(t, fake.LastAddPackageFiles(inertPath), "nothing may be simulated or broadcast")
	assert.Equal(t, "tool_err", entries[0].Result)
}

// ---- waitLive

func TestWaitLive_stopsAtLive(t *testing.T) {
	fake := chain.NewFake()
	fake.SetPackageMetaSequence(inertPath, parkedMeta, liveMeta)

	got, err := waitLive(context.Background(), fake, inertPath, testParkWait)

	require.NoError(t, err)
	assert.True(t, got.Live())
	assert.Equal(t, 2, fake.PackageMetaCalls(inertPath))
}

// shippedParkWait is the wait gno_addpkg runs with; the tests using it run in a
// synctest bubble, whose clock advances only when every goroutine is blocked.
var shippedParkWait = parkWait{timeout: parkWaitTimeout, interval: parkPollInterval}

// The tool description promises "up to 30s": the last poll lands on the
// deadline, not an interval past it.
func TestWaitLive_stopsAtTheDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fake := chain.NewFake()
		fake.SetPackageMetaSequence(inertPath, parkedMeta)
		start := time.Now()

		got, err := waitLive(context.Background(), fake, inertPath, shippedParkWait)

		require.NoError(t, err)
		assert.Equal(t, parkedMeta, got)
		assert.Equal(t, parkWaitTimeout, time.Since(start))
		assert.Equal(t, 16, fake.PackageMetaCalls(inertPath), "a poll every 2s from 0s to 30s")
	})
}

// A redeploy parked over a live private realm leaves the old code callable;
// the new code is not live until the submission is enabled.
func TestWaitLive_pendingOverLiveIsNotLive(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fake := chain.NewFake()
		fake.SetPackageMetaSequence(inertPath, chain.PackageMeta{Status: chain.PackageLive, Pending: true, Reason: "x"})

		got, err := waitLive(context.Background(), fake, inertPath, shippedParkWait)

		require.NoError(t, err)
		assert.False(t, got.Live())
		assert.Equal(t, 16, fake.PackageMetaCalls(inertPath), "kept waiting to the deadline")
	})
}

// A flaky RPC after one answer must not turn "parked" into "unknown".
func TestWaitLive_reportsTheLastAnswerAfterALaterError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fake := chain.NewFake()
		fake.SetPackageMetaSequence(inertPath, parkedMeta)
		fake.FailPackageMetaFrom(inertPath, 1, errors.New("rpc down"))

		got, err := waitLive(context.Background(), fake, inertPath, shippedParkWait)

		require.NoError(t, err)
		assert.Equal(t, parkedMeta, got)
		assert.Equal(t, 16, fake.PackageMetaCalls(inertPath), "kept polling through the errors")
	})
}

func TestWaitLive_errorsOnlyWhenTheChainNeverAnswered(t *testing.T) {
	fake := chain.NewFake()
	fake.SetPackageMetaErr(inertPath, errors.New("rpc down"))

	_, err := waitLive(context.Background(), fake, inertPath, testParkWait)

	require.Error(t, err)
}

func TestWaitLive_honoursCancellation(t *testing.T) {
	fake := chain.NewFake()
	fake.SetPackageMetaSequence(inertPath, parkedMeta)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	got, err := waitLive(ctx, fake, inertPath, parkWait{timeout: time.Minute, interval: time.Second})

	require.NoError(t, err)
	assert.Equal(t, parkedMeta, got)
	assert.Less(t, time.Since(start), time.Second)
}
