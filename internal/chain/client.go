// Package chain abstracts read-only access to a gno chain via the Client interface.
package chain

import (
	"context"
	"errors"

	gnoclient "github.com/gnolang/gno/gno.land/pkg/gnoclient"
	"github.com/gnolang/gno/tm2/pkg/std"
)

// Signer is the chain-bound session key abstraction used by write methods.
// Implemented by the in-memory session keypair in internal/session/.
// Pubkey returns the raw ed25519 public key bytes; Real.CallAsUser/RunAsUser wraps
// these into a std.Signature alongside a non-zero SessionAddr so the ante handler
// loads the matching session record from auth/accounts/<master>/session/<addr>.
type Signer interface {
	Address() string                     // bech32 g1...
	Pubkey() []byte                      // raw ed25519 public key (32 bytes)
	Sign(payload []byte) ([]byte, error) // ed25519
}

// ErrSimulateUnsupported is returned by Real.CallAsUser/RunAsUser when the
// underlying gnoclient does not expose a simulate primitive. Tools translate
// this to a simulate_unsupported ToolError.
var ErrSimulateUnsupported = errors.New("chain: simulate not supported by current gnoclient")

// ErrSessionQueryUnsupported is returned when a chain client cannot resolve
// a session — for example, when the caller could not supply a master address.
// The session.Manager catches this and falls back to local-authoritative state
// instead of wiping sessions on hydrate.
var ErrSessionQueryUnsupported = errors.New("chain: session query not supported")

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

// AddPackageResult is the outcome of a vm/MsgAddPackage broadcast (or simulation).
type AddPackageResult struct {
	TxHash    string
	Height    int64
	GasUsed   int64
	Simulated bool
}

// SessionStatus is the chain-side state of a session keyed by pubkey.
// The Active=false case covers expired, revoked, and never-registered.
type SessionStatus struct {
	Active         bool
	AllowPaths     []string
	AllowRun       bool
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

	// ListPaths enumerates package paths under target via vm/qpaths. target is
	// either a path prefix ("gno.land/r/demo/") or "@namespace" ("@demo" → both
	// /p/ and /r/). limit<=0 uses the chain default (1000); the chain caps at
	// 10000. Returns fully-qualified package paths.
	ListPaths(ctx context.Context, target string, limit int) ([]string, error)

	// Doc returns the realm's package + per-function godoc.
	// Backed by vm/qdoc.
	Doc(ctx context.Context, realm string) (string, error)

	// CallAsUser broadcasts (or simulates) a session-signed vm/MsgCall for the
	// given realm function. master is the bech32 master address (g1...) the
	// session was registered under; the broadcast MsgCall.Caller is set to
	// master and the tx is signed by the session keypair (via signer) with
	// Signature.SessionAddr pointing at signer.Address(). signer must be
	// non-nil when simulate is false.
	CallAsUser(ctx context.Context, signer Signer, master, realm, fn string, args []string, simulate bool) (CallResult, error)

	// RunAsUser broadcasts (or simulates) a session-signed vm/MsgRun for
	// ad-hoc gno code execution. master is the bech32 master address the
	// session is registered under. signer must be non-nil when simulate is false.
	RunAsUser(ctx context.Context, signer Signer, master, code string, simulate bool) (RunResult, error)

	// QuerySession returns the chain-side state for the session at
	// auth/accounts/<master>/session/<sessionAddr>. Both arguments are bech32
	// (g1...) addresses. Returns SessionStatus{Active: false} (no error) when
	// the chain has no record. Returns ErrSessionQueryUnsupported when the
	// query cannot be made (e.g. master is unknown to the caller); the
	// session.Manager handles this by keeping local state authoritative.
	QuerySession(ctx context.Context, master, sessionAddr string) (SessionStatus, error)

	// Call broadcasts (or simulates) a STANDARD vm/MsgCall signed by the agent key
	// (Caller = signer's own address; no master, no SessionAddr). signer is a gnoclient.Signer.
	Call(ctx context.Context, signer gnoclient.Signer, realm, fn string, args []string, simulate bool) (CallResult, error)

	// Run broadcasts (or simulates) a STANDARD vm/MsgRun signed by the agent key.
	Run(ctx context.Context, signer gnoclient.Signer, code string, simulate bool) (RunResult, error)

	// AddPackage broadcasts (or simulates) a vm/MsgAddPackage signed by the agent key.
	AddPackage(ctx context.Context, signer gnoclient.Signer, deployPath string, files []*std.MemFile, simulate bool) (AddPackageResult, error)

	// Balance returns the ugnot balance of a bech32 address. A never-funded address
	// (unknown to the chain) reports 0 with no error. Intended as a "can this account
	// pay gas" pre-check for agent testnet writes; it conflates funded-to-zero and
	// unknown (both 0) and is not a general balance API.
	Balance(ctx context.Context, addr string) (int64, error)
}

// Resolver returns the Client to use for a given profile name.
// The caller wires this up — typically maps profile name to a Real
// instance constructed from the profile's RPC URL. Tools use
// chain.Resolver as the dependency-injection point for the chain
// client so they remain agnostic to how clients are constructed and
// cached.
type Resolver func(profile string) Client
