package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func main() {
	var (
		realmsDir    = flag.String("realms", "/opt/e2e/realms", "realm tree to deploy at genesis")
		chainID      = flag.String("chain-id", "test9999", "chain id (must match ^test-?\\d+$)")
		rpcListen    = flag.String("rpc-listen", "tcp://127.0.0.1:26687", "node RPC listen address")
		faucetListen = flag.String("faucet-listen", "127.0.0.1:8590", "faucet HTTP listen address")
		webListen    = flag.String("web-listen", "127.0.0.1:8688", "gnoweb HTTP listen address")
		hostname     = flag.String("hostname", "testnet.gnomcp.sim", "alias hostname advertised in gnoconnect meta-tags")
		grant        = flag.Int64("grant", 1_000_000_000, "faucet grant per request, in ugnot (default 1000 GNOT: gas fees are 10M, storage deposits caller-paid — a grant must fund several of each)")
		claDir       = flag.String("cla-dir", "", "path to a gno.land/r/sys/cla package dir to seed the CLA deploy gate (empty = gate off); deps resolve from GNOROOT/examples")
		claHash      = flag.String("cla-hash", "", "required CLA hash to activate enforcement (empty = realm deployed but gate disabled)")
	)
	flag.Parse()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	n, rpcAddr, err := Boot(logger, Config{
		RealmsDir: *realmsDir, ChainID: *chainID, RPCListen: *rpcListen,
		CLAPkgDir: *claDir, CLARequiredHash: *claHash,
	})
	if err != nil {
		logger.Error("simnet: node boot failed", "err", err)
		os.Exit(1)
	}
	logger.Info("simnet: node ready", "rpc", rpcAddr, "chain-id", *chainID)

	faucetAddr, stopFaucet, err := StartFaucet(logger, FaucetConfig{
		RPCAddr: rpcAddr, ChainID: *chainID, Listen: *faucetListen, GrantUgnot: *grant,
	})
	if err != nil {
		logger.Error("simnet: faucet failed", "err", err)
		os.Exit(1)
	}
	logger.Info("simnet: faucet ready", "listen", faucetAddr)

	// The advertised RPC swaps the bind host for the alias: the AUT and
	// gno_profile_add must see a remote-looking URL, never a loopback one.
	rpcPort := rpcAddr[strings.LastIndex(rpcAddr, ":")+1:]
	advertised := fmt.Sprintf("http://%s:%s", *hostname, rpcPort)
	webAddr, stopWeb, err := StartGnoweb(logger, GnowebConfig{
		NodeRPC: rpcAddr, AdvertisedRPC: advertised, ChainID: *chainID, Listen: *webListen,
	})
	if err != nil {
		logger.Error("simnet: gnoweb failed", "err", err)
		os.Exit(1)
	}
	logger.Info("simnet: gnoweb ready", "listen", webAddr, "advertises", advertised)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	logger.Info("simnet: shutting down")
	stopWeb()
	stopFaucet()
	_ = n.Stop()
}
