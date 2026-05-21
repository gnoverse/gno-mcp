package chain

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	gnoclient "github.com/gnolang/gno/gno.land/pkg/gnoclient"
	"github.com/gnolang/gno/gno.land/pkg/sdk/vm"
	rpcclient "github.com/gnolang/gno/tm2/pkg/bft/rpc/client"
	"github.com/gnolang/gno/tm2/pkg/crypto"
	"github.com/gnolang/gno/tm2/pkg/std"
)

// gnoclientSignerProvider is satisfied by session-managed keys. Real.Call/Run
// type-asserts the Signer to this interface to acquire a tx-signing gnoclient.Signer.
// Keeps gnoclient.Signer out of the chain.Signer interface (avoids leakage).
type gnoclientSignerProvider interface {
	GnoclientSigner() gnoclient.Signer
}

// Real implements Client against a live gno chain via gnoclient + RPC.
//
// Context propagation: the gnoclient package does not accept context.Context;
// Real's methods therefore drop the caller's ctx. Callers that need to bound
// RPC duration must wrap the call externally (e.g., goroutine + select on ctx)
// until gnoclient grows ctx-aware variants.
type Real struct {
	cli *gnoclient.Client
}

// Assert Real satisfies Client at compile time.
var _ Client = (*Real)(nil)

// NewReal creates a Real client connected to rpcURL.
// rpcURL must be non-empty. The chainID argument is accepted to keep the
// constructor signature stable across milestones; it will be consumed by
// Milestone B signing operations and is unused here.
func NewReal(rpcURL, _ string) (*Real, error) {
	if rpcURL == "" {
		return nil, fmt.Errorf("rpc-url must not be empty")
	}

	rpc, err := rpcclient.NewHTTPClient(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("rpc client: %w", err)
	}

	return &Real{
		cli: &gnoclient.Client{RPCClient: rpc},
	}, nil
}

// Render returns the rendered output for a realm at a given subpath.
// Backed by vm/qrender.
func (r *Real) Render(_ context.Context, realm, path string) (string, error) {
	out, _, err := r.cli.Render(realm, path)
	if err != nil {
		return "", fmt.Errorf("vm/qrender: %w", err)
	}
	return out, nil
}

// Eval evaluates an expression in a realm's context.
// Backed by vm/qeval.
func (r *Real) Eval(_ context.Context, realm, expr string) (string, error) {
	out, _, err := r.cli.QEval(realm, expr)
	if err != nil {
		return "", fmt.Errorf("vm/qeval: %w", err)
	}
	return out, nil
}

// File returns the raw source of a single file in a realm.
// Backed by vm/qfile with an explicit file name appended to realm.
// Returns an error if file is empty; use ListFiles to enumerate names.
func (r *Real) File(_ context.Context, realm, file string) (string, error) {
	if file == "" {
		return "", fmt.Errorf("vm/qfile: file name must not be empty; use ListFiles for listings")
	}
	qres, err := r.cli.Query(gnoclient.QueryCfg{
		Path: "vm/qfile",
		Data: []byte(realm + "/" + file),
	})
	if err != nil {
		return "", fmt.Errorf("vm/qfile: %w", err)
	}
	return string(qres.Response.Data), nil
}

// ListFiles returns the file names that make up a realm.
// Backed by vm/qfile without a file name; result is newline-separated.
func (r *Real) ListFiles(_ context.Context, realm string) ([]string, error) {
	qres, err := r.cli.Query(gnoclient.QueryCfg{
		Path: "vm/qfile",
		Data: []byte(realm),
	})
	if err != nil {
		return nil, fmt.Errorf("vm/qfile: %w", err)
	}

	var files []string
	for _, name := range strings.Split(string(qres.Response.Data), "\n") {
		name = strings.TrimSpace(name)
		if name != "" {
			files = append(files, name)
		}
	}
	return files, nil
}

// Doc returns the realm's package + per-function godoc.
// Backed by vm/qdoc.
func (r *Real) Doc(_ context.Context, realm string) (string, error) {
	qres, err := r.cli.Query(gnoclient.QueryCfg{
		Path: "vm/qdoc",
		Data: []byte(realm),
	})
	if err != nil {
		return "", fmt.Errorf("vm/qdoc: %w", err)
	}
	return string(qres.Response.Data), nil
}

