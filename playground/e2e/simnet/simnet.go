// Command simnet runs a simulated testnet for the playground e2e harness:
// a real in-memory gnoland node (BFT + mempool + tm2 RPC) genesis-seeded
// with the e2e test realms, plus a faucet and a gnoweb front. It exists so
// faucet/write/connect agent flows are deterministic local-tier scenarios.
package main

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/gnolang/gno/gno.land/pkg/gnoland"
	"github.com/gnolang/gno/gno.land/pkg/gnoland/ugnot"
	"github.com/gnolang/gno/gno.land/pkg/integration"
	"github.com/gnolang/gno/gnovm/pkg/gnoenv"
	"github.com/gnolang/gno/tm2/pkg/bft/node"
	"github.com/gnolang/gno/tm2/pkg/std"

	"github.com/gnoverse/gno-mcp/internal/keystore"
)

// Config carries the node half of simnet. ChainID must match the gnomcp
// profile allowlist (^test-?\d+$ pattern); RPCListen ":0" lets the kernel
// pick a free port, which is required for concurrent test runs.
type Config struct {
	RealmsDir string // tree mirroring pkg paths (gno.land/r/test/...), each leaf has gnomod.toml
	ChainID   string
	RPCListen string // e.g. tcp://127.0.0.1:26687; ":0" or "tcp://127.0.0.1:0" for tests
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
