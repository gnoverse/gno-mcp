package write

import (
	"context"
	"fmt"
	"time"

	"github.com/gnoverse/gno-mcp/internal/audit"
	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/keystore"
	"github.com/gnoverse/gno-mcp/internal/server"
)

// RegisterKeySend registers the gno_key_send tool.
// ks resolves both the source signer and the destination address (own keys
// only); resolver returns the chain client that broadcasts the bank send;
// alog records the transfer.
func RegisterKeySend(s *server.Server, ks *keystore.Keystore, resolver chain.Resolver, alog *audit.Log) {
	s.Registry().Add(&server.Tool{
		Name: "gno_key_send",
		Description: "Transfers ugnot between two of the agent's own keys in the same profile (bank MsgSend). " +
			"Use to fund a secondary key from the agent's main funded key so you can exercise realms " +
			"involving multiple addresses (escrow, transfers, multisig). " +
			"Both from and to are key NAMES, not addresses; the destination must be a key that already " +
			"exists in this profile — create it with gno_key_generate and list keys with gno_key_list. " +
			"Does NOT send to arbitrary external addresses and does NOT call realm functions " +
			"(use gno_call with send= for a payable call). The source key must be funded. " +
			"Returns the from/to addresses, the amount, the tx hash, and an equivalent gnokey command (illustrative — gnomcp already signed it).",
		InputSchema: keySendInputSchema(s),
		OutputKind:  server.OutputText,
		Capability:  server.CapWrite,
		Annotations: server.Annotations{ReadOnly: false, Destructive: true, Idempotent: false, OpenWorld: true},
		Handler: func(ctx context.Context, args map[string]any) (server.Result, error) {
			return keySendHandler(ctx, args, s, ks, resolver, alog)
		},
	})
}

func keySendInputSchema(s *server.Server) map[string]any {
	props := map[string]any{
		"from": map[string]any{
			"type":        "string",
			"default":     "default",
			"description": "Name of the source key to send from (default: \"default\"). Must be a funded key in this profile.",
		},
		"to": map[string]any{
			"type":        "string",
			"description": "Name of the destination key to send to. Must be an existing key in this profile (gno_key_generate). e.g. \"bob\".",
		},
		"amount": map[string]any{
			"type":        "integer",
			"description": "Amount of ugnot to transfer, a whole number greater than zero. e.g. 5000000.",
		},
	}
	required := []string{"to", "amount"}
	// Testnet only: local profiles have a single built-in key (test1), so an
	// own-keys transfer there is always a self-send. Multi-key lives on testnet.
	addTestnetProfileArg(s, props, &required)
	return map[string]any{"type": "object", "properties": props, "required": required, "additionalProperties": false}
}

func keySendHandler(ctx context.Context, args map[string]any, s *server.Server, ks *keystore.Keystore, resolver chain.Resolver, alog *audit.Log) (server.Result, error) {
	start := time.Now()
	var (
		profileName string
		argsSummary string
		auditResult = "tool_err"
	)
	defer func() {
		alog.Record(audit.Entry{
			Tool:        "gno_key_send",
			Profile:     profileName,
			ArgsSummary: argsSummary,
			Result:      auditResult,
			Duration:    time.Since(start).Milliseconds(),
		})
	}()

	profileName, profile, err := requireProfile(args, s)
	if err != nil {
		return server.Result{}, err
	}
	fromKey, err := server.StringArg(args, "from")
	if err != nil {
		return server.Result{}, err
	}
	toKey, err := server.StringArg(args, "to")
	if err != nil {
		return server.Result{}, err
	}
	if toKey == "" {
		return server.Result{}, fmt.Errorf("to: required — the destination key name")
	}
	amount, err := server.Int64Arg(args, "amount")
	if err != nil {
		return server.Result{}, err
	}
	// Set the audit summary before the fallible steps (including the amount guard)
	// so even a rejected attempt is logged with its amount and destination key.
	// Both are low-sensitivity (a key name and an amount), and gno_key_send
	// self-audits so the generic arg redaction does not apply.
	argsSummary = fmt.Sprintf("to=%s amount=%d", toKey, amount)
	if amount <= 0 {
		return server.Result{}, &server.ToolError{
			Code:    "invalid_amount",
			Message: fmt.Sprintf("amount must be a positive number of ugnot, got %d", amount),
			Extra:   map[string]any{"profile": profileName},
		}
	}

	// Resolve the destination as one of this profile's own keys. A missing key is
	// reported as such; the own-keys-only constraint means there is no untrusted
	// address to validate.
	toAddr, err := ks.AgentAddress(profileName, toKey, profile)
	if err != nil {
		if terr := agentKeyToolError(err, profileName, fmt.Sprintf("destination key %q does not exist — create it with gno_key_generate", toKey)); terr != nil {
			return server.Result{}, terr
		}
		return server.Result{}, fmt.Errorf("gno_key_send: resolve destination %q: %w", toKey, err)
	}

	c := resolver(profileName)
	if c == nil {
		return server.Result{}, fmt.Errorf("profile %q: no chain client available", profileName)
	}

	// Acquire the source signer (runs the testnet unfunded pre-check on the from key).
	signer, fromAddr, aerr := acquireAgentSigner(ctx, ks, c, "gno_key_send",
		"create or fund it with gno_key_generate / gno_faucet_fund", profileName, fromKey, profile, false)
	if aerr != nil {
		return server.Result{}, aerr
	}
	if fromAddr == toAddr {
		return server.Result{}, &server.ToolError{
			Code:    "same_account",
			Message: "from and to resolve to the same address — choose distinct keys",
			Extra:   map[string]any{"profile": profileName, "address": fromAddr},
		}
	}

	res, err := c.Send(ctx, signer, toAddr, amount)
	if err != nil {
		auditResult = "broadcast_err"
		return server.Result{}, fmt.Errorf("gno_key_send broadcast: %w", err)
	}
	auditResult = "ok"

	gkCmd := chain.GnokeyCmd{
		Sub: "send", To: toAddr, Send: fmt.Sprintf("%dugnot", amount),
		RPC: profile.RPCURL, ChainID: profile.ChainID, Signer: fromAddr,
	}.String()
	return attachGnokeyCmd(server.Result{
		Text: fmt.Sprintf("Sent %d ugnot from %s to %s (tx %s).", amount, fromAddr, toAddr, res.TxHash),
		StructuredContent: map[string]any{
			"from_address": fromAddr,
			"to_address":   toAddr,
			"amount_ugnot": amount,
			"tx_hash":      res.TxHash,
			"height":       res.Height,
		},
	}, gkCmd), nil
}
