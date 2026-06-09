package clientfaucet

import (
	"context"
	"fmt"

	"github.com/gnoverse/gno-mcp/internal/chain"
)

// LinkFaucet covers tier 1 (a human faucet page at faucetURL) and the manual
// fallback (faucetURL == ""). It never sends anything: it returns human
// instructions and confirms funding by polling on-chain balance.
type LinkFaucet struct {
	faucetURL string
	chain     chain.Client
}

func (l *LinkFaucet) Fund(_ context.Context, address, _ string) (Outcome, error) {
	if l.faucetURL == "" {
		return Outcome{
			Backend:      "manual",
			Address:      address,
			Instructions: fmt.Sprintf("No faucet is configured for this profile. Send ugnot to %s, then retry the write.", address),
		}, nil
	}
	return Outcome{
		Backend:      "link",
		Address:      address,
		FaucetURL:    l.faucetURL,
		Instructions: fmt.Sprintf("Open %s and fund %s, then retry the write.", l.faucetURL, address),
	}, nil
}

func (l *LinkFaucet) Funded(ctx context.Context, address string) (bool, error) {
	bal, err := l.chain.Balance(ctx, address)
	if err != nil {
		return false, err
	}
	return bal > 0, nil
}
