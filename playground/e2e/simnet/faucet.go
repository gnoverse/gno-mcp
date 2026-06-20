package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	gnoclient "github.com/gnolang/gno/gno.land/pkg/gnoclient"
	rpcclient "github.com/gnolang/gno/tm2/pkg/bft/rpc/client"

	"github.com/gnoverse/gno-mcp/faucet"
	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/keystore"
)

// FaucetConfig carries the faucet half of simnet. The dispenser signs as
// test1 (premined at genesis); limits are generous — isolation comes from
// fresh containers per run, not from rationing.
type FaucetConfig struct {
	RPCAddr    string // node RPC address (from Boot)
	ChainID    string
	Listen     string // e.g. "127.0.0.1:8590"; "127.0.0.1:0" for tests
	GrantUgnot int64
	PerAddrMax int // grants per address per 24h; 0 -> 5 (generous default)
}

// StartFaucet starts an HTTP faucet serving POST /fund on cfg.Listen.
// The dispenser funds from the premined test1 account. Returns the actual
// listen address and a shutdown function.
func StartFaucet(logger *slog.Logger, cfg FaucetConfig) (string, func(), error) {
	rpc, err := rpcclient.NewHTTPClient(cfg.RPCAddr)
	if err != nil {
		return "", nil, fmt.Errorf("faucet rpc client: %w", err)
	}
	signer, err := gnoclient.SignerFromBip39(keystore.Test1Mnemonic, cfg.ChainID, "", 0, 0)
	if err != nil {
		return "", nil, fmt.Errorf("faucet signer: %w", err)
	}
	info, err := signer.Info()
	if err != nil {
		return "", nil, fmt.Errorf("faucet signer info: %w", err)
	}
	cli := &gnoclient.Client{RPCClient: rpc, Signer: signer}
	disp := faucet.NewGnoclientDispenser(cli, info.GetAddress(),
		fmt.Sprintf("%dugnot", chain.DefaultGasFeeUgnot), chain.DefaultGasWanted)
	perAddrMax := cfg.PerAddrMax
	if perAddrMax == 0 {
		perAddrMax = 5 // generous default; isolation comes from fresh containers
	}
	f := faucet.New(cfg.ChainID, cfg.GrantUgnot, disp, faucet.NewLimiter(faucet.LimiterCfg{
		PerAddrMax:    perAddrMax,
		PerIPMax:      100,
		DailyCapUgnot: 1_000_000_000_000,
		GrantUgnot:    cfg.GrantUgnot,
	}))

	ln, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		return "", nil, fmt.Errorf("faucet listen %s: %w", cfg.Listen, err)
	}
	srv := &http.Server{
		Handler:      f.Handler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("faucet server", "err", err)
		}
	}()
	return ln.Addr().String(), func() { _ = srv.Close() }, nil
}
