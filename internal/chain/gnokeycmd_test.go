package chain

import (
	"strings"
	"testing"
)

// The rendered command must carry the gas flags and target the given chain, so a
// reader sees exactly the tx gnomcp put on the wire. With no GasFeeUgnot set the
// fee falls back to the floor.
func TestGnokeyCommand_callBroadcast(t *testing.T) {
	got := GnokeyCmd{
		Sub: "call", PkgPath: "gno.land/r/demo/counter", Func: "Bump",
		Args: []string{"1"}, RPC: "http://localhost:26657",
		ChainID: "dev", Signer: "g1alice",
	}.String()

	for _, want := range []string{
		"gnokey maketx call",
		"-pkgpath gno.land/r/demo/counter",
		"-func Bump",
		"-args 1",
		"-gas-fee 10000ugnot",  // the floor fallback (DefaultGasFeeUgnot)
		"-gas-wanted 10000000", // DefaultGasWanted
		"-remote http://localhost:26657",
		"-chainid dev",
		"-broadcast g1alice",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("call command missing %q\ngot: %s", want, got)
		}
	}
	if strings.Contains(got, "-simulate") {
		t.Errorf("a broadcast command must not carry -simulate\ngot: %s", got)
	}
}

// A set GasFeeUgnot is echoed verbatim, so the displayed command matches the fee
// gnomcp actually offered (the chain's live, possibly congestion-raised price).
func TestGnokeyCommand_usesOfferedFee(t *testing.T) {
	got := GnokeyCmd{
		Sub: "call", PkgPath: "gno.land/r/demo/counter", Func: "Bump",
		RPC: "rpc", ChainID: "dev", Signer: "g1alice", GasFeeUgnot: 80_000,
	}.String()
	if !strings.Contains(got, "-gas-fee 80000ugnot") {
		t.Errorf("expected the offered fee in -gas-fee\ngot: %s", got)
	}
	if strings.Contains(got, "-gas-fee 10000ugnot") {
		t.Errorf("must not fall back to the floor when a fee is set\ngot: %s", got)
	}
}

// A set GasWanted is echoed verbatim, so a right-sized heavy tx's displayed
// command shows the gas limit gnomcp actually offered (e.g. a CLA Sign), not the
// stale default that would out-of-gas if run.
func TestGnokeyCommand_usesOfferedGasWanted(t *testing.T) {
	got := GnokeyCmd{
		Sub: "call", PkgPath: "gno.land/r/sys/cla", Func: "Sign",
		RPC: "rpc", ChainID: "dev", Signer: "g1alice", GasWanted: 22_500_000,
	}.String()
	if !strings.Contains(got, "-gas-wanted 22500000") {
		t.Errorf("expected the offered gas-wanted\ngot: %s", got)
	}
	if strings.Contains(got, "-gas-wanted 10000000") {
		t.Errorf("must not fall back to the default when gas-wanted is set\ngot: %s", got)
	}
}

func TestGnokeyCommand_callWithSend(t *testing.T) {
	got := GnokeyCmd{
		Sub: "call", PkgPath: "gno.land/r/demo/auction", Func: "Bid",
		Send: "5000000ugnot", RPC: "rpc", ChainID: "dev", Signer: "g1alice",
	}.String()
	if !strings.Contains(got, "-send 5000000ugnot") {
		t.Errorf("expected -send flag\ngot: %s", got)
	}
}

// A zero-arg call must not emit a stray empty -args.
func TestGnokeyCommand_callNoArgs(t *testing.T) {
	got := GnokeyCmd{Sub: "call", PkgPath: "gno.land/r/demo/counter", Func: "Bump", RPC: "rpc", ChainID: "dev", Signer: "g1alice"}.String()
	if strings.Contains(got, "-args") {
		t.Errorf("zero-arg call must not emit -args\ngot: %s", got)
	}
}

// The trailing positional is always the signer address — never blank, even
// though a writer may not pass an explicit key name.
func TestGnokeyCommand_signerAlwaysPresent(t *testing.T) {
	got := GnokeyCmd{Sub: "call", PkgPath: "gno.land/r/demo/counter", Func: "Bump", RPC: "rpc", ChainID: "dev", Signer: "g1xyz"}.String()
	if !strings.HasSuffix(got, "-broadcast g1xyz") {
		t.Errorf("command must end with the signer positional\ngot: %s", got)
	}
	if strings.HasSuffix(got, "-broadcast ") || strings.HasSuffix(got, "-broadcast") {
		t.Errorf("command must not end with a blank signer positional\ngot: %s", got)
	}
}

// Simulate renders the dry-run form instead of -broadcast.
func TestGnokeyCommand_simulate(t *testing.T) {
	got := GnokeyCmd{Sub: "call", PkgPath: "gno.land/r/demo/counter", Func: "Bump", RPC: "rpc", ChainID: "dev", Signer: "g1alice", Simulate: true}.String()
	if !strings.Contains(got, "-simulate only") {
		t.Errorf("simulate must render -simulate only\ngot: %s", got)
	}
	if strings.Contains(got, "-broadcast") {
		t.Errorf("simulate must not render -broadcast\ngot: %s", got)
	}
}

func TestGnokeyCommand_send(t *testing.T) {
	got := GnokeyCmd{Sub: "send", To: "g1dest", Send: "1000000ugnot", RPC: "rpc", ChainID: "dev", Signer: "g1alice"}.String()
	for _, want := range []string{"gnokey maketx send", "-send 1000000ugnot", "-to g1dest", "g1alice"} {
		if !strings.Contains(got, want) {
			t.Errorf("send command missing %q\ngot: %s", want, got)
		}
	}
}

// addpkg can't reproduce gnomcp's inline upload, so it shows a -pkgdir placeholder.
func TestGnokeyCommand_addpkg(t *testing.T) {
	got := GnokeyCmd{Sub: "addpkg", PkgPath: "gno.land/r/alice/blog", MaxDeposit: "10000000ugnot", RPC: "rpc", ChainID: "dev", Signer: "g1alice"}.String()
	for _, want := range []string{"gnokey maketx addpkg", "-pkgpath gno.land/r/alice/blog", "-pkgdir <your-source-dir>", "-max-deposit 10000000ugnot"} {
		if !strings.Contains(got, want) {
			t.Errorf("addpkg command missing %q\ngot: %s", want, got)
		}
	}
}

// run can't reproduce inline code, so it shows a source-file placeholder after the signer.
func TestGnokeyCommand_run(t *testing.T) {
	got := GnokeyCmd{Sub: "run", RPC: "rpc", ChainID: "dev", Signer: "g1alice"}.String()
	if !strings.Contains(got, "gnokey maketx run") || !strings.HasSuffix(got, "g1alice <your-script.gno>") {
		t.Errorf("run command shape wrong\ngot: %s", got)
	}
}

// A session-signed write names the master account (-master) and signs with the
// session address — both real gnokey maketx surface (verified against source).
func TestGnokeyCommand_sessionMaster(t *testing.T) {
	got := GnokeyCmd{Sub: "call", PkgPath: "gno.land/r/demo/counter", Func: "Bump", RPC: "rpc", ChainID: "dev", Signer: "g1session", Master: "g1master"}.String()
	if !strings.Contains(got, "-master g1master") {
		t.Errorf("session command must carry -master\ngot: %s", got)
	}
	if !strings.HasSuffix(got, "-master g1master g1session") {
		t.Errorf("session positional must be the session signer after -master\ngot: %s", got)
	}
}
