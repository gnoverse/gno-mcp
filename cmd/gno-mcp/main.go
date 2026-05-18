package main

import (
	"context"
	"fmt"
	"os"

	"github.com/gnolang/gno-mcp/internal/audit"
	"github.com/gnolang/gno-mcp/internal/client"
	"github.com/gnolang/gno-mcp/internal/mcp"
	"github.com/gnolang/gno-mcp/internal/session"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gno-mcp:", err)
		os.Exit(1)
	}
}

func run() error {
	a, err := audit.Default()
	if err != nil {
		return err
	}

	c := client.NewFake()

	// Network defaults to staging.gno.land in v0.2 because the demo session
	// can't be funded on mainnet without explicit user authorization — we want
	// the friendlier-failure-mode network as the default.
	network := os.Getenv("GNO_MCP_NETWORK")
	if network == "" {
		network = "staging.gno.land"
	}

	sess := session.New(session.Options{
		Network: network,
		Balance: func(ctx context.Context, network, addr string) (int64, error) {
			info, err := c.AddressInfo(ctx, network, addr)
			if err != nil {
				return 0, err
			}
			var ugnot int64
			for _, ch := range info.Balance {
				if ch < '0' || ch > '9' {
					break
				}
				ugnot = ugnot*10 + int64(ch-'0')
			}
			return ugnot, nil
		},
	})

	return mcp.NewWithSession(c, a, sess).ServeStdio()
}