// Call broadcasts (or simulates) a vm/MsgCall transaction through gnoclient.
// signer must be non-nil and must implement gnoclientSignerProvider (the
// session.Keypair satisfies this).
func (r *Real) Call(_ context.Context, signer Signer, realm, fn string, args []string, simulate bool) (CallResult, error) {
	if signer == nil {
		return CallResult{}, fmt.Errorf("call: signer required (got nil)")
	}
	provider, ok := signer.(gnoclientSignerProvider)
	if !ok {
		return CallResult{}, fmt.Errorf("call: signer does not provide a gnoclient.Signer (session keypair required)")
	}
	gsigner := provider.GnoclientSigner()
	if gsigner == nil {
		return CallResult{}, fmt.Errorf("call: gnoclientSignerProvider returned nil Signer")
	}

	caller, err := crypto.AddressFromBech32(signer.Address())
	if err != nil {
		return CallResult{}, fmt.Errorf("call: invalid signer address %q: %w", signer.Address(), err)
	}

	msg := vm.MsgCall{
		Caller:  caller,
		PkgPath: realm,
		Func:    fn,
		Args:    args,
	}
	baseCfg := gnoclient.BaseTxCfg{
		GasFee:    "1ugnot",
		GasWanted: 5_000_000,
	}

	// Attach the gnoclient.Signer for this call.
	// NOTE: not goroutine-safe across concurrent Real.Call invocations;
	// the session manager must serialize per-client access.
	r.cli.Signer = gsigner

	if simulate {
		unsignedTx, err := gnoclient.NewCallTx(baseCfg, msg)
		if err != nil {
			return CallResult{}, fmt.Errorf("call: build unsigned tx: %w", err)
		}
		signedTx, err := r.cli.SignTx(*unsignedTx, 0, 0)
		if err != nil {
			return CallResult{}, fmt.Errorf("call: sign tx for simulate: %w", err)
		}
		deliver, err := r.cli.Simulate(signedTx)
		if err != nil {
			return CallResult{}, fmt.Errorf("call: simulate: %w", err)
		}
		return CallResult{
			Simulated: true,
			GasUsed:   deliver.GasUsed,
			Result:    string(deliver.Data),
		}, nil
	}

	res, err := r.cli.Call(baseCfg, msg)
	if err != nil {
		return CallResult{}, fmt.Errorf("call: %w", err)
	}
	return CallResult{
		TxHash:  hex.EncodeToString(res.Hash),
		Height:  res.Height,
		Result:  string(res.DeliverTx.Data),
		GasUsed: res.DeliverTx.GasUsed,
	}, nil
}

// Run broadcasts (or simulates) a vm/MsgRun transaction through gnoclient.
// The code string is wrapped in a single-file MemPackage with package name
// "main". signer must be non-nil and must implement gnoclientSignerProvider.
func (r *Real) Run(_ context.Context, signer Signer, code string, simulate bool) (RunResult, error) {
	if signer == nil {
		return RunResult{}, fmt.Errorf("run: signer required (got nil)")
	}
	provider, ok := signer.(gnoclientSignerProvider)
	if !ok {
		return RunResult{}, fmt.Errorf("run: signer does not provide a gnoclient.Signer (session keypair required)")
	}
	gsigner := provider.GnoclientSigner()
	if gsigner == nil {
		return RunResult{}, fmt.Errorf("run: gnoclientSignerProvider returned nil Signer")
	}

	caller, err := crypto.AddressFromBech32(signer.Address())
	if err != nil {
		return RunResult{}, fmt.Errorf("run: invalid signer address %q: %w", signer.Address(), err)
	}

	files := []*std.MemFile{{Name: "main.gno", Body: code}}
	msg := vm.NewMsgRun(caller, nil, files)
	baseCfg := gnoclient.BaseTxCfg{
		GasFee:    "1ugnot",
		GasWanted: 5_000_000,
	}

	// Attach the gnoclient.Signer for this call.
	// NOTE: not goroutine-safe across concurrent Real.Run invocations;
	// the session manager must serialize per-client access.
	r.cli.Signer = gsigner

	if simulate {
		unsignedTx, err := gnoclient.NewRunTx(baseCfg, msg)
		if err != nil {
			return RunResult{}, fmt.Errorf("run: build unsigned tx: %w", err)
		}
		signedTx, err := r.cli.SignTx(*unsignedTx, 0, 0)
		if err != nil {
			return RunResult{}, fmt.Errorf("run: sign tx for simulate: %w", err)
		}
		deliver, err := r.cli.Simulate(signedTx)
		if err != nil {
			return RunResult{}, fmt.Errorf("run: simulate: %w", err)
		}
		return RunResult{
			Simulated: true,
			GasUsed:   deliver.GasUsed,
			Output:    string(deliver.Data),
		}, nil
	}

	res, err := r.cli.Run(baseCfg, msg)
	if err != nil {
		return RunResult{}, fmt.Errorf("run: %w", err)
	}
	return RunResult{
		TxHash:  hex.EncodeToString(res.Hash),
		Height:  res.Height,
		Output:  string(res.DeliverTx.Data),
		GasUsed: res.DeliverTx.GasUsed,
	}, nil
}

// QuerySession returns the chain-side SessionStatus for the given bech32 pubkey.
//
// Per the Task 2.3 research (see internal/chain/README.md), no per-pubkey session
// ABCI path exists in the current chain build. This implementation returns
// ErrSessionQueryUnsupported for any non-empty pubkey. The session.Manager
// catches this sentinel and degrades to local-authoritative session state.
//
// When a per-pubkey session ABCI path lands in gnoclient (post-PR #5307), this
// method will be replaced with a real implementation. The chain.Client interface
// stays correct in the meantime.
func (r *Real) QuerySession(_ context.Context, sessionPubkey string) (SessionStatus, error) {
	if sessionPubkey == "" {
		return SessionStatus{}, fmt.Errorf("querysession: pubkey must not be empty")
	}
	return SessionStatus{}, ErrSessionQueryUnsupported
}
