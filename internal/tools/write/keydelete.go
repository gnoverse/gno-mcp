package write

import (
	"context"
	"fmt"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/keystore"
	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/gnoverse/gno-mcp/internal/server"
)

// RegisterKeyDelete registers the gno_key_delete tool.
// resolver provides the chain client used to check the key's balance before
// deleting, so a funded key is not abandoned by accident.
func RegisterKeyDelete(s *server.Server, ks *keystore.Keystore, resolver chain.Resolver) {
	s.Registry().Add(&server.Tool{
		Name: "gno_key_delete",
		Description: "Permanently deletes a named testnet agent key for a profile. " +
			"IRREVERSIBLE: any ugnot the deleted address held becomes unreachable. " +
			"Testnet profiles only (local uses the built-in test1). Use to free a slot when the " +
			"profile is at its key cap, or to replace a key (delete, then gno_key_generate again). " +
			"The key arg is required — there is no default, so you cannot delete a key by omission. " +
			"Refuses by default if the key still holds ugnot (key_has_funds). To remove a funded key safely, pass " +
			"sweep_to=<another key name> — it atomically moves the full balance (minus gas) to that key, then deletes. " +
			"Pass force=true instead to delete and permanently abandon the funds. " +
			"Returns agent_identity_unavailable if no such key exists, and key_deletion_unsupported for a non-testnet profile.",
		InputSchema: keyDeleteInputSchema(s),
		OutputKind:  server.OutputText,
		Capability:  server.CapWrite,
		Annotations: server.Annotations{
			ReadOnly:    false,
			Destructive: true,
			Idempotent:  false,
			OpenWorld:   true,
		},
		Handler: func(ctx context.Context, args map[string]any) (server.Result, error) {
			return keyDeleteHandler(ctx, args, s, ks, resolver)
		},
	})
}

func keyDeleteHandler(
	ctx context.Context,
	args map[string]any,
	s *server.Server,
	ks *keystore.Keystore,
	resolver chain.Resolver,
) (server.Result, error) {
	profileName, p, err := requireProfile(args, s)
	if err != nil {
		return server.Result{}, err
	}
	keyName, err := keyArg(args)
	if err != nil {
		return server.Result{}, err
	}
	if keyName == "" {
		return server.Result{}, fmt.Errorf("key: required — name the key to delete (no default, to avoid deleting the wrong key)")
	}
	force, err := server.BoolArg(args, "force")
	if err != nil {
		return server.Result{}, err
	}
	sweepToKey, err := server.StringArg(args, "sweep_to")
	if err != nil {
		return server.Result{}, err
	}
	if !p.IsTestnet() {
		return server.Result{}, &server.ToolError{
			Code:    "key_deletion_unsupported",
			Message: fmt.Sprintf("gno_key_delete is testnet-only; profile %q is not a testnet profile", profileName),
			Extra:   map[string]any{"profile": profileName},
		}
	}

	// Resolve the key's address (and confirm it exists) before touching the chain.
	addr, err := ks.AgentAddress(profileName, keyName, p)
	if err != nil {
		if terr := agentKeyToolError(err, profileName, fmt.Sprintf("no key named %q to delete", keyName)); terr != nil {
			return server.Result{}, terr
		}
		return server.Result{}, fmt.Errorf("gno_key_delete: %w", err)
	}

	c := resolver(profileName)
	if c == nil {
		return server.Result{}, fmt.Errorf("profile %q: no chain client available", profileName)
	}
	bal, err := c.Balance(ctx, addr)
	if err != nil {
		return server.Result{}, fmt.Errorf("gno_key_delete: balance check: %w", err)
	}

	text := fmt.Sprintf("Deleted key %q (address %s).", keyName, addr)
	sc := map[string]any{"deleted_key": keyName, "deleted_address": addr}

	switch {
	case sweepToKey != "":
		// Atomic recovery: sweep the full live balance (minus the gas fee) to
		// another of the agent's keys, then delete — so funds can't be stranded by
		// stale bookkeeping. A balance at or below the gas fee can't pay for its own
		// transfer; it is left and reported.
		swept, err := sweepBeforeDelete(ctx, ks, c, profileName, keyName, sweepToKey, addr, bal, p)
		if err != nil {
			return server.Result{}, err
		}
		if swept > 0 {
			text = fmt.Sprintf("Swept %d ugnot to key %q, then deleted key %q (address %s).", swept, sweepToKey, keyName, addr)
			sc["swept_ugnot"] = swept
			sc["swept_to"] = sweepToKey
		} else if bal > 0 {
			text = fmt.Sprintf("Deleted key %q (address %s); its balance of %d ugnot was at or below the gas fee, so it could not be swept and is now unreachable.", keyName, addr, bal)
			sc["abandoned_balance_ugnot"] = bal
		}
	case bal > 0 && !force:
		return server.Result{}, keyHasFundsError(ks, profileName, keyName, addr, bal, p)
	case bal > 0: // force
		text = fmt.Sprintf("Deleted key %q (address %s), abandoning %d ugnot — those funds are now permanently unreachable.", keyName, addr, bal)
		sc["abandoned_balance_ugnot"] = bal
	}

	if _, err := ks.DeleteForProfile(profileName, keyName, p); err != nil {
		if terr := agentKeyToolError(err, profileName, fmt.Sprintf("no key named %q to delete", keyName)); terr != nil {
			return server.Result{}, terr
		}
		return server.Result{}, fmt.Errorf("gno_key_delete: %w", err)
	}
	return server.Result{Text: text, StructuredContent: sc}, nil
}

