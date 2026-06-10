package chain

import (
	"context"
	"fmt"

	rpcclient "github.com/gnolang/gno/tm2/pkg/bft/rpc/client"
)

// QueryChainID dials rpcURL and returns the chain-id the node reports
// (ResultStatus.NodeInfo.Network). Read-only; used to verify that a profile's
// declared chain-id matches the node it points at before the profile is
// trusted (dynamic adds, where gnoweb-advertised data is a hint, not truth).
func QueryChainID(ctx context.Context, rpcURL string) (string, error) {
	if rpcURL == "" {
		return "", fmt.Errorf("rpc-url must not be empty")
	}
	rpc, err := rpcclient.NewHTTPClient(rpcURL)
	if err != nil {
		return "", fmt.Errorf("rpc client for %q: %w", rpcURL, err)
	}
	defer rpc.Close()
	st, err := rpc.Status(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("status query against %q: %w", rpcURL, err)
	}
	return st.NodeInfo.Network, nil
}
