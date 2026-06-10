package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func srcDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}

func TestAgentFaucet_helpListsFlags(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "-help")
	cmd.Dir = srcDir()
	out, _ := cmd.CombinedOutput() // -help exits non-zero; ignore err
	for _, flag := range []string{"-rpc-url", "-chain-id", "-mnemonic", "-listen", "-grant"} {
		assert.Contains(t, string(out), flag, "usage should list %s", flag)
	}
}

func TestAgentFaucet_helpDoesNotLeakMnemonic(t *testing.T) {
	const sentinel = "zzz-secret-funding-mnemonic-sentinel-zzz"
	cmd := exec.Command("go", "run", ".", "-help")
	cmd.Dir = srcDir()
	cmd.Env = append(os.Environ(), "GNOMCP_FAUCET_MNEMONIC="+sentinel)
	out, _ := cmd.CombinedOutput() // -help exits non-zero; ignore err
	assert.NotContains(t, string(out), sentinel,
		"usage output must never print the funding mnemonic (flag default leak)")
}
