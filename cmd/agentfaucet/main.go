package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	gnoclient "github.com/gnolang/gno/gno.land/pkg/gnoclient"
	rpcclient "github.com/gnolang/gno/tm2/pkg/bft/rpc/client"

	"github.com/gnoverse/gno-mcp/faucet"
)

func main() {
	rpcURL := flag.String("rpc-url", "", "gno node RPC URL (required)")
	chainID := flag.String("chain-id", "", "chain-id of the target testnet (required)")
	mnemonic := flag.String("mnemonic", os.Getenv("GNOMCP_FAUCET_MNEMONIC"), "BIP-39 mnemonic for the funding key (or GNOMCP_FAUCET_MNEMONIC)")
	listen := flag.String("listen", "127.0.0.1:8590", "address to listen on")
	grant := flag.Int64("grant", 1_000_000_000, "ugnot amount per drip")
	perAddrCooldown := flag.Duration("per-addr-cooldown", 24*time.Hour, "cooldown window between grants to the same address")
	perIPMax := flag.Int("per-ip-max", 5, "max grants per IP per per-ip-window")
	perIPWindow := flag.Duration("per-ip-window", time.Hour, "sliding window for per-IP rate limiting")
	dailyCap := flag.Int64("daily-cap", 100_000_000_000, "hard daily ugnot outflow cap")
	flag.Parse()

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

	disp := faucet.NewGnoclientDispenser(cli, info.GetAddress(), "10000000ugnot", 10_000_000)
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
	log.Fatal(srv.ListenAndServe())
}
