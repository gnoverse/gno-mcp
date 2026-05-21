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
// Multiple allow_paths entries each get their own --allow-paths flag. gnokey
// flag parsing accumulates repeated flags into a slice.
func FormatGnokeyCreateCommand(profile *profiles.Profile, sessionPubkey string, scope Scope) string {
	var sb strings.Builder
	sb.WriteString("gnokey maketx session create \\\n")
	sb.WriteString(fmt.Sprintf("  --pubkey %s \\\n", sessionPubkey))
	for _, p := range scope.AllowPaths {
		sb.WriteString(fmt.Sprintf("  --allow-paths %s \\\n", p))
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
