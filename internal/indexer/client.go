// Package indexer abstracts read-only access to tx-indexer GraphQL queries.
package indexer

import (
	"context"
	"time"
)

// TxEvent is one indexed transaction touching a realm.
type TxEvent struct {
	Hash   string
	Height int64
	Time   time.Time
	Kind   string // MsgAddPackage / MsgCall / MsgRun
	Caller string
	Func   string // for MsgCall
	Args   []string
}

// Client abstracts tx-indexer GraphQL queries.
type Client interface {
	// History returns every transaction touching realm in chronological order.
	History(ctx context.Context, realm string) ([]TxEvent, error)

	// Activity returns events for realm within the closed interval
	// [since, until]. A nil pointer means that bound is unconstrained.
	Activity(ctx context.Context, realm string, since, until *time.Time) ([]TxEvent, error)
}

// Resolver returns the Client to use for a given profile name.
// nil = profile has no tx-indexer-url configured; the caller should
// surface a clear error. Tools take Resolver as the DI seam for the
// indexer client.
type Resolver func(profile string) Client
