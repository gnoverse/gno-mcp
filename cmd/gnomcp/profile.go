package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gnoverse/gno-mcp/internal/gnoweb"
	"github.com/gnoverse/gno-mcp/internal/profiles"
)

// reservedNames cannot be redefined by the user — local/testnet are built-in
// defaults; "default" is reserved to avoid ambiguity with config conventions.
var reservedNames = map[string]bool{"local": true, "testnet": true, "default": true}

type profileAddOpts struct {
	FromGnoweb string
	RPC        string
	ChainID    string
	Master     string
	IndexerURL string
}

// globalProfilePath is the file gnomcp profile mutates.
func globalProfilePath() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "gnomcp", "profiles.toml")
	}
	return "profiles.toml"
}

// loadGlobal returns the profiles currently in the global file (empty if absent).
func loadGlobal(path string) (map[string]profiles.Profile, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]profiles.Profile{}, nil
		}
		return nil, err
	}
	defer f.Close()
	cfg, err := profiles.Load(f)
	if err != nil {
		return nil, err
	}
	return cfg.Profiles, nil
}

func profileAdd(path, name string, opts profileAddOpts) error {
	if reservedNames[name] {
		return fmt.Errorf("%q is a reserved built-in profile name", name)
	}
	rpc, chainID := opts.RPC, opts.ChainID
	if opts.FromGnoweb != "" {
		conn, err := gnoweb.Discover(&http.Client{Timeout: 10 * time.Second}, opts.FromGnoweb)
		if err != nil {
			return err
		}
		rpc, chainID = conn.RPC, conn.ChainID
	}
	if rpc == "" || chainID == "" {
		return fmt.Errorf("need --rpc and --chain-id (or --from-gnoweb)")
	}
	cur, err := loadGlobal(path)
	if err != nil {
		return err
	}
	cur[name] = profiles.Profile{
		RPCURL:        rpc,
		ChainID:       chainID,
		MasterAddress: opts.Master,
		TxIndexerURL:  opts.IndexerURL,
	}
	return profiles.WriteFile(path, cur)
}

func profileRemove(path, name string) error {
	if reservedNames[name] {
		return fmt.Errorf("%q is a built-in profile and cannot be removed", name)
	}
	cur, err := loadGlobal(path)
	if err != nil {
		return err
	}
	if _, ok := cur[name]; !ok {
		return fmt.Errorf("profile %q not found in the global config (%s) — gnomcp profile only manages that file, not built-in defaults or a project-local profiles.toml", name, path)
	}
	delete(cur, name)
	return profiles.WriteFile(path, cur)
}

func profileList(path string) error {
	cfg, err := profiles.LoadResolved(profiles.Sources{GlobalPath: path, ProjectPath: "profiles.toml"})
	if err != nil {
		return err
	}
	names := make([]string, 0, len(cfg.Profiles))
	for n := range cfg.Profiles {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		p := cfg.Profiles[n]
		mode := "read-only"
		if p.MasterAddress != "" {
			mode = "writable"
		}
		fmt.Printf("%-12s %-10s %s  [%s]\n", n, p.ChainID, p.RPCURL, mode)
	}
	return nil
}

// parseProfileAddArgs parses `<name> [flags]` for `gnomcp profile add`. The
// profile name is the first positional; flags follow it. Go's flag parser
// stops at the first non-flag, so the name is split off before parsing —
// otherwise `add <name> --rpc ...` would silently drop every flag after <name>.
func parseProfileAddArgs(args []string) (string, profileAddOpts, error) {
	if len(args) < 1 || strings.HasPrefix(args[0], "-") {
		return "", profileAddOpts{}, fmt.Errorf("profile name required as the first argument")
	}
	name := args[0]
	fs := flag.NewFlagSet("profile add", flag.ContinueOnError)
	var o profileAddOpts
	fs.StringVar(&o.FromGnoweb, "from-gnoweb", "", "gnoweb URL to autofill rpc/chain-id")
	fs.StringVar(&o.RPC, "rpc", "", "RPC URL")
	fs.StringVar(&o.ChainID, "chain-id", "", "chain id (dev and known testnets, e.g. topaz-1, are writable; any other id, e.g. gnoland1, is read-only)")
	fs.StringVar(&o.Master, "master", "", "master address g1... (enables writes)")
	fs.StringVar(&o.IndexerURL, "indexer-url", "", "tx indexer GraphQL URL (optional)")
	if err := fs.Parse(args[1:]); err != nil {
		return "", profileAddOpts{}, err
	}
	return name, o, nil
}

// runProfile handles `gnomcp profile <list|add|remove>`.
func runProfile(args []string) {
	path := globalProfilePath()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gnomcp profile <list|add|remove>")
		os.Exit(1)
	}
	switch args[0] {
	case "list":
		if err := profileList(path); err != nil {
			fatal(err)
		}
	case "add":
		name, o, err := parseProfileAddArgs(args[1:])
		if err != nil {
			fmt.Fprintln(os.Stderr, "usage: gnomcp profile add <name> [--from-gnoweb URL | --rpc URL --chain-id ID] [--master g1...] [--indexer-url URL]")
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if err := profileAdd(path, name, o); err != nil {
			fatal(err)
		}
		fmt.Printf("added profile %q to %s\n", name, path)
	case "remove":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: gnomcp profile remove <name>")
			os.Exit(1)
		}
		if err := profileRemove(path, args[1]); err != nil {
			fatal(err)
		}
		fmt.Printf("removed profile %q\n", args[1])
	default:
		fmt.Fprintf(os.Stderr, "unknown profile subcommand %q\n", args[0])
		os.Exit(1)
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
