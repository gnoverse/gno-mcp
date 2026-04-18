package main

import (
	"fmt"
	"os"

	"github.com/gnolang/gno-mcp/internal/mcp"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gno-mcp:", err)
		os.Exit(1)
	}
}

func run() error {
	return mcp.New().ServeStdio()
}
