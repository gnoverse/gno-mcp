package session

import (
	"strings"
	"testing"
	"time"

	"github.com/gnoverse/gno-mcp/internal/profiles"
)

func testGnokeyProfile() *profiles.Profile {
	return &profiles.Profile{
		ChainType: "local",
		RPCURL:    "http://127.0.0.1:26657",
		ChainID:   "dev",
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
		if !strings.Contains(cmd, want) {
			t.Errorf("FormatGnokeyCreateCommand output missing %q\nfull output:\n%s", want, cmd)
		}
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
		if !strings.Contains(cmd, p) {
			t.Errorf("command missing path %q\nfull output:\n%s", p, cmd)
		}
	}
	count := strings.Count(cmd, "--allow-paths")
	if count != len(paths) {
		t.Errorf("--allow-paths appears %d times, want %d (one per path)", count, len(paths))
	}
}

func TestFormatCreate_allowRunAppendsVMRun(t *testing.T) {
	profile := testGnokeyProfile()
	scope := testGnokeyScope([]string{"gno.land/r/test/blog"})
	scope.AllowRun = true
	cmd := FormatGnokeyCreateCommand(profile, "gpub1test", scope)

	if !strings.Contains(cmd, "--allow-paths vm/run") {
		t.Errorf("expected --allow-paths vm/run line when AllowRun=true:\n%s", cmd)
	}
	// vm/exec entries should come before vm/run.
	execIdx := strings.Index(cmd, "--allow-paths vm/exec:")
	runIdx := strings.Index(cmd, "--allow-paths vm/run")
	if execIdx < 0 || runIdx < 0 || execIdx > runIdx {
		t.Errorf("vm/exec must come before vm/run; execIdx=%d runIdx=%d", execIdx, runIdx)
	}
}

func TestFormatCreate_allowRunOnly(t *testing.T) {
	profile := testGnokeyProfile()
	scope := Scope{
		SpendLimit: "1000000ugnot",
		ExpiresIn:  time.Hour,
		AllowRun:   true,
	}
	cmd := FormatGnokeyCreateCommand(profile, "gpub1test", scope)

	if !strings.Contains(cmd, "--allow-paths vm/run") {
		t.Errorf("expected --allow-paths vm/run:\n%s", cmd)
	}
	if strings.Contains(cmd, "vm/exec:") {
		t.Errorf("unexpected vm/exec entry when AllowPaths empty:\n%s", cmd)
	}
}

func TestFormatCreate_noAllowRunOmitsVMRun(t *testing.T) {
	profile := testGnokeyProfile()
	scope := testGnokeyScope([]string{"gno.land/r/test/blog"})
	cmd := FormatGnokeyCreateCommand(profile, "gpub1test", scope)

	if strings.Contains(cmd, "vm/run") {
		t.Errorf("unexpected vm/run entry when AllowRun=false:\n%s", cmd)
	}
}

func TestFormatGnokeyCreate_HasGasAndBroadcast(t *testing.T) {
	p := &profiles.Profile{RPCURL: "https://rpc.test11.testnets.gno.land:443", ChainID: "test11"}
	cmd := FormatGnokeyCreateCommand(p, "gpub...", Scope{SpendLimit: "1000ugnot", ExpiresIn: time.Hour})
	for _, want := range []string{"--gas-fee", "--gas-wanted", "--broadcast"} {
		if !strings.Contains(cmd, want) {
			t.Errorf("create command missing %q:\n%s", want, cmd)
		}
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
		if !strings.Contains(cmd, want) {
			t.Errorf("FormatGnokeyRevokeCommand output missing %q\nfull output:\n%s", want, cmd)
		}
	}
}
