package write

import (
	"fmt"

	"github.com/gnoverse/gno-mcp/internal/profiles"
)

// signedByLine renders the acting-identity line for a write result. The agent
// identity is the well-known test1 account on local (dev) chains and a
// per-profile generated key on testnet, so the label is tier-dependent.
func signedByLine(identity, signerAddr, master, chainType string) string {
	if identity == "session" {
		return fmt.Sprintf("Signed by: session %s on behalf of master %s", signerAddr, master)
	}
	if chainType == profiles.ChainTypeLocal {
		return fmt.Sprintf("Signed by: agent test1 (%s)", signerAddr)
	}
	return fmt.Sprintf("Signed by: agent (%s)", signerAddr)
}
