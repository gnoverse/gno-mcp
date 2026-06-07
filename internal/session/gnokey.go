package session

import (
	"fmt"
	"strings"
	"time"

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
	sb.WriteString(fmt.Sprintf("  --pubkey %s \\\n", sessionPubkey))
	for _, p := range scope.AllowPaths {
		sb.WriteString(fmt.Sprintf("  --allow-paths vm/exec:%s \\\n", p))
	}
	if scope.AllowRun {
		sb.WriteString("  --allow-paths vm/run \\\n")
	}
	sb.WriteString(fmt.Sprintf("  --spend-limit %s \\\n", scope.SpendLimit))
	expiresAt := time.Now().Add(scope.ExpiresIn).Unix()
	sb.WriteString(fmt.Sprintf("  --expires-at %d \\\n", expiresAt))
	sb.WriteString(fmt.Sprintf("  --gas-fee %s \\\n", defaultGnokeyGasFee))
	sb.WriteString(fmt.Sprintf("  --gas-wanted %d \\\n", defaultGnokeyGasWanted))
	sb.WriteString(fmt.Sprintf("  --remote %s \\\n", profile.RPCURL))
	sb.WriteString(fmt.Sprintf("  --chainid %s \\\n", profile.ChainID))
	sb.WriteString("  --broadcast \\\n")
	sb.WriteString("  <your-master-key-name>")
	return sb.String()
}

// Defaults baked into the emitted gnokey commands so the user can paste and run
// without hunting for flag values. Match chain.Real's BaseTxCfg so gnokey
// broadcasts behave the same as gnomcp's session-signed ones.
const (
	defaultGnokeyGasFee    = "10000000ugnot"
	defaultGnokeyGasWanted = 10_000_000
)

// FormatGnokeyRevokeCommand returns the gnokey shell command the user must run
// to revoke a session key on chain.
func FormatGnokeyRevokeCommand(profile *profiles.Profile, sessionPubkey string) string {
	var sb strings.Builder
	sb.WriteString("gnokey maketx session revoke \\\n")
	sb.WriteString(fmt.Sprintf("  --pubkey %s \\\n", sessionPubkey))
	sb.WriteString(fmt.Sprintf("  --gas-fee %s \\\n", defaultGnokeyGasFee))
	sb.WriteString(fmt.Sprintf("  --gas-wanted %d \\\n", defaultGnokeyGasWanted))
	sb.WriteString(fmt.Sprintf("  --remote %s \\\n", profile.RPCURL))
	sb.WriteString(fmt.Sprintf("  --chainid %s \\\n", profile.ChainID))
	sb.WriteString("  --broadcast \\\n")
	sb.WriteString("  <your-master-key-name>")
	return sb.String()
}
