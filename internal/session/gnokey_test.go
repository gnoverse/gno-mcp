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
