package chain

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"

	gnoclient "github.com/gnolang/gno/gno.land/pkg/gnoclient"
	"github.com/gnolang/gno/gno.land/pkg/gnoland"
	"github.com/gnolang/gno/gno.land/pkg/gnoland/ugnot"
	"github.com/gnolang/gno/gno.land/pkg/sdk/vm"
	"github.com/gnolang/gno/tm2/pkg/amino"
	rpcclient "github.com/gnolang/gno/tm2/pkg/bft/rpc/client"
	"github.com/gnolang/gno/tm2/pkg/crypto"
	tmed25519 "github.com/gnolang/gno/tm2/pkg/crypto/ed25519"
	"github.com/gnolang/gno/tm2/pkg/std"
)

// Real implements Client against a live gno chain via gnoclient + RPC.
//
// Context propagation: the gnoclient package does not accept context.Context;
// Real's methods therefore drop the caller's ctx. Callers that need to bound
// RPC duration must wrap the call externally (e.g., goroutine + select on ctx)
// until gnoclient grows ctx-aware variants.
type Real struct {
	cli     *gnoclient.Client
	chainID string
}

// Assert Real satisfies Client at compile time.
var _ Client = (*Real)(nil)

// NewReal creates a Real client connected to rpcURL with the given chainID.
// Both must be non-empty: chainID is part of every signed tx's signature
// payload, so a mismatch with the node's chainID produces an opaque
// verification failure at broadcast time.
func NewReal(rpcURL, chainID string) (*Real, error) {
	if rpcURL == "" {
		return nil, fmt.Errorf("rpc-url must not be empty")
	}
	if chainID == "" {
		return nil, fmt.Errorf("chain-id must not be empty")
	}

	rpc, err := rpcclient.NewHTTPClient(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("rpc client: %w", err)
	}

	return &Real{
		cli:     &gnoclient.Client{RPCClient: rpc},
		chainID: chainID,
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

// CallAsUser broadcasts (or simulates) a session-signed vm/MsgCall through
// gnoclient. MsgCall.Caller is the master address; the signature carries the
// session's pubkey and SessionAddr so the chain's ante handler verifies against
// the session record at auth/accounts/<master>/session/<sessionAddr>.
func (r *Real) CallAsUser(_ context.Context, signer Signer, master, realm, fn string, args []string, simulate bool) (CallResult, error) {
	if signer == nil {
		return CallResult{}, fmt.Errorf("call as user: signer required (got nil)")
	}
	if master == "" {
		return CallResult{}, fmt.Errorf("call as user: master address required for session-signed tx")
	}

	masterAddr, err := crypto.AddressFromBech32(master)
	if err != nil {
		return CallResult{}, fmt.Errorf("call as user: invalid master address %q: %w", master, err)
	}
	sessionAddr, err := crypto.AddressFromBech32(signer.Address())
	if err != nil {
		return CallResult{}, fmt.Errorf("call as user: invalid session address %q: %w", signer.Address(), err)
	}

	msg := vm.MsgCall{
		Caller:  masterAddr,
		PkgPath: realm,
		Func:    fn,
		Args:    args,
	}
	unsignedTx, err := gnoclient.NewCallTx(defaultBaseTxCfg(), msg)
	if err != nil {
		return CallResult{}, fmt.Errorf("call as user: build unsigned tx: %w", err)
	}

	signedTx, err := r.signTxForSession(unsignedTx, signer, masterAddr, sessionAddr)
	if err != nil {
		return CallResult{}, fmt.Errorf("call as user: sign tx: %w", err)
	}

	if simulate {
		deliver, err := r.cli.Simulate(signedTx)
		if err != nil {
			return CallResult{}, fmt.Errorf("call as user: simulate: %w", err)
		}
		return CallResult{
			Simulated: true,
			GasUsed:   deliver.GasUsed,
			Result:    string(deliver.Data),
		}, nil
	}

	res, err := r.cli.BroadcastTxCommit(signedTx)
	if err != nil {
		return CallResult{}, fmt.Errorf("call as user: broadcast tx: %w", err)
	}
	return CallResult{
		TxHash:  hex.EncodeToString(res.Hash),
		Height:  res.Height,
		Result:  string(res.DeliverTx.Data),
		GasUsed: res.DeliverTx.GasUsed,
	}, nil
}

// RunAsUser broadcasts (or simulates) a session-signed vm/MsgRun. The code is
// wrapped in a single-file MemPackage with package name "main".
func (r *Real) RunAsUser(_ context.Context, signer Signer, master, code string, simulate bool) (RunResult, error) {
	if signer == nil {
		return RunResult{}, fmt.Errorf("run as user: signer required (got nil)")
	}
	if master == "" {
		return RunResult{}, fmt.Errorf("run as user: master address required for session-signed tx")
	}

	masterAddr, err := crypto.AddressFromBech32(master)
	if err != nil {
		return RunResult{}, fmt.Errorf("run as user: invalid master address %q: %w", master, err)
	}
	sessionAddr, err := crypto.AddressFromBech32(signer.Address())
	if err != nil {
		return RunResult{}, fmt.Errorf("run as user: invalid session address %q: %w", signer.Address(), err)
	}

	files := []*std.MemFile{{Name: "main.gno", Body: code}}
	msg := vm.NewMsgRun(masterAddr, nil, files)
	unsignedTx, err := gnoclient.NewRunTx(defaultBaseTxCfg(), msg)
	if err != nil {
		return RunResult{}, fmt.Errorf("run as user: build unsigned tx: %w", err)
	}

	signedTx, err := r.signTxForSession(unsignedTx, signer, masterAddr, sessionAddr)
	if err != nil {
		return RunResult{}, fmt.Errorf("run as user: sign tx: %w", err)
	}

	if simulate {
		deliver, err := r.cli.Simulate(signedTx)
		if err != nil {
			return RunResult{}, fmt.Errorf("run as user: simulate: %w", err)
		}
		return RunResult{
			Simulated: true,
			GasUsed:   deliver.GasUsed,
			Output:    string(deliver.Data),
		}, nil
	}

	res, err := r.cli.BroadcastTxCommit(signedTx)
	if err != nil {
		return RunResult{}, fmt.Errorf("run as user: broadcast tx: %w", err)
	}
	return RunResult{
		TxHash:  hex.EncodeToString(res.Hash),
		Height:  res.Height,
		Output:  string(res.DeliverTx.Data),
		GasUsed: res.DeliverTx.GasUsed,
	}, nil
}

// QuerySession looks up a session account at auth/accounts/<master>/session/<sessionAddr>.
// The chain emits the GnoSessionAccount via amino-JSON. Returns
// SessionStatus{Active: false}, nil when the chain reports "session not
// found." Returns ErrSessionQueryUnsupported on any other query failure
// (transient RPC, malformed response) so the Manager preserves local state
// rather than wiping sessions on a flake.
func (r *Real) QuerySession(_ context.Context, master, sessionAddr string) (SessionStatus, error) {
	if master == "" || sessionAddr == "" {
		return SessionStatus{}, ErrSessionQueryUnsupported
	}

	path := fmt.Sprintf("auth/accounts/%s/session/%s", master, sessionAddr)
	qres, err := r.cli.Query(gnoclient.QueryCfg{Path: path})
	if err != nil {
		if isSessionNotFoundErr(err) {
			return SessionStatus{Active: false}, nil
		}
		return SessionStatus{}, ErrSessionQueryUnsupported
	}
	if len(qres.Response.Data) == 0 || string(qres.Response.Data) == "null" {
		return SessionStatus{Active: false}, nil
	}

	acc, err := decodeSessionAccount(qres.Response.Data)
	if err != nil {
		return SessionStatus{}, fmt.Errorf("querysession: decode session account: %w", err)
	}

	realmPaths, allowRun := splitAllowPaths(acc.AllowPaths)
	return SessionStatus{
		Active:         true,
		AllowPaths:     realmPaths,
		AllowRun:       allowRun,
		SpendLimit:     acc.SpendLimit.String(),
		SpendRemaining: spendRemaining(acc.SpendLimit, acc.SpendUsed).String(),
		ExpiresAt:      acc.ExpiresAt,
	}, nil
}

// splitAllowPaths translates chain-native permission entries into gnomcp's
// internal representation: "vm/exec:<realm>" becomes a bare realm path,
// "vm/run" sets allowRun=true. Tokens outside the MVP grammar (e.g.
// bank/send) are dropped silently — future versions may surface them.
func splitAllowPaths(chainPaths []string) (realmPaths []string, allowRun bool) {
	for _, p := range chainPaths {
		if stripped, ok := strings.CutPrefix(p, "vm/exec:"); ok {
			realmPaths = append(realmPaths, stripped)
			continue
		}
		if p == "vm/run" {
			allowRun = true
			continue
		}
	}
	return realmPaths, allowRun
}

// signTxForSession runs the session-signing flow: query the session account
// for its (account_number, sequence), compute sign-bytes, sign with the
// session keypair, then inject Signature.SessionAddr.
func (r *Real) signTxForSession(unsignedTx *std.Tx, signer Signer, masterAddr, sessionAddr crypto.Address) (*std.Tx, error) {
	accNum, seq, err := r.querySessionSequence(masterAddr, sessionAddr)
	if err != nil {
		return nil, fmt.Errorf("query session sequence: %w", err)
	}

	signBytes, err := unsignedTx.GetSignBytes(r.chainID, accNum, seq)
	if err != nil {
		return nil, fmt.Errorf("get sign bytes: %w", err)
	}

	sig, err := signer.Sign(signBytes)
	if err != nil {
		return nil, fmt.Errorf("session sign: %w", err)
	}

	pubBytes := signer.Pubkey()
	if len(pubBytes) != tmed25519.PubKeyEd25519Size {
		return nil, fmt.Errorf("invalid session pubkey length %d", len(pubBytes))
	}
	var pk tmed25519.PubKeyEd25519
	copy(pk[:], pubBytes)

	signedTx := *unsignedTx
	signedTx.Signatures = []std.Signature{{
		PubKey:      pk,
		Signature:   sig,
		SessionAddr: sessionAddr,
	}}
	return &signedTx, nil
}

// querySessionSequence returns the session account's (AccountNumber, Sequence)
// from auth/accounts/<master>/session/<sessionAddr>. The response is
// amino-JSON-encoded (NOT std JSON) so we route through decodeSessionAccount.
func (r *Real) querySessionSequence(master, sessionAddr crypto.Address) (uint64, uint64, error) {
	path := fmt.Sprintf("auth/accounts/%s/session/%s", master.String(), sessionAddr.String())
	qres, err := r.cli.Query(gnoclient.QueryCfg{Path: path})
	if err != nil {
		return 0, 0, fmt.Errorf("auth/accounts query: %w", err)
	}
	if len(qres.Response.Data) == 0 || string(qres.Response.Data) == "null" {
		return 0, 0, errors.New("session not found")
	}
	acc, err := decodeSessionAccount(qres.Response.Data)
	if err != nil {
		return 0, 0, fmt.Errorf("decode session account: %w", err)
	}
	return acc.AccountNumber, acc.Sequence, nil
}

// decodeSessionAccount parses the amino-JSON payload returned by
// auth/accounts/<master>/session/<addr>. Amino-JSON differs from std JSON in
// three load-bearing ways: embedded structs are not flattened (so the embedded
// std.BaseSessionAccount appears under its own key), integers are
// string-encoded, and std.Coins marshals as "<amount><denom>". encoding/json
// silently drops the embedded subtree, zeroing AccountNumber/Sequence/
// ExpiresAt and corrupting the next tx signature. Mirror gnokey's flat-decode
// shape at tm2/pkg/crypto/keys/client/maketx.go:220.
func decodeSessionAccount(data []byte) (*gnoland.GnoSessionAccount, error) {
	var wire struct {
		BaseSessionAccount std.BaseSessionAccount
		AllowPaths         []string `json:"allow_paths,omitempty"`
	}
	if err := amino.UnmarshalJSON(data, &wire); err != nil {
		return nil, fmt.Errorf("amino: %w", err)
	}
	return &gnoland.GnoSessionAccount{
		BaseSessionAccount: wire.BaseSessionAccount,
		AllowPaths:         wire.AllowPaths,
	}, nil
}

// isSessionNotFoundErr returns true when err matches the chain's
// std.ErrSessionNotFound. gnoclient.Query wraps the ABCI response error in a
// string-only chain (the typed error is not preserved), so we string-match
// the stable "session not found error" prefix coined at
// tm2/pkg/std/errors.go:62.
func isSessionNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "session not found")
}

