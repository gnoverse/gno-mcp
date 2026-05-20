package chain

import (
	"context"
	"fmt"
	"strings"

	gnoclient "github.com/gnolang/gno/gno.land/pkg/gnoclient"
	rpcclient "github.com/gnolang/gno/tm2/pkg/bft/rpc/client"
)

// Real implements Client against a live gno chain via gnoclient + RPC.
type Real struct {
	cli *gnoclient.Client
}

// Assert Real satisfies Client at compile time.
var _ Client = (*Real)(nil)

// NewReal creates a Real client connected to rpcURL.
// rpcURL must be non-empty; chainID is stored for future Milestone B writes.
func NewReal(rpcURL, _ string) (*Real, error) {
	if rpcURL == "" {
		return nil, fmt.Errorf("rpc-url must not be empty")
	}

	rpc, err := rpcclient.NewHTTPClient(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("rpc client: %w", err)
	}

	return &Real{
		cli: &gnoclient.Client{RPCClient: rpc},
	}, nil
}

// Render returns the rendered output for a realm at a given subpath.
// Backed by vm/qrender.
func (r *Real) Render(_ context.Context, realm, path string) (string, error) {
	out, _, err := r.cli.Render(realm, path)
	if err != nil {
		return "", fmt.Errorf("vm/qrender: %w", err)
	}
	return out, nil
}

// Eval evaluates an expression in a realm's context.
// Backed by vm/qeval.
func (r *Real) Eval(_ context.Context, realm, expr string) (string, error) {
	out, _, err := r.cli.QEval(realm, expr)
	if err != nil {
		return "", fmt.Errorf("vm/qeval: %w", err)
	}
	return out, nil
}

// File returns the raw source of a single file in a realm.
// Backed by vm/qfile with an explicit file name appended to realm.
func (r *Real) File(_ context.Context, realm, file string) (string, error) {
	qres, err := r.cli.Query(gnoclient.QueryCfg{
		Path: "vm/qfile",
		Data: []byte(realm + "/" + file),
	})
	if err != nil {
		return "", fmt.Errorf("vm/qfile: %w", err)
	}
	return string(qres.Response.Data), nil
}

// ListFiles returns the file names that make up a realm.
// Backed by vm/qfile without a file name; result is newline-separated.
func (r *Real) ListFiles(_ context.Context, realm string) ([]string, error) {
	qres, err := r.cli.Query(gnoclient.QueryCfg{
		Path: "vm/qfile",
		Data: []byte(realm),
	})
	if err != nil {
		return nil, fmt.Errorf("vm/qfile: %w", err)
	}

	var files []string
	for _, name := range strings.Split(string(qres.Response.Data), "\n") {
		if name != "" {
			files = append(files, name)
		}
	}
	return files, nil
}

// Doc returns the realm's package + per-function godoc.
// Backed by vm/qdoc.
func (r *Real) Doc(_ context.Context, realm string) (string, error) {
	qres, err := r.cli.Query(gnoclient.QueryCfg{
		Path: "vm/qdoc",
		Data: []byte(realm),
	})
	if err != nil {
		return "", fmt.Errorf("vm/qdoc: %w", err)
	}
	return string(qres.Response.Data), nil
}
