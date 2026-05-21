package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/gnoverse/gno-mcp/internal/chain"
)

// chainCheckResult is the outcome of querying the chain for a session pubkey.
// Unsupported=true means the chain build does not expose a per-pubkey session
// query path; callers should keep local state as authoritative.
type chainCheckResult struct {
	Active      bool
	Unsupported bool
	Status      chain.SessionStatus
}

// queryChain looks up the session by pubkey using the profile's chain client.
// When the chain returns ErrSessionQueryUnsupported, the result has
// Unsupported=true and a nil error so callers can distinguish "chain says
// inactive" (delete) from "chain doesn't know" (keep).
// Other chain errors are reported as Active=false with no error.
// Returns a typed error only when the resolver cannot provide a client.
func queryChain(ctx context.Context, resolver chain.Resolver, profile, sessionPubkey string) (chainCheckResult, error) {
	client := resolver(profile)
	if client == nil {
		return chainCheckResult{}, fmt.Errorf("session: no chain client for profile %q", profile)
	}
	status, err := client.QuerySession(ctx, sessionPubkey)
	if err != nil {
		if errors.Is(err, chain.ErrSessionQueryUnsupported) {
			return chainCheckResult{Unsupported: true}, nil
		}
		return chainCheckResult{Active: false}, nil
	}
	return chainCheckResult{
		Active: status.Active,
		Status: status,
	}, nil
}
