package read

import (
	"context"
	"fmt"

	"github.com/gnoverse/gno-mcp/internal/budget"
	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/server"
)

// RegisterAccount wires the gno_account tool into s. The resolver maps a
// profile name to the chain.Client used to satisfy calls.
//
// gno_account reads the on-chain account record for any address — the
// general "does this address exist / can it afford X / what nonce is it at"
// query. It complements gno_key_address (the agent's own address) and
// gno_history/gno_activity (per-realm transaction logs via tx-indexer).
func RegisterAccount(s *server.Server, resolve chain.Resolver) {
	s.Registry().Add(&server.Tool{
		Name: "gno_account",
		Description: "Fetches on-chain account state for a gno address: balance, sequence (nonce), and account number. " +
			"Use to check whether an address exists, can afford a transaction, or what sequence its next tx needs. " +
			"An address with no on-chain record returns exists:false — that is a normal answer (never funded or used), not an error. " +
			"Does NOT return transaction history (use gno_history/gno_activity) and does NOT create keys (use gno_key_generate). " +
			"address must be a bech32 'g1…' string (e.g. 'g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5'). " +
			"Backed by auth/accounts; HEAD-only.",
		InputSchema: accountInputSchema(s),
		OutputKind:  server.OutputText,
		Capability:  server.CapBaseRead,
		Annotations: server.Annotations{ReadOnly: true, Idempotent: true, OpenWorld: true},
		Handler:     accountHandler(resolve),
	})
}

func accountHandler(resolve chain.Resolver) server.Handler {
	return func(ctx context.Context, args map[string]any) (server.Result, error) {
		address, err := server.StringArg(args, "address")
		if err != nil {
			return server.Result{}, err
		}
		if address == "" {
			return server.Result{}, fmt.Errorf("address is required (bech32 'g1…')")
		}
		profile, err := server.StringArg(args, "profile")
		if err != nil {
			return server.Result{}, err
		}

		c := resolve(profile)
		if c == nil {
			return server.Result{}, fmt.Errorf("no chain client for profile %q", profile)
		}

		info, err := c.Account(ctx, address)
		if err != nil {
			return server.Result{}, fmt.Errorf("gno_account: %w", err)
		}

		// An empty coin set renders as the empty string; "0" reads better.
		coins := info.Coins.String()
		if coins == "" {
			coins = "0"
		}

		var text string
		if info.Exists {
			text = fmt.Sprintf("Account %s: balance %s, sequence %d, account number %d.",
				address, coins, info.Sequence, info.AccountNumber)
		} else {
			text = fmt.Sprintf("Account %s has no on-chain record (never funded or used).", address)
		}
		wrapped, _ := budget.Wrapped(text, "", "account", address)

		return server.Result{
			Text: wrapped,
			StructuredContent: map[string]any{
				"address":        address,
				"exists":         info.Exists,
				"coins":          coins,
				"sequence":       info.Sequence,
				"account_number": info.AccountNumber,
			},
		}, nil
	}
}

func accountInputSchema(s *server.Server) map[string]any {
	props := map[string]any{
		"address": map[string]any{
			"type":        "string",
			"description": "Bech32 account address (e.g. 'g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5'). Required.",
		},
	}
	required := []string{"address"}
	addProfileArg(s, props, &required)
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	}
}
