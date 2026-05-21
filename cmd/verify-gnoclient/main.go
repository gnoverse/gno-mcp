// cmd/verify-gnoclient/main.go
//
// Verification stub for gnoclient API used by Milestone B Real impls.
// Run: go run ./cmd/verify-gnoclient
package main

import (
	"fmt"

	abci "github.com/gnolang/gno/tm2/pkg/bft/abci/types"
	ctypes "github.com/gnolang/gno/tm2/pkg/bft/rpc/core/types"
	"github.com/gnolang/gno/tm2/pkg/std"

	"github.com/gnolang/gno/gno.land/pkg/gnoclient"
	"github.com/gnolang/gno/gno.land/pkg/sdk/vm"
)

func main() {
	// ---- Client struct fields
	var c *gnoclient.Client
	_ = c

	// ---- BaseTxCfg fields (no CallCfg/RunCfg — gnoclient uses BaseTxCfg for both)
	var base gnoclient.BaseTxCfg
	_ = base.GasFee
	_ = base.GasWanted
	_ = base.AccountNumber
	_ = base.SequenceNumber
	_ = base.Memo

	// ---- Signer interface (gnoclient.Signer, not keys.Signer)
	var signer gnoclient.Signer
	_ = signer

	// ---- SignerFromKeybase (concrete impl)
	var sfk gnoclient.SignerFromKeybase
	_ = sfk

	// ---- Call signature: (*Client).Call(cfg BaseTxCfg, msgs ...vm.MsgCall) (*ctypes.ResultBroadcastTxCommit, error)
	var msgCall vm.MsgCall
	_ = msgCall

	// ---- Run signature: (*Client).Run(cfg BaseTxCfg, msgs ...vm.MsgRun) (*ctypes.ResultBroadcastTxCommit, error)
	var msgRun vm.MsgRun
	_ = msgRun

	// ---- Simulate: separate (*Client).Simulate(tx *std.Tx) (*abci.ResponseDeliverTx, error)
	var tx std.Tx
	_ = tx
	deliverTx := &abci.ResponseDeliverTx{}
	_ = deliverTx.GasUsed
	_ = deliverTx.GasWanted

	// ---- ResultBroadcastTxCommit fields used by Real.Call/Run
	res := &ctypes.ResultBroadcastTxCommit{}
	_ = res.Hash
	_ = res.Height
	_ = res.DeliverTx.GasUsed
	_ = res.DeliverTx.Data

	fmt.Println("gnoclient verification: compiles OK")
	fmt.Println("")
	fmt.Println("Key findings:")
	fmt.Println("  - No CallCfg/RunCfg types: gnoclient uses BaseTxCfg for both Call and Run")
	fmt.Println("  - Simulate: separate (*Client).Simulate(*std.Tx) (*abci.ResponseDeliverTx, error)")
	fmt.Println("  - No ABCI session query path in gnoclient v1.1.0 (HARD BLOCKER for Real.QuerySession)")
	fmt.Println("  - gnoclient.Signer != keys.Signer (different interfaces)")
	fmt.Println("  - bech32 address prefix: g1, pubkey prefix: gpub (from tm2/pkg/crypto/globals.go)")
}
