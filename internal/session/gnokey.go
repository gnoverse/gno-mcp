package session

import (
	"fmt"
	"strings"
	"time"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/profiles"
)

// FormatGnokeyCreateCommand returns the gnokey shell command the user must run
// to authorize a session key on chain.
//
// Each entry in scope.AllowPaths (a bare realm path like "gno.land/r/foo") is
// emitted as "--allow-paths vm/exec:<realm>" — the chain's session permission
// format for MsgCall is "<route>/<type>:<resource>". Multiple --allow-paths
// flags accumulate into a slice in gnokey's flag parser.
// feeUgnot is the chain's live per-write GasFee (chain.Client.GasFeeUgnot) to
// stamp as --gas-fee; <=0 falls back to the floor (a chain priced above the
// genesis floor rejects the floor fee, so callers should pass the live value).
func FormatGnokeyCreateCommand(profile *profiles.Profile, sessionPubkey string, scope Scope, feeUgnot int64) string {
	var sb strings.Builder
	sb.WriteString("gnokey maketx session create \\\n")
	fmt.Fprintf(&sb, "  --pubkey %s \\\n", sessionPubkey)
	for _, p := range scope.AllowPaths {
		fmt.Fprintf(&sb, "  --allow-paths vm/exec:%s \\\n", p)
	}
	if scope.AllowRun {
		sb.WriteString("  --allow-paths vm/run \\\n")
	}
	fmt.Fprintf(&sb, "  --spend-limit %s \\\n", scope.SpendLimit)
	expiresAt := time.Now().Add(scope.ExpiresIn).Unix()
	fmt.Fprintf(&sb, "  --expires-at %d \\\n", expiresAt)
	gasFee := defaultGnokeyGasFee
	if feeUgnot > 0 {
		gasFee = fmt.Sprintf("%dugnot", feeUgnot)
	}
	fmt.Fprintf(&sb, "  --gas-fee %s \\\n", gasFee)
	fmt.Fprintf(&sb, "  --gas-wanted %d \\\n", defaultGnokeyGasWanted)
	fmt.Fprintf(&sb, "  --remote %s \\\n", profile.RPCURL)
	fmt.Fprintf(&sb, "  --chainid %s \\\n", profile.ChainID)
	sb.WriteString("  --broadcast \\\n")
	sb.WriteString("  <your-master-key-name>")
	return sb.String()
}

// Defaults baked into the emitted session create/revoke gnokey commands so the
// user can paste and run without hunting for flag values. The --gas-fee is
// normally the chain's live price supplied by the caller; the floor here is
// the fallback when the fee is unknown, in which case the user bumps
// --gas-fee from gnokey's insufficient-fee error.
var (
	defaultGnokeyGasFee    = fmt.Sprintf("%dugnot", chain.DefaultGasFeeUgnot)
	defaultGnokeyGasWanted = chain.DefaultGasWanted
)

// FormatGnokeyRevokeCommand returns the gnokey shell command the user must run
// to revoke a session key on chain. feeUgnot is the chain's live per-write
// GasFee to stamp as --gas-fee; <=0 falls back to the floor.
func FormatGnokeyRevokeCommand(profile *profiles.Profile, sessionPubkey string, feeUgnot int64) string {
	gasFee := defaultGnokeyGasFee
	if feeUgnot > 0 {
		gasFee = fmt.Sprintf("%dugnot", feeUgnot)
	}
	var sb strings.Builder
	sb.WriteString("gnokey maketx session revoke \\\n")
	fmt.Fprintf(&sb, "  --pubkey %s \\\n", sessionPubkey)
	fmt.Fprintf(&sb, "  --gas-fee %s \\\n", gasFee)
	fmt.Fprintf(&sb, "  --gas-wanted %d \\\n", defaultGnokeyGasWanted)
	fmt.Fprintf(&sb, "  --remote %s \\\n", profile.RPCURL)
	fmt.Fprintf(&sb, "  --chainid %s \\\n", profile.ChainID)
	sb.WriteString("  --broadcast \\\n")
	sb.WriteString("  <your-master-key-name>")
	return sb.String()
}
