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
	sb.WriteString(fmt.Sprintf("  --spend-limit %s \\\n", scope.SpendLimit))
	expiresAt := time.Now().Add(scope.ExpiresIn).Unix()
	sb.WriteString(fmt.Sprintf("  --expires-at %d \\\n", expiresAt))
	sb.WriteString(fmt.Sprintf("  --remote %s \\\n", profile.RPCURL))
	sb.WriteString(fmt.Sprintf("  --chainid %s \\\n", profile.ChainID))
	sb.WriteString("  <your-master-key-name>")
	return sb.String()
}

// FormatGnokeyRevokeCommand returns the gnokey shell command the user must run
// to revoke a session key on chain.
func FormatGnokeyRevokeCommand(profile *profiles.Profile, sessionPubkey string) string {
	var sb strings.Builder
	sb.WriteString("gnokey maketx session revoke \\\n")
	sb.WriteString(fmt.Sprintf("  --pubkey %s \\\n", sessionPubkey))
	sb.WriteString(fmt.Sprintf("  --remote %s \\\n", profile.RPCURL))
	sb.WriteString(fmt.Sprintf("  --chainid %s \\\n", profile.ChainID))
	sb.WriteString("  <your-master-key-name>")
	return sb.String()
}
