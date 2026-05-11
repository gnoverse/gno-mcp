package main

import (
	"fmt"
	"os"

	"github.com/gnolang/gno-mcp/internal/audit"
	"github.com/gnolang/gno-mcp/internal/client"
	"github.com/gnolang/gno-mcp/internal/mcp"
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
	return mcp.New(client.NewFake(), a).ServeStdio()
}
