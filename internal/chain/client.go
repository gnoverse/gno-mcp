// Package chain abstracts read-only access to a gno chain via the Client interface.
package chain

import (
	"context"
	"errors"
)

// Signer is the chain-bound session key abstraction used by write methods.
// Implemented by the in-memory session keypair in internal/session/.
type Signer interface {
	Address() string                     // bech32 g1...
	Sign(payload []byte) ([]byte, error) // ed25519
}

// ErrSimulateUnsupported is returned by Real.Call/Run when the underlying
// gnoclient does not expose a simulate primitive. Tools translate this to a
// simulate_unsupported ToolError.
var ErrSimulateUnsupported = errors.New("chain: simulate not supported by current gnoclient")

// CallResult is the outcome of a vm/MsgCall broadcast (or simulation).
type CallResult struct {
	TxHash    string
	Height    int64
	Result    string
	GasUsed   int64
	Simulated bool
}

// RunResult is the outcome of a vm/MsgRun broadcast (or simulation).
type RunResult struct {
	TxHash    string
	Height    int64
	Output    string
	GasUsed   int64
	Simulated bool
}

// SessionStatus is the chain-side state of a session keyed by pubkey.
// The Active=false case covers expired, revoked, and never-registered.
type SessionStatus struct {
	Active         bool
	AllowPaths     []string
	SpendLimit     string
	SpendRemaining string
	ExpiresAt      int64
}

// Client abstracts the gno chain operations gnomcp needs.
// Implemented by Real (gnoclient + RPC) and Fake (in-memory for tests).
type Client interface {
	// Render returns the rendered markdown for a realm at a given subpath.
	// Backed by vm/qrender.
	Render(ctx context.Context, realm, path string) (string, error)

	// Eval evaluates an expression in a realm's context.
	// Backed by vm/qeval.
	Eval(ctx context.Context, realm, expr string) (string, error)

	// File returns the raw source of a single file in a realm.
	// Backed by vm/qfile with an explicit file name.
	File(ctx context.Context, realm, file string) (string, error)

	// ListFiles returns the file names that make up a realm.
	// Backed by vm/qfile without a file name argument.
	ListFiles(ctx context.Context, realm string) ([]string, error)

	// Doc returns the realm's package + per-function godoc.
	// Backed by vm/qdoc.
	Doc(ctx context.Context, realm string) (string, error)

	// Call broadcasts (or simulates) a vm/MsgCall for the given realm function.
	// signer must be non-nil when simulate is false.
	Call(ctx context.Context, signer Signer, realm, fn string, args []string, simulate bool) (CallResult, error)

	// Run broadcasts (or simulates) a vm/MsgRun for ad-hoc gno code execution.
	// signer must be non-nil when simulate is false.
	Run(ctx context.Context, signer Signer, code string, simulate bool) (RunResult, error)

	// QuerySession returns the chain-side state for a session identified by its
	// bech32 pubkey (gpub1...). Returns SessionStatus{Active: false} (no error)
	// when the pubkey has no on-chain session record.
	QuerySession(ctx context.Context, sessionPubkey string) (SessionStatus, error)
}
