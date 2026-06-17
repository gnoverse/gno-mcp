package session

import (
	"strings"
	"testing"
	"time"

	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testGnokeyProfile() *profiles.Profile {
	return &profiles.Profile{
		RPCURL:  "http://127.0.0.1:26657",
		ChainID: "dev",
	}
}

func testGnokeyScope(paths []string) Scope {
	return Scope{
		SpendLimit: "1000000ugnot",
		ExpiresIn:  time.Hour,
		AllowPaths: paths,
	}
}

func TestFormatCreate_includesAllExpectedFlags(t *testing.T) {
	profile := testGnokeyProfile()
	scope := testGnokeyScope([]string{"gno.land/r/test/blog"})
	cmd := FormatGnokeyCreateCommand(profile, "gpub1abcdeftest", scope)

	checks := []string{
		"gnokey maketx session create",
		"--pubkey gpub1",
		"--allow-paths vm/exec:gno.land/r/test/blog",
		"--spend-limit 1000000ugnot",
		"--remote http://",
		"--chainid dev",
		"<your-master-key-name>",
	}
	for _, want := range checks {
		assert.Contains(t, cmd, want, "FormatGnokeyCreateCommand output missing %q", want)
	}
}

func TestFormatCreate_multipleAllowPaths(t *testing.T) {
	profile := testGnokeyProfile()
	paths := []string{
		"gno.land/r/test/blog",
		"gno.land/r/test/counter",
		"gno.land/r/test/other",
	}
	scope := testGnokeyScope(paths)
	cmd := FormatGnokeyCreateCommand(profile, "gpub1test", scope)

	for _, p := range paths {
		assert.Contains(t, cmd, p, "command missing path %q", p)
	}
	count := strings.Count(cmd, "--allow-paths")
	assert.Equal(t, len(paths), count, "--allow-paths should appear once per path")
}

func TestFormatCreate_allowRunAppendsVMRun(t *testing.T) {
	profile := testGnokeyProfile()
	scope := testGnokeyScope([]string{"gno.land/r/test/blog"})
	scope.AllowRun = true
	cmd := FormatGnokeyCreateCommand(profile, "gpub1test", scope)

	assert.Contains(t, cmd, "--allow-paths vm/run", "expected --allow-paths vm/run line when AllowRun=true")
	// vm/exec entries should come before vm/run.
	execIdx := strings.Index(cmd, "--allow-paths vm/exec:")
	runIdx := strings.Index(cmd, "--allow-paths vm/run")
	require.True(t, execIdx >= 0 && runIdx >= 0 && execIdx < runIdx,
		"vm/exec must come before vm/run; execIdx=%d runIdx=%d", execIdx, runIdx)
}

func TestFormatCreate_allowRunOnly(t *testing.T) {
	profile := testGnokeyProfile()
	scope := Scope{
		SpendLimit: "1000000ugnot",
		ExpiresIn:  time.Hour,
		AllowRun:   true,
	}
	cmd := FormatGnokeyCreateCommand(profile, "gpub1test", scope)

	assert.Contains(t, cmd, "--allow-paths vm/run", "expected --allow-paths vm/run")
	assert.NotContains(t, cmd, "vm/exec:", "unexpected vm/exec entry when AllowPaths empty")
}

func TestFormatCreate_noAllowRunOmitsVMRun(t *testing.T) {
	profile := testGnokeyProfile()
	scope := testGnokeyScope([]string{"gno.land/r/test/blog"})
	cmd := FormatGnokeyCreateCommand(profile, "gpub1test", scope)

	assert.NotContains(t, cmd, "vm/run", "unexpected vm/run entry when AllowRun=false")
}

func TestFormatGnokeyCreate_HasGasAndBroadcast(t *testing.T) {
	p := &profiles.Profile{RPCURL: "https://rpc.test13.testnets.gno.land:443", ChainID: "test-13"}
	cmd := FormatGnokeyCreateCommand(p, "gpub...", Scope{SpendLimit: "1000ugnot", ExpiresIn: time.Hour})
	for _, want := range []string{"--gas-fee", "--gas-wanted", "--broadcast"} {
		assert.Contains(t, cmd, want, "create command missing %q", want)
	}
}

func TestFormatRevoke_includesPubkey(t *testing.T) {
	profile := testGnokeyProfile()
	cmd := FormatGnokeyRevokeCommand(profile, "gpub1abcdef")

	checks := []string{
		"gnokey maketx session revoke",
		"--pubkey gpub1abc",
	}
	for _, want := range checks {
		assert.Contains(t, cmd, want, "FormatGnokeyRevokeCommand output missing %q", want)
	}
}
