// Command gnoquery is the e2e harness's chain ground-truth oracle: it reads
// on-chain state directly via gnoclient (the same pinned library gnomcp wraps,
// but called below gnomcp's tool layer, so it's an independent check). It exists
// to verify what an agent-under-test claims actually landed on the simnet chain.
//
// Usage:
//
//	gnoquery [-rpc URL] status
//	gnoquery [-rpc URL] height
//	gnoquery [-rpc URL] render <pkgpath> [path]
//	gnoquery [-rpc URL] eval   <pkgpath> <expr>
//	gnoquery [-rpc URL] balance <bech32-addr>
//
// Exit codes: 0 ok, 1 RPC/query error (e.g. realm not found), 2 usage error.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	gnoclient "github.com/gnolang/gno/gno.land/pkg/gnoclient"
	rpcclient "github.com/gnolang/gno/tm2/pkg/bft/rpc/client"
	"github.com/gnolang/gno/tm2/pkg/crypto"
)

// defaultRPC is the in-container simnet node; override with -rpc.
const defaultRPC = "http://testnet.gnomcp.sim:26687"

const usage = `usage: gnoquery [-rpc URL] <command>
  status                       chain-id and latest height
  height                       latest block height
  render <pkgpath> [path]      a realm's Render(path) output
  eval   <pkgpath> <expr>      evaluate an expression (e.g. "Total()")
  balance <bech32-addr>        an account's coins`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(argv []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gnoquery", flag.ContinueOnError)
	fs.SetOutput(stderr)
	rpc := fs.String("rpc", defaultRPC, "node RPC URL")
	if err := fs.Parse(argv); err != nil {
		return 2
	}
	args := fs.Args()
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return 2
	}

	switch args[0] {
	case "status":
		return withClient(*rpc, stderr, func(c *gnoclient.Client) int {
			st, err := c.RPCClient.Status(context.Background(), nil)
			if err != nil {
				return fail(stderr, err)
			}
			fmt.Fprintf(stdout, "chain-id: %s\nheight: %d\n", st.NodeInfo.Network, st.SyncInfo.LatestBlockHeight)
			return 0
		})

	case "height":
		return withClient(*rpc, stderr, func(c *gnoclient.Client) int {
			st, err := c.RPCClient.Status(context.Background(), nil)
			if err != nil {
				return fail(stderr, err)
			}
			fmt.Fprintln(stdout, st.SyncInfo.LatestBlockHeight)
			return 0
		})

	case "render":
		if len(args) < 2 {
			fmt.Fprintln(stderr, "usage: gnoquery render <pkgpath> [path]")
			return 2
		}
		path := ""
		if len(args) > 2 {
			path = args[2]
		}
		return withClient(*rpc, stderr, func(c *gnoclient.Client) int {
			out, _, err := c.Render(args[1], path)
			return emit(stdout, stderr, out, err)
		})

	case "eval":
		if len(args) < 3 {
			fmt.Fprintln(stderr, "usage: gnoquery eval <pkgpath> <expr>")
			return 2
		}
		return withClient(*rpc, stderr, func(c *gnoclient.Client) int {
			out, _, err := c.QEval(args[1], args[2])
			return emit(stdout, stderr, out, err)
		})

	case "balance":
		if len(args) < 2 {
			fmt.Fprintln(stderr, "usage: gnoquery balance <bech32-addr>")
			return 2
		}
		addr, err := crypto.AddressFromBech32(args[1])
		if err != nil {
			fmt.Fprintf(stderr, "gnoquery: invalid address %q: %v\n", args[1], err)
			return 2
		}
		return withClient(*rpc, stderr, func(c *gnoclient.Client) int {
			acc, _, err := c.QueryAccount(addr)
			if err != nil {
				return fail(stderr, err)
			}
			if acc == nil {
				fmt.Fprintln(stdout, "(account not found)")
				return 1
			}
			fmt.Fprintln(stdout, acc.GetCoins().String())
			return 0
		})

	default:
		fmt.Fprintf(stderr, "gnoquery: unknown command %q\n%s\n", args[0], usage)
		return 2
	}
}

// withClient dials the RPC and runs fn with a query-only client (no signer).
func withClient(rpcURL string, stderr io.Writer, fn func(*gnoclient.Client) int) int {
	rpc, err := rpcclient.NewHTTPClient(rpcURL)
	if err != nil {
		return fail(stderr, err)
	}
	return fn(&gnoclient.Client{RPCClient: rpc})
}

// emit prints a query result, or the error to stderr with exit 1.
func emit(stdout, stderr io.Writer, out string, err error) int {
	if err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintln(stdout, out)
	return 0
}

func fail(stderr io.Writer, err error) int {
	fmt.Fprintln(stderr, "gnoquery:", err)
	return 1
}
