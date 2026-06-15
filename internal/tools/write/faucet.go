package write

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/clientfaucet"
	"github.com/gnoverse/gno-mcp/internal/keystore"
	"github.com/gnoverse/gno-mcp/internal/server"
)

const (
	faucetPollTimeout  = 60 * time.Second
	faucetPollInterval = 3 * time.Second
)

// RegisterFaucetFund registers the gno_faucet_fund tool.
// ks provides the agent address; resolver returns the chain client for balance
// polling; httpClient is used by the service-faucet backend (if configured).
func RegisterFaucetFund(s *server.Server, ks *keystore.Keystore, resolver chain.Resolver, httpClient *http.Client) {
	s.Registry().Add(&server.Tool{
		Name: "gno_faucet_fund",
		Description: "Funds the agent's own testnet account for a profile so it can submit transactions. " +
			"Uses whatever the profile configures: an automatic faucet service, an existing faucet page, " +
			"or (if neither) reports the address to fund manually. Requires a generated agent key " +
			"(run gno_key_generate first). Optional args: profile (testnet profiles only; the server default applies when omitted) " +
			"and key (which named key to fund; default \"default\"). Reports the acting " +
			"address and whether it is funded yet; if not, surfaces where to fund it.",
		InputSchema: faucetFundInputSchema(s),
		OutputKind:  server.OutputText,
		Capability:  server.CapWrite,
		Annotations: server.Annotations{ReadOnly: false, Destructive: false, Idempotent: true, OpenWorld: true},
		Handler: func(ctx context.Context, args map[string]any) (server.Result, error) {
			return faucetFundHandler(ctx, args, s, ks, resolver, httpClient)
		},
	})
}

func faucetFundInputSchema(s *server.Server) map[string]any {
	props := map[string]any{}
	required := []string{}
	addTestnetProfileArg(s, props, &required)
	addOptionalKeyArg(props)
	return map[string]any{"type": "object", "properties": props, "required": required, "additionalProperties": false}
}

func faucetFundHandler(ctx context.Context, args map[string]any, s *server.Server, ks *keystore.Keystore, resolver chain.Resolver, httpClient *http.Client) (server.Result, error) {
	profileName, profile, err := requireProfile(args, s)
	if err != nil {
		return server.Result{}, err
	}
	keyName, err := keyArg(args)
	if err != nil {
		return server.Result{}, err
	}
	addr, err := ks.AgentAddress(profileName, keyName, profile)
	if err != nil {
		if terr := agentKeyToolError(err, profileName, "run gno_key_generate first, then gno_faucet_fund"); terr != nil {
			return server.Result{}, terr
		}
		return server.Result{}, fmt.Errorf("gno_faucet_fund: agent address: %w", err)
	}
	c := resolver(profileName)
	if c == nil {
		return server.Result{}, fmt.Errorf("profile %q: no chain client available", profileName)
	}

	f := clientfaucet.Resolve(profile, c, httpClient)
	out, err := f.Fund(ctx, addr, profile.ChainID)
	if err != nil {
		return server.Result{}, err
	}

	funded, pollErr := pollFunded(ctx, f, addr, faucetPollTimeout, faucetPollInterval)
	status := "not funded yet"
	if funded {
		status = "funded"
	} else if pollErr != nil {
		status = fmt.Sprintf("not funded yet (last balance check failed: %v)", pollErr)
	}
	return server.Result{Text: fmt.Sprintf("%s\nAgent address %s: %s.", out.Instructions, addr, status)}, nil
}

// pollFunded polls Funded until true or timeout, returning the funded state and
// the last balance-check error (if any) so a dead RPC is distinguishable from a
// genuinely-unfunded address. Bounded — never blocks indefinitely on a human action.
func pollFunded(ctx context.Context, f clientfaucet.Faucet, addr string, timeout, interval time.Duration) (bool, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		ok, err := f.Funded(ctx, addr)
		if err == nil && ok {
			return true, nil
		}
		if err != nil {
			lastErr = err
		}
		if time.Now().After(deadline) {
			return false, lastErr
		}
		select {
		case <-ctx.Done():
			return false, lastErr
		case <-time.After(interval):
		}
	}
}
