// Command simnet runs a simulated testnet for the playground e2e harness:
// a real in-memory gnoland node (BFT + mempool + tm2 RPC) genesis-seeded
// with the e2e test realms, plus a faucet and a gnoweb front. It exists so
// faucet/write/connect agent flows are deterministic local-tier scenarios.
package main

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/gnolang/gno/gno.land/pkg/gnoland"
	"github.com/gnolang/gno/gno.land/pkg/gnoland/ugnot"
	"github.com/gnolang/gno/gno.land/pkg/integration"
	"github.com/gnolang/gno/gno.land/pkg/sdk/vm"
	"github.com/gnolang/gno/gnovm/pkg/gnoenv"
	"github.com/gnolang/gno/tm2/pkg/bft/node"
	"github.com/gnolang/gno/tm2/pkg/crypto"
	"github.com/gnolang/gno/tm2/pkg/std"

	"github.com/gnoverse/gno-mcp/internal/keystore"
)

// claPkgPath is the canonical CLA realm path the keeper queries
// (vm.params sysCLAPkgDefault). The seeded realm must deploy here for the gate
// to bind.
const claPkgPath = "gno.land/r/sys/cla"

// Config carries the node half of simnet. ChainID must be on gnomcp's
// writable testnet list (profiles.IsTestnetChainID); RPCListen ":0" lets the
// kernel pick a free port, which is required for concurrent test runs.
type Config struct {
	RealmsDir string // tree mirroring pkg paths (gno.land/r/test/...), each leaf has gnomod.toml
	ChainID   string
	RPCListen string // e.g. tcp://127.0.0.1:26687; ":0" or "tcp://127.0.0.1:0" for tests

	// CLAPkgDir, when set, seeds the gno.land/r/sys/cla realm from this on-disk
	// package directory so deploys hit the same CLA gate the live networks
	// enforce. Its render/addrset deps resolve from GNOROOT/examples. Empty
	// leaves the gate open (default simnet behaviour).
	CLAPkgDir string
	// CLARequiredHash activates CLA enforcement: a final genesis tx calls
	// cla.SetRequiredHash with this value, after every package is deployed.
	// Empty deploys the realm but leaves enforcement disabled.
	CLARequiredHash string
}

// Boot starts the in-memory node with the realms deployed at genesis by
// test1 (premined by DefaultTestingGenesisConfig). Returns the node and its
// actual RPC listen address.
//
// Genesis txs are signed against the default "tendermint_test" chain-id
// inside GenerateTxs; SkipGenesisSigVerification bypasses that mismatch so
// we can use an arbitrary ChainID for the running node.
func Boot(logger *slog.Logger, c Config) (*node.Node, string, error) {
	cfg := integration.TestingMinimalNodeConfig(gnoenv.RootDir())
	cfg.TMConfig.RPC.ListenAddress = c.RPCListen
	cfg.Genesis.ChainID = c.ChainID
	cfg.SkipGenesisSigVerification = true

	loader := integration.NewPkgsLoader()
	// Seed the CLA realm first: registering its package name before the test
	// realms load means LoadAllPackagesFromDir won't double-add it, and its
	// render/addrset deps resolve from the examples tree.
	if c.CLAPkgDir != "" {
		examplesRoot := filepath.Join(gnoenv.RootDir(), "examples")
		// Empty name → LoadPackage reads the package's gnomod + imports and
		// resolves the dependency closure (addrset, render deps) from
		// examplesRoot. Passing a non-empty name skips that import scan.
		if err := loader.LoadPackage(examplesRoot, c.CLAPkgDir, ""); err != nil {
			return nil, "", fmt.Errorf("load cla realm from %q: %w", c.CLAPkgDir, err)
		}
	}
	if err := loader.LoadAllPackagesFromDir(c.RealmsDir); err != nil {
		return nil, "", fmt.Errorf("load realms from %q: %w", c.RealmsDir, err)
	}
	key, err := integration.GeneratePrivKeyFromMnemonic(keystore.Test1Mnemonic, "", 0, 0)
	if err != nil {
		return nil, "", fmt.Errorf("derive test1 key: %w", err)
	}
	fee := std.NewFee(500000, std.MustParseCoin(ugnot.ValueString(1_000_000)))
	txs, err := loader.GenerateTxs(key, fee, nil)
	if err != nil {
		return nil, "", fmt.Errorf("generate genesis txs: %w", err)
	}
	state := cfg.Genesis.AppState.(gnoland.GnoGenesisState)
	state.Txs = append(state.Txs, txs...)
	// Activate the gate as the final genesis tx: every package above deployed
	// with requiredHash still empty (dormant gate), so none were blocked; this
	// tx sets the hash, gating runtime deploys until the caller signs the CLA.
	if c.CLAPkgDir != "" && c.CLARequiredHash != "" {
		actTx, err := claActivationTx(key, c.CLARequiredHash, fee)
		if err != nil {
			return nil, "", fmt.Errorf("build cla activation tx: %w", err)
		}
		state.Txs = append(state.Txs, actTx)
	}
	cfg.Genesis.AppState = state

	n, err := gnoland.NewInMemoryNode(logger, cfg)
	if err != nil {
		return nil, "", fmt.Errorf("create node: %w", err)
	}
	if err := n.Start(); err != nil {
		return nil, "", fmt.Errorf("start node: %w", err)
	}
	select {
	case <-n.Ready():
	case <-time.After(15 * time.Second):
		_ = n.Stop()
		return nil, "", fmt.Errorf("node not ready after 15s")
	}
	return n, n.Config().RPC.ListenAddress, nil
}

// claActivationTx builds the signed genesis tx that activates the CLA gate by
// calling cla.SetRequiredHash(hash) as the deployer (test1, the realm's
// genesis admin). Signed against "tendermint_test" to match GenerateTxs;
// SkipGenesisSigVerification makes the chain-id moot.
func claActivationTx(key crypto.PrivKey, hash string, fee std.Fee) (gnoland.TxWithMetadata, error) {
	msg := vm.NewMsgCall(key.PubKey().Address(), nil, claPkgPath, "SetRequiredHash", []string{hash})
	txs := []gnoland.TxWithMetadata{{Tx: std.Tx{Msgs: []std.Msg{msg}, Fee: fee}}}
	if err := gnoland.SignGenesisTxs(txs, key, "tendermint_test"); err != nil {
		return gnoland.TxWithMetadata{}, err
	}
	return txs[0], nil
}
