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
)

// version is overridden at release time via -ldflags "-X main.version=...";
// dev builds report "dev".
var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println(version)
		return
	}

	rpcURL := flag.String("rpc-url", "", "gno node RPC URL (required)")
	chainID := flag.String("chain-id", "", "chain-id of the target testnet (required)")
	// The mnemonic is NOT a flag default: a non-empty string default is printed by
	// -help and on any flag error, leaking the funding key to stderr/logs. The env
	// var is read after parsing as a fallback when -mnemonic is unset; prefer the
	// env var in practice, since argv is visible to ps and shell history.
	mnemonic := flag.String("mnemonic", "", "BIP-39 mnemonic for the funding key (default: $GNOMCP_FAUCET_MNEMONIC)")
	listen := flag.String("listen", "127.0.0.1:8590", "address to listen on")
	grant := flag.Int64("grant", 1_000_000_000, "ugnot amount granted per fund request")
	// Gas is scoped to the faucet's only tx type — a bank send (~1.6M on test-13).
	// Defaults mirror the gno faucet (1 GNOT fee, 5M wanted), far below the gnomcp
	// write default that is sized for deploys; a faucet never deploys.
	gasFee := flag.String("gas-fee", "1000000ugnot", "gas fee per dispense tx (e.g. 1000000ugnot)")
	gasWanted := flag.Int64("gas-wanted", 5_000_000, "gas limit per dispense tx")
	perAddrCooldown := flag.Duration("per-addr-cooldown", 24*time.Hour, "cooldown window between grants to the same address")
	perIPMax := flag.Int("per-ip-max", 60, "coarse anti-hammer guard: max grants per IP per per-ip-window (not a token-safety control — the faucet is typically called server-side, so all requests share one egress IP)")
	perIPWindow := flag.Duration("per-ip-window", time.Hour, "sliding window for the per-IP anti-hammer guard")
	dailyCap := flag.Int64("daily-cap", 100_000_000_000, "hard daily ugnot outflow cap")
	dripBurst := flag.Int64("drip-burst", 0, "global outflow token-bucket capacity in ugnot — the largest burst tolerated; this is the master switch (0 disables the drip control entirely)")
	dripRate := flag.Int64("drip-rate", 0, "global outflow token-bucket refill, ugnot/sec; inert unless drip-burst is set (daily-cap still applies)")
	minFundingBalance := flag.Int64("min-funding-balance", 0, "refuse grants (503) while the funding wallet holds fewer ugnot than this (0 disables)")
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

	disp := faucet.NewGnoclientDispenser(cli, info.GetAddress(), *gasFee, *gasWanted)
	lim := faucet.NewLimiter(faucet.LimiterCfg{
		PerAddrWindow:       *perAddrCooldown,
		PerAddrMax:          1,
		PerIPWindow:         *perIPWindow,
		PerIPMax:            *perIPMax,
		DailyCapUgnot:       *dailyCap,
		GrantUgnot:          *grant,
		DripBurstUgnot:      *dripBurst,
		DripRateUgnotPerSec: *dripRate,
	})

	var opts []faucet.Option
	if *minFundingBalance > 0 {
		opts = append(opts, faucet.WithBalanceFloor(*minFundingBalance,
			faucet.NewGnoclientBalance(cli, info.GetAddress(), 30*time.Second)))
	}
	f := faucet.New(*chainID, *grant, disp, lim, opts...)

	log.Printf("agentfaucet: chain=%s listen=%s from=%s grant=%d", *chainID, *listen, info.GetAddress(), *grant)
	// Dispenses are serialized (gnoclient signs against a queried account
	// sequence), so concurrent /fund requests queue behind one chain round-trip
	// each. WriteTimeout must comfortably exceed worst-case queue-depth × send
	// latency; a request that blows it still completes its dispense on-chain
	// (Send ignores ctx) but delivers no response — sized at 30s for low
	// concurrency.
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
