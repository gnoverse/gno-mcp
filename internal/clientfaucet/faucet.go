// Package clientfaucet selects and drives a per-profile funding backend for the
// agent's testnet key: a ServiceFaucet (automatic service), a LinkFaucet (an
// existing human faucet page), or the manual fallback (LinkFaucet with no URL).
package clientfaucet

import (
	"context"
	"net/http"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/profiles"
)

// Outcome describes what Fund did and how the human (if any) should proceed.
type Outcome struct {
	Backend      string // "service" | "link" | "manual"
	Address      string // agent address being funded
	TxHash       string // service backend only
	FaucetURL    string // link backend, if a URL is configured
	Instructions string // profile-adaptive, human-readable next step
}

// Faucet is a funding backend. Fund initiates funding; Funded reports whether
// the address now holds a positive balance (the confirmation poll).
type Faucet interface {
	Fund(ctx context.Context, address, chainID string) (Outcome, error)
	Funded(ctx context.Context, address string) (bool, error)
}

// Resolve picks the backend for p by precedence: faucet-service-url > faucet-url
// > none. The link backend handles both tier 1 (URL set) and the manual fallback
// (URL empty). chainClient backs the Funded balance poll for both backends.
func Resolve(p profiles.Profile, chainClient chain.Client, httpClient *http.Client) Faucet {
	if p.FaucetServiceURL != "" {
		return &ServiceFaucet{url: p.FaucetServiceURL, http: httpClient, chain: chainClient}
	}
	return &LinkFaucet{faucetURL: p.FaucetURL, chain: chainClient}
}
