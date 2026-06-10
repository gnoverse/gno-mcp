package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	gnoclient "github.com/gnolang/gno/gno.land/pkg/gnoclient"
	rpcclient "github.com/gnolang/gno/tm2/pkg/bft/rpc/client"

	"github.com/gnoverse/gno-mcp/faucet"
	"github.com/gnoverse/gno-mcp/internal/chain"
)

func main() {
	rpcURL := flag.String("rpc-url", "", "gno node RPC URL (required)")
	chainID := flag.String("chain-id", "", "chain-id of the target testnet (required)")
	// The mnemonic is NOT a flag default: a non-empty string default is printed by
	// -help and on any flag error, leaking the funding key to stderr/logs. The env
	// var is read after parsing as a fallback when -mnemonic is unset; prefer the
	// env var in practice, since argv is visible to ps and shell history.
	mnemonic := flag.String("mnemonic", "", "BIP-39 mnemonic for the funding key (default: $GNOMCP_FAUCET_MNEMONIC)")
	listen := flag.String("listen", "127.0.0.1:8590", "address to listen on")
	grant := flag.Int64("grant", 1_000_000_000, "ugnot amount per drip")
	perAddrCooldown := flag.Duration("per-addr-cooldown", 24*time.Hour, "cooldown window between grants to the same address")
	perIPMax := flag.Int("per-ip-max", 5, "max grants per IP per per-ip-window")
	perIPWindow := flag.Duration("per-ip-window", time.Hour, "sliding window for per-IP rate limiting")
	dailyCap := flag.Int64("daily-cap", 100_000_000_000, "hard daily ugnot outflow cap")
	flag.Parse()

	if *mnemonic == "" {
		*mnemonic = os.Getenv("GNOMCP_FAUCET_MNEMONIC")
	}
	if *rpcURL == "" || *chainID == "" || *mnemonic == "" {
		log.Fatal("agentfaucet: -rpc-url, -chain-id, and -mnemonic (or GNOMCP_FAUCET_MNEMONIC) are required")
	}
	if !faucet.IsTestnetChainID(*chainID) {
		log.Fatalf("agentfaucet: chain-id %q is not a testnet (only test* is allowed)", *chainID)
	}

	signer, err := gnoclient.SignerFromBip39(*mnemonic, *chainID, "", 0, 0)
	if err != nil {
		log.Fatalf("agentfaucet: build signer: %v", err)
	}

	info, err := signer.Info()
	if err != nil {
		log.Fatalf("agentfaucet: signer info: %v", err)
	}

	rpc, err := rpcclient.NewHTTPClient(*rpcURL)
	if err != nil {
		log.Fatalf("agentfaucet: rpc client: %v", err)
	}

	cli := &gnoclient.Client{RPCClient: rpc, Signer: signer}

	disp := faucet.NewGnoclientDispenser(cli, info.GetAddress(),
		fmt.Sprintf("%dugnot", chain.DefaultGasFeeUgnot), chain.DefaultGasWanted)
	lim := faucet.NewLimiter(faucet.LimiterCfg{
		PerAddrWindow: *perAddrCooldown,
		PerAddrMax:    1,
		PerIPWindow:   *perIPWindow,
		PerIPMax:      *perIPMax,
		DailyCapUgnot: *dailyCap,
		GrantUgnot:    *grant,
	})
	f := faucet.New(*chainID, *grant, disp, lim)

	log.Printf("agentfaucet: chain=%s listen=%s from=%s grant=%d", *chainID, *listen, info.GetAddress(), *grant)
	srv := &http.Server{
		Addr:         *listen,
		Handler:      f.Handler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-serveErr:
		log.Fatalf("agentfaucet: serve: %v", err)
	case sig := <-stop:
		log.Printf("agentfaucet: %s received, shutting down", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("agentfaucet: graceful shutdown failed: %v", err)
		}
	}
}
