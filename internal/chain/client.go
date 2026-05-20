// Package chain abstracts read-only access to a gno chain via the Client interface.
package chain

import "context"

// Client abstracts the gno chain operations gnomcp needs.
// Implemented by Real (gnoclient + RPC) and Fake (in-memory for tests).
// Milestone A is read-only; write methods (Call, Run, Sign) come in Milestone B.
type Client interface {
	// Render returns the rendered markdown for a realm at a given subpath.
	// Backed by vm/qrender.
	Render(ctx context.Context, realm, path string) (string, error)

	// Eval evaluates an expression in a realm's context.
	// Backed by vm/qeval.
	Eval(ctx context.Context, realm, expr string) (string, error)

	// File returns the raw source of a file in a realm.
	// If file is empty, returns a list of file names.
	// Backed by vm/qfile.
	File(ctx context.Context, realm, file string) (string, error)

	// Doc returns the realm's package + per-function godoc.
	// Backed by vm/qdoc.
	Doc(ctx context.Context, realm string) (string, error)
}
