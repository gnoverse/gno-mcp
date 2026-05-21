package session

import (
	"context"
	"fmt"

	"github.com/gnoverse/gno-mcp/internal/chain"
)

// chainCheckResult is the outcome of querying the chain for a session pubkey.
type chainCheckResult struct {
	Active bool
	Status chain.SessionStatus
}

// queryChain looks up the session by pubkey using the profile's chain client.
// Any chain error (including ErrSessionQueryUnsupported from chains that do
// not implement per-pubkey session query) is reported as Active=false with no
// error. The caller decides how to surface this absence (e.g., trust local
// state until a successful chain confirmation or the next Hydrate cycle).
// Returns a typed error only when the resolver cannot provide a client.
func queryChain(ctx context.Context, resolver chain.Resolver, profile, sessionPubkey string) (chainCheckResult, error) {
	client := resolver(profile)
	if client == nil {
		return chainCheckResult{}, fmt.Errorf("session: no chain client for profile %q", profile)
	}
	status, err := client.QuerySession(ctx, sessionPubkey)
	if err != nil {
		return chainCheckResult{Active: false}, nil
	}
	return chainCheckResult{
		Active: status.Active,
		Status: status,
	}, nil
}
