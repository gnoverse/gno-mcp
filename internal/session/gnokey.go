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
func FormatGnokeyCreateCommand(profile *profiles.Profile, sessionPubkey string, scope Scope) string {
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
	fmt.Fprintf(&sb, "  --gas-fee %s \\\n", defaultGnokeyGasFee)
	fmt.Fprintf(&sb, "  --gas-wanted %d \\\n", defaultGnokeyGasWanted)
	fmt.Fprintf(&sb, "  --remote %s \\\n", profile.RPCURL)
	fmt.Fprintf(&sb, "  --chainid %s \\\n", profile.ChainID)
	sb.WriteString("  --broadcast \\\n")
	sb.WriteString("  <your-master-key-name>")
	return sb.String()
}

// Defaults baked into the emitted gnokey commands so the user can paste and run
// without hunting for flag values. Sourced from chain.Real's BaseTxCfg constants
// so gnokey broadcasts behave the same as gnomcp's session-signed ones — drift
// here would desync session spend accounting.
var (
	defaultGnokeyGasFee    = fmt.Sprintf("%dugnot", chain.DefaultGasFeeUgnot)
	defaultGnokeyGasWanted = chain.DefaultGasWanted
)

// FormatGnokeyRevokeCommand returns the gnokey shell command the user must run
// to revoke a session key on chain.
func FormatGnokeyRevokeCommand(profile *profiles.Profile, sessionPubkey string) string {
	var sb strings.Builder
	sb.WriteString("gnokey maketx session revoke \\\n")
	fmt.Fprintf(&sb, "  --pubkey %s \\\n", sessionPubkey)
	fmt.Fprintf(&sb, "  --gas-fee %s \\\n", defaultGnokeyGasFee)
	fmt.Fprintf(&sb, "  --gas-wanted %d \\\n", defaultGnokeyGasWanted)
	fmt.Fprintf(&sb, "  --remote %s \\\n", profile.RPCURL)
	fmt.Fprintf(&sb, "  --chainid %s \\\n", profile.ChainID)
	sb.WriteString("  --broadcast \\\n")
	sb.WriteString("  <your-master-key-name>")
	return sb.String()
}