// DefaultGasFeeUgnot is the ugnot amount of every write tx's GasFee. The chain
// charges a session this full GasFee per tx (sessions are billed the offered
// fee, not the gas actually used), so session spend tracking must deduct this —
// not GasUsed — to stay in sync with the chain's accounting.
const DefaultGasFeeUgnot int64 = 10_000_000

// DefaultMaxDepositUgnot caps the storage deposit for an agent AddPackage. Sufficient
// for typical realms; a deploy rejected for an insufficient deposit is the signal to raise it.
const DefaultMaxDepositUgnot int64 = 10_000_000

// agentClient returns a gnoclient.Client bound to the agent signer for a
// standard (non-session) tx, reusing Real's RPC connection.
func (r *Real) agentClient(signer gnoclient.Signer) *gnoclient.Client {
	return &gnoclient.Client{RPCClient: r.cli.RPCClient, Signer: signer}
}

// Call broadcasts (or simulates) a STANDARD vm/MsgCall signed by the agent key.
// Caller is the signer's own address; no session machinery is involved.
func (r *Real) Call(_ context.Context, signer gnoclient.Signer, realm, fn string, args []string, simulate bool) (CallResult, error) {
	if signer == nil {
		return CallResult{}, fmt.Errorf("call: signer required (got nil)")
	}
	info, err := signer.Info()
	if err != nil {
		return CallResult{}, fmt.Errorf("call: signer info: %w", err)
	}
	cli := r.agentClient(signer)
	msg := vm.MsgCall{Caller: info.GetAddress(), PkgPath: realm, Func: fn, Args: args}
	if simulate {
		unsigned, err := gnoclient.NewCallTx(defaultBaseTxCfg(), msg)
		if err != nil {
			return CallResult{}, fmt.Errorf("call: build tx: %w", err)
		}
		signed, err := cli.SignTx(*unsigned, 0, 0)
		if err != nil {
			return CallResult{}, fmt.Errorf("call: sign: %w", err)
		}
		deliver, err := cli.Simulate(signed)
		if err != nil {
			return CallResult{}, fmt.Errorf("call: simulate: %w", err)
		}
		return CallResult{Simulated: true, GasUsed: deliver.GasUsed, Result: string(deliver.Data)}, nil
	}
	res, err := cli.Call(defaultBaseTxCfg(), msg)
	if err != nil {
		return CallResult{}, fmt.Errorf("call: broadcast: %w", err)
	}
	return CallResult{TxHash: hex.EncodeToString(res.Hash), Height: res.Height, Result: string(res.DeliverTx.Data), GasUsed: res.DeliverTx.GasUsed}, nil
}