// sweepBeforeDelete moves bal-minus-gas from the key being deleted to sweepToKey
// (another of the profile's keys) and returns the amount swept. It returns 0 when
// the balance is at or below the gas fee (nothing recoverable). It does NOT
// delete — the caller deletes only after a successful sweep, so a failed transfer
// never loses the key.
func sweepBeforeDelete(ctx context.Context, ks *keystore.Keystore, c chain.Client, profileName, keyName, sweepToKey, fromAddr string, bal int64, p profiles.Profile) (int64, error) {
	if sweepToKey == keyName {
		return 0, &server.ToolError{
			Code:    "invalid_sweep_target",
			Message: fmt.Sprintf("sweep_to %q is the key being deleted — choose a different key to receive the funds", sweepToKey),
			Extra:   map[string]any{"profile": profileName, "key": keyName},
		}
	}
	// Validate the sweep target exists before the sub-gas early-return below, so
	// sweep_to=<garbage> is rejected the same way whether or not there is a
	// sweepable balance.
	toAddr, err := ks.AgentAddress(profileName, sweepToKey, p)
	if err != nil {
		if terr := agentKeyToolError(err, profileName, fmt.Sprintf("sweep_to key %q does not exist — create it or name another", sweepToKey)); terr != nil {
			return 0, terr
		}
		return 0, fmt.Errorf("gno_key_delete: resolve sweep_to %q: %w", sweepToKey, err)
	}
	if bal <= chain.DefaultGasFeeUgnot {
		return 0, nil // sub-gas dust: nothing the transfer can cover
	}
	signer, err := ks.SignerForProfile(profileName, keyName, p)
	if err != nil {
		return 0, fmt.Errorf("gno_key_delete: signer for %q: %w", keyName, err)
	}
	amount := bal - chain.DefaultGasFeeUgnot
	if _, err := c.Send(ctx, signer, toAddr, amount); err != nil {
		return 0, fmt.Errorf("gno_key_delete: sweep to %q failed (key NOT deleted): %w", sweepToKey, err)
	}
	return amount, nil
}

// keyHasFundsError builds the refuse-on-funds error, leading with the safe
// recovery (sweep) and naming a concrete sweep target when one exists; force is
// the spelled-out last resort.
func keyHasFundsError(ks *keystore.Keystore, profileName, keyName, addr string, bal int64, p profiles.Profile) error {
	extra := map[string]any{
		"profile":       profileName,
		"key":           keyName,
		"address":       addr,
		"balance_ugnot": bal,
	}
	target := sweepTarget(ks, profileName, keyName, p)
	var msg string
	if target != "" {
		extra["sweep_target"] = target
		msg = fmt.Sprintf(
			"key %q holds %d ugnot — deleting it makes those funds permanently unreachable. "+
				"Recover them in one step: re-run with sweep_to=%q to move the full balance to that key and delete. "+
				"(Or pass force=true to delete and abandon the funds.)",
			keyName, bal, target)
	} else {
		msg = fmt.Sprintf(
			"key %q holds %d ugnot and is the only key in this profile, so there is no other key to sweep to. "+
				"Pass force=true to delete and permanently abandon the funds, or keep the key.",
			keyName, bal)
	}
	return &server.ToolError{Code: "key_has_funds", Message: msg, Extra: extra}
}

func keyDeleteInputSchema(s *server.Server) map[string]any {
	props := map[string]any{}
	required := []string{"key"}
	addTestnetProfileArg(s, props, &required)
	addOptionalKeyArg(props)
	props["sweep_to"] = map[string]any{
		"type": "string",
		"description": "Name of another of your keys to receive this key's funds before deletion. " +
			"Atomically sweeps the full live balance (minus the gas fee) to that key, then deletes — the safe way " +
			"to remove a funded key without stranding anything. e.g. \"main\". A balance at or below the gas fee is left behind.",
	}
	props["force"] = map[string]any{
		"type": "boolean",
		"description": "Delete even if the key still holds ugnot, permanently abandoning those funds (default false). " +
			"Prefer sweep_to to recover the balance instead; use force only when you intend to discard the funds " +
			"or there is no other key to sweep to.",
	}
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	}
}

// sweepTarget picks another of the profile's keys to suggest as a sweep
// destination, preferring the default key; returns "" if keyName is the only key.
func sweepTarget(ks *keystore.Keystore, profileName, keyName string, p profiles.Profile) string {
	keys, _ := ks.ListKeys(profileName, p) // best-effort: a listing failure just yields no concrete suggestion
	var first string
	for _, k := range keys {
		if k.Name == keyName {
			continue
		}
		if k.Name == keystore.DefaultKeyName {
			return k.Name
		}
		if first == "" {
			first = k.Name
		}
	}
	return first
}
