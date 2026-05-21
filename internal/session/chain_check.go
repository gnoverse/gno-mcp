package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/gnoverse/gno-mcp/internal/chain"
)

// chainCheckResult is the outcome of querying the chain for a session.
// Unsupported=true means the chain client could not be asked (e.g. master
// address unknown); callers should keep local state as authoritative.
type chainCheckResult struct {
	Active      bool
	Unsupported bool
	Status      chain.SessionStatus
}

// queryChain looks up the session at auth/accounts/<master>/session/<sessionAddr>
// using the profile's chain client. When the chain returns
// ErrSessionQueryUnsupported (e.g. master is empty), the result has
// Unsupported=true and a nil error so callers can distinguish "chain says
// inactive" (delete) from "chain doesn't know" (keep).
// Other chain errors are reported as Active=false with no error.
// Returns a typed error only when the resolver cannot provide a client.
func queryChain(ctx context.Context, resolver chain.Resolver, profile, master, sessionAddr string) (chainCheckResult, error) {
	client := resolver(profile)
	if client == nil {
		return chainCheckResult{}, fmt.Errorf("session: no chain client for profile %q", profile)
	}
	if master == "" {
		// No master yet (e.g. older session created before MasterAddress was
		// plumbed). Don't wipe — let local state govern.
		return chainCheckResult{Unsupported: true}, nil
	}
	status, err := client.QuerySession(ctx, master, sessionAddr)
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