// Run broadcasts (or simulates) a STANDARD vm/MsgRun signed by the agent key.
// The code is wrapped in a single-file MemPackage with package name "main".
func (r *Real) Run(_ context.Context, signer gnoclient.Signer, code string, simulate bool) (RunResult, error) {
	if signer == nil {
		return RunResult{}, fmt.Errorf("run: signer required (got nil)")
	}
	info, err := signer.Info()
	if err != nil {
		return RunResult{}, fmt.Errorf("run: signer info: %w", err)
	}
	cli := r.agentClient(signer)
	files := []*std.MemFile{{Name: "main.gno", Body: code}}
	msg := vm.NewMsgRun(info.GetAddress(), nil, files)
	if simulate {
		unsigned, err := gnoclient.NewRunTx(defaultBaseTxCfg(), msg)
		if err != nil {
			return RunResult{}, fmt.Errorf("run: build tx: %w", err)
		}
		signed, err := cli.SignTx(*unsigned, 0, 0)
		if err != nil {
			return RunResult{}, fmt.Errorf("run: sign: %w", err)
		}
		deliver, err := cli.Simulate(signed)
		if err != nil {
			return RunResult{}, fmt.Errorf("run: simulate: %w", err)
		}
		return RunResult{Simulated: true, GasUsed: deliver.GasUsed, Output: string(deliver.Data)}, nil
	}
	res, err := cli.Run(defaultBaseTxCfg(), msg)
	if err != nil {
		return RunResult{}, fmt.Errorf("run: broadcast: %w", err)
	}
	return RunResult{TxHash: hex.EncodeToString(res.Hash), Height: res.Height, Output: string(res.DeliverTx.Data), GasUsed: res.DeliverTx.GasUsed}, nil
}

