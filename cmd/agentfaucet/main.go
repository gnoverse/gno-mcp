package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	gnoclient "github.com/gnolang/gno/gno.land/pkg/gnoclient"
	rpcclient "github.com/gnolang/gno/tm2/pkg/bft/rpc/client"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	otelmetric "go.opentelemetry.io/otel/metric"

	"github.com/gnoverse/gno-mcp/faucet"
)

// version is overridden at release time via -ldflags "-X main.version=...";
// dev builds report "dev".
var version = "dev"

// balanceTTL is the funding-balance cache/poll interval, shared by the optional
// balance floor and the funding-balance gauge poller.
const balanceTTL = 30 * time.Second

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
	metricsAddr := flag.String("metrics-addr", "", "address for the Prometheus /metrics listener, e.g. 127.0.0.1:8591 (empty disables; opt-in)")
	trustedProxies := flag.Int("trusted-proxies", 0, "number of trusted reverse-proxy hops for X-Forwarded-For client IP (0 = ignore XFF, use direct peer; set to the proxy count, e.g. 1 behind a single ALB)")
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

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	fatal := func(msg string, args ...any) {
		logger.Error(msg, args...)
		os.Exit(1)
	}

	if *mnemonic == "" {
		*mnemonic = os.Getenv("GNOMCP_FAUCET_MNEMONIC")
	}
	if *rpcURL == "" || *chainID == "" || *mnemonic == "" {
		fatal("missing required config", "need", "-rpc-url, -chain-id, and -mnemonic (or GNOMCP_FAUCET_MNEMONIC)")
	}
	if !faucet.IsTestnetChainID(*chainID) {
		fatal("chain-id is not a testnet (only test* is allowed)", "chain_id", *chainID)
	}

	signer, err := gnoclient.SignerFromBip39(*mnemonic, *chainID, "", 0, 0)
	if err != nil {
		fatal("build signer", "err", err.Error())
	}

	info, err := signer.Info()
	if err != nil {
		fatal("signer info", "err", err.Error())
	}

	rpc, err := rpcclient.NewHTTPClient(*rpcURL)
	if err != nil {
		fatal("rpc client", "err", err.Error())
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

	// The funding-balance source is always built so the balance gauge can observe
	// it; the optional floor reuses the same TTL-cached query.
	balanceFn := faucet.NewGnoclientBalance(cli, info.GetAddress(), balanceTTL)

	opts := []faucet.Option{
		faucet.WithLogger(logger),
		faucet.WithTrustedProxies(*trustedProxies),
	}
	if *minFundingBalance > 0 {
		opts = append(opts, faucet.WithBalanceFloor(*minFundingBalance, balanceFn))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// mp is the meter provider for HTTP server metrics; nil leaves the handler
	// uninstrumented when metrics are disabled.
	var mp otelmetric.MeterProvider
	if *metricsAddr != "" {
		tel, err := setupTelemetry(ctx, version)
		if err != nil {
			fatal("setup telemetry", "err", err.Error())
		}
		defer func() { _ = tel.shutdown(context.Background()) }()
		mp = tel.provider

		// -1 sentinel: the balance gauge stays unset until the first successful
		// poll, so an unreachable RPC at startup reads as "no data" rather than a
		// misleading 0 (empty wallet).
		var balanceGauge atomic.Int64
		balanceGauge.Store(-1)
		if err := tel.registerGauges(lim, &balanceGauge); err != nil {
			fatal("register gauges", "err", err.Error())
		}
		opts = append(opts, faucet.WithMetrics(tel.metrics))

		// Poll the funding balance on a TTL so the gauge callback only reads an
		// atomic and never blocks a scrape on an RPC.
		go pollBalance(ctx, balanceFn, &balanceGauge, balanceTTL, logger)

		// Bind the metrics listener synchronously so a port conflict fails loudly
		// at startup (like the main listener) instead of logging from a goroutine
		// while the faucet runs on, blind, without metrics.
		metricsLn, err := net.Listen("tcp", *metricsAddr)
		if err != nil {
			fatal("metrics listen", "addr", *metricsAddr, "err", err.Error())
		}
		metricsSrv := &http.Server{Handler: metricsMux(tel.handler), ReadTimeout: 5 * time.Second}
		go func() {
			logger.Info("metrics listening", "addr", *metricsAddr)
			if err := metricsSrv.Serve(metricsLn); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Error("metrics serve", "err", err.Error())
			}
		}()
		defer func() {
			sctx, scancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer scancel()
			_ = metricsSrv.Shutdown(sctx)
		}()
	}

	f := faucet.New(*chainID, *grant, disp, lim, opts...)

	// otelhttp records HTTP server metrics (request duration, body sizes) against
	// our meter provider; it would otherwise use the global no-op provider. mp is
	// nil when metrics are disabled, leaving the handler bare.
	var serverOpts []otelhttp.Option
	if mp != nil {
		serverOpts = append(serverOpts, otelhttp.WithMeterProvider(mp))
	}

	logger.Info("starting", "chain", *chainID, "listen", *listen, "from", info.GetAddress().String(), "grant", *grant)
	// Dispenses are serialized (gnoclient signs against a queried account
	// sequence), so concurrent /fund requests queue behind one chain round-trip
	// each. WriteTimeout must comfortably exceed worst-case queue-depth × send
	// latency; a request that blows it still completes its dispense on-chain
	// (Send ignores ctx) but delivers no response — sized at 30s for low
	// concurrency.
	srv := &http.Server{
		Addr:         *listen,
		Handler:      otelhttp.NewHandler(f.Handler(), "agentfaucet", serverOpts...),
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

	select {
	case err := <-serveErr:
		fatal("serve", "err", err.Error())
	case <-ctx.Done():
		logger.Info("shutdown signal received")
		sctx, scancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer scancel()
		if err := srv.Shutdown(sctx); err != nil {
			logger.Error("graceful shutdown failed", "err", err.Error())
		}
	}
}

// metricsMux serves the Prometheus handler at /metrics.
func metricsMux(h http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", h)
	return mux
}

// pollBalance refreshes balance into dst every interval until ctx is done, so
// the balance gauge reads a fresh atomic without an RPC on the scrape path.
func pollBalance(ctx context.Context, balance func(context.Context) (int64, error), dst *atomic.Int64, interval time.Duration, logger *slog.Logger) {
	tick := func() {
		v, err := balance(ctx)
		if err != nil {
			logger.Warn("funding balance poll failed", "err", err.Error())
			return
		}
		dst.Store(v)
	}
	tick() // seed immediately
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			tick()
		}
	}
}
