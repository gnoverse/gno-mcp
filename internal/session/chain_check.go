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
// Returns Active=false (no error) when the session is not registered on chain,
// has expired, has been revoked, or the chain does not support per-pubkey
// session query (ErrSessionQueryUnsupported). Returns an error only when the
// resolver cannot provide a client for the profile.
//
// Per D8 (see runlog): Real.QuerySession returns ErrSessionQueryUnsupported
// for every non-empty pubkey today. The fall-through "any chain error =
// inactive" branch absorbs this. The session.Manager treats inactive as
// "trust local state until next broadcast proves otherwise."
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