// AddPackage broadcasts (or simulates) a vm/MsgAddPackage signed by the agent key.
// Defense-in-depth: MemPackage.ValidateBasic rejects unsorted files. The addpkg handler
// (Task 5) sorts authoritatively; this guards any direct caller.
func (r *Real) AddPackage(_ context.Context, signer gnoclient.Signer, deployPath string, files []*std.MemFile, simulate bool) (AddPackageResult, error) {
	if signer == nil {
		return AddPackageResult{}, fmt.Errorf("addpackage: signer required (got nil)")
	}
	info, err := signer.Info()
	if err != nil {
		return AddPackageResult{}, fmt.Errorf("addpackage: signer info: %w", err)
	}
	slices.SortFunc(files, func(a, b *std.MemFile) int { return strings.Compare(a.Name, b.Name) })
	cli := r.agentClient(signer)
	msg := vm.NewMsgAddPackage(info.GetAddress(), deployPath, files)
	msg.MaxDeposit = std.Coins{{Denom: ugnot.Denom, Amount: DefaultMaxDepositUgnot}}
	if simulate {
		unsigned, err := gnoclient.NewAddPackageTx(defaultBaseTxCfg(), msg)
		if err != nil {
			return AddPackageResult{}, fmt.Errorf("addpackage: build tx: %w", err)
		}
		signed, err := cli.SignTx(*unsigned, 0, 0)
		if err != nil {
			return AddPackageResult{}, fmt.Errorf("addpackage: sign: %w", err)
		}
		deliver, err := cli.Simulate(signed)
		if err != nil {
			return AddPackageResult{}, fmt.Errorf("addpackage: simulate: %w", err)
		}
		return AddPackageResult{Simulated: true, GasUsed: deliver.GasUsed}, nil
	}
	res, err := cli.AddPackage(defaultBaseTxCfg(), msg)
	if err != nil {
		return AddPackageResult{}, fmt.Errorf("addpackage: broadcast: %w", err)
	}
	return AddPackageResult{TxHash: hex.EncodeToString(res.Hash), Height: res.Height, GasUsed: res.DeliverTx.GasUsed}, nil
}

// defaultBaseTxCfg returns the gas/fee defaults for write txs.
// Chain requires 1ugnot per 1000 gas; padded to 10000000ugnot @ 10M gas.
func defaultBaseTxCfg() gnoclient.BaseTxCfg {
	return gnoclient.BaseTxCfg{
		GasFee:    fmt.Sprintf("%dugnot", DefaultGasFeeUgnot),
		GasWanted: 10_000_000,
	}
}

// spendRemaining returns limit - used, dropping any zero/negative denoms.
func spendRemaining(limit, used std.Coins) std.Coins {
	diff := limit.SubUnsafe(used)
	out := make(std.Coins, 0, len(diff))
	for _, c := range diff {
		if c.Amount > 0 {
			out = append(out, c)
		}
	}
	return out
}
