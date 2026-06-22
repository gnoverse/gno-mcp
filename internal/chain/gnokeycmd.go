package chain

import (
	"fmt"
	"strings"
)

// GnokeyCmd describes a gnomcp write as the `gnokey maketx` command that would
// build the same transaction. String() renders an *illustrative* command: it
// shows, transparently, the tx gnomcp put on the wire (pkgpath, func, args,
// gas) — it is NOT meant to be run instead of the gnomcp write. The signing
// key lives in gnomcp's keystore (~/.gnomcp/keys/), not gnokey's, so the
// command won't sign as-is; it is for understanding and display. The -gas-fee
// mirrors the fee gnomcp offered (GasFeeUgnot; the chain's live price, floored
// at DefaultGasFeeUgnot), and -gas-wanted is DefaultGasWanted.
//
// The string names no skill or MCP detail beyond the gnokey CLI itself, so it
// is meaningful to any consumer of a write result, with or without the gno
// skill loaded.
type GnokeyCmd struct {
	Sub        string   // "call" | "run" | "addpkg" | "send"
	PkgPath    string   // call, addpkg
	Func       string   // call
	Args       []string // call
	To         string   // send: recipient address
	Send       string   // call/run/addpkg: attached coins; send: the amount (the -send value)
	PkgDir     string   // addpkg: local source dir (placeholder — gnomcp uploads inline)
	MaxDeposit string   // addpkg (and call/run when set)
	RPC        string   // -remote
	ChainID    string   // -chainid
	Signer     string   // the signing address — the trailing key positional (agent addr, or session addr)
	Master     string   // session master address; non-empty → -master <Master> (session-signed tx)
	Simulate   bool     // dry-run → -simulate only instead of -broadcast

	// GasFeeUgnot is the -gas-fee the echoed command shows; it should match the
	// fee gnomcp offered (the chain's live price). Zero falls back to the floor.
	GasFeeUgnot int64
}

// String renders the equivalent gnokey command on a single line.
func (g GnokeyCmd) String() string {
	parts := []string{"gnokey", "maketx", g.Sub}

	switch g.Sub {
	case "call":
		parts = append(parts, "-pkgpath", g.PkgPath, "-func", g.Func)
		for _, a := range g.Args {
			parts = append(parts, "-args", a)
		}
	case "addpkg":
		pkgdir := g.PkgDir
		if pkgdir == "" {
			pkgdir = "<your-source-dir>" // gnomcp uploads files inline; gnokey needs a dir
		}
		parts = append(parts, "-pkgpath", g.PkgPath, "-pkgdir", pkgdir)
	}

	if g.Send != "" {
		parts = append(parts, "-send", g.Send)
	}
	if g.Sub == "send" && g.To != "" {
		parts = append(parts, "-to", g.To)
	}
	if g.MaxDeposit != "" {
		parts = append(parts, "-max-deposit", g.MaxDeposit)
	}

	gasFee := g.GasFeeUgnot
	if gasFee == 0 {
		gasFee = DefaultGasFeeUgnot
	}
	parts = append(parts,
		"-gas-fee", fmt.Sprintf("%dugnot", gasFee),
		"-gas-wanted", fmt.Sprintf("%d", DefaultGasWanted),
		"-remote", g.RPC,
		"-chainid", g.ChainID,
	)

	if g.Simulate {
		parts = append(parts, "-simulate", "only")
	} else {
		parts = append(parts, "-broadcast")
	}

	// Session-signed txs name the authorizing master account; the trailing
	// positional is always the signing address (gnokey accepts a bech32 there).
	if g.Master != "" {
		parts = append(parts, "-master", g.Master)
	}
	parts = append(parts, g.Signer)
	if g.Sub == "run" {
		parts = append(parts, "<your-script.gno>") // gnomcp runs inline code; gnokey needs a file
	}

	return strings.Join(parts, " ")
}
