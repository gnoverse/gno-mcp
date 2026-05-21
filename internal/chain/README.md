# chain package — verified gnoclient API references

Captured during Milestone B Task 2.3 against `github.com/gnolang/gno v1.1.0`.
Update this file if gno dependencies change.

## Call / Run signatures

Verified from `go doc -all github.com/gnolang/gno/gno.land/pkg/gnoclient`:

```
func (c *Client) Call(cfg BaseTxCfg, msgs ...vm.MsgCall) (*ctypes.ResultBroadcastTxCommit, error)
func (c *Client) Run(cfg BaseTxCfg, msgs ...vm.MsgRun)  (*ctypes.ResultBroadcastTxCommit, error)
```

There is NO `CallCfg` or `RunCfg` type. Both methods share `BaseTxCfg`:

```go
type BaseTxCfg struct {
    GasFee         string // e.g. "1ugnot"
    GasWanted      int64  // e.g. 5_000_000
    AccountNumber  uint64 // 0 = auto-query from chain
    SequenceNumber uint64 // 0 = auto-query from chain
    Memo           string
}
```

`MsgCall` fields used by Real.Call:

```go
type MsgCall struct {
    Caller  crypto.Address // derived from Signer.Info().GetAddress()
    Send    std.Coins      // nil/empty for most calls
    PkgPath string         // realm path, e.g. "gno.land/r/demo/counter"
    Func    string         // function name, e.g. "Increment"
    Args    []string       // stringified args (gnokey CLI style)
}
```

`MsgRun` fields used by Real.Run:

```go
type MsgRun struct {
    Caller  crypto.Address
    Send    std.Coins
    Package *std.MemPackage // contains MemFile list with the gno source
}
```

`ResultBroadcastTxCommit` fields extracted by Real.Call/Run:

```go
type ResultBroadcastTxCommit struct {
    Hash      []byte
    Height    int64
    CheckTx   abci.ResponseCheckTx
    DeliverTx abci.ResponseDeliverTx  // .GasUsed, .Data
}
```

## Simulate support

`(*Client).Simulate` exists as a **separate method** (option b):

```
func (c *Client) Simulate(tx *std.Tx) (*abci.ResponseDeliverTx, error)
```

Implementation path: `ABCIQuery(ctx, ".app/simulate", amino.Marshal(tx))`.
Returns `*abci.ResponseDeliverTx{GasWanted, GasUsed, ResponseBase{Data, Error, Log}}`.

Decision: **option (b) applies**. Real.Call/Run implement the simulate path by:
1. Build unsigned `std.Tx` via `gnoclient.NewCallTx` / `gnoclient.NewRunTx`.
2. Sign with a zero/placeholder signature (public key only) via `(*Client).SignTx`.
3. Call `(*Client).Simulate(signedTx)`.
4. Return `CallResult{Simulated: true, GasUsed: deliverTx.GasUsed}`.

Keep the `simulate` flag in the tool schema.

## ABCI path for per-pubkey session lookup

**No session ABCI path exists in gnoclient v1.1.0.**

The auth module registers only two query sub-paths (verified in
`tm2/pkg/sdk/auth/handler.go`):

- `auth/accounts/<bech32addr>` — returns `GnoAccount` (JSON)
- `auth/gasprice` — returns current gas price

The vm module registers: `vm/qrender`, `vm/qeval`, `vm/qfile`, `vm/qdoc`,
`vm/qpaths`. No session sub-path.

No session realm exists in the genesis configuration either. There is no
`auth/qsession`, `vm/qsession`, or equivalent path anywhere in the gno
source tree at this version.

**Consequence for Real.QuerySession (Task 2.6):** The method cannot resolve
session status from the chain using an ABCI query. Options at implementation
time:

1. Return `ErrSimulateUnsupported`-style sentinel `ErrSessionQueryUnsupported`
   and let the tool surface `session_query_unsupported`.
2. Query the session realm via `vm/qeval` if the session contract publishes a
   query function by pubkey (requires knowing the realm path at runtime).
3. Return `SessionStatus{Active: false}` (conservative: treat unknown = inactive).

This is flagged as a **chain-side blocker** for a full per-pubkey session
query. The design decision belongs to G. Do NOT implement Real.QuerySession
as if a direct ABCI path exists.

## gnoclient.Signer adapter

`gnoclient.Signer` (NOT `keys.Signer`) is the interface consumed by `Client`:

```go
type gnoclient.Signer interface {
    Sign(SignCfg) (*std.Tx, error)
    Info() (keys.Info, error)    // returns address + pubkey
    Validate() error
}

type gnoclient.SignCfg struct {
    UnsignedTX     std.Tx
    SequenceNumber uint64
    AccountNumber  uint64
}
```

`keys.Info` (from `tm2/pkg/crypto/keys`):

```go
type keys.Info interface {
    GetType()    KeyType
    GetName()    string
    GetPubKey()  crypto.PubKey
    GetAddress() crypto.Address
    GetPath()    (*hd.BIP44Params, error)
}
```

The `chain.Signer` interface (defined in `client.go`) is **not** directly
compatible with `gnoclient.Signer`. The adapter pattern for Real.Call/Run:

- `chain.Signer.Address()` → bech32 string → `crypto.AddressFromBech32()` →
  `MsgCall.Caller` / `MsgRun.Caller`.
- `chain.Signer.Sign(payload)` → raw ed25519 bytes. This does NOT map to
  `gnoclient.Signer.Sign(SignCfg)`, which signs a full `std.Tx`.

Therefore Real.Call/Run must NOT wrap `chain.Signer` as a `gnoclient.Signer`.
Instead they must use `gnoclient.SignerFromBip39` or `gnoclient.SignerFromKeybase`
and inject it into `gnoclient.Client.Signer`. The `chain.Signer` abstraction
provides the address lookup only; the actual tx signing uses the session
keypair material stored on disk (Task 3.x manages this).

Account number + sequence: when `BaseTxCfg.AccountNumber == 0 &&
SequenceNumber == 0`, `(*Client).SignTx` auto-queries the chain via
`QueryAccount`. Pass zeros to enable auto-query (safe default).

## bech32 prefixes

Verified from `tm2/pkg/crypto/globals.go` (defaults set by `setBech32Defaults`):

- Address prefix: `g` → bech32 addresses are `g1...`
- Pubkey prefix: `gpub` → bech32 pubkeys are `gpub1...`

`crypto.PubKeyToBech32(pub)` and `crypto.PubKeyFromBech32(str)` are the
canonical encode/decode functions (amino-marshals the pubkey bytes).

## GasFee / GasWanted defaults

Recommended placeholders (calibrate via e2e):

- `GasFee`: `"1ugnot"`
- `GasWanted`: `5_000_000`
