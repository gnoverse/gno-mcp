package write

import (
	"context"
	"errors"
	"fmt"

	"github.com/gnoverse/gno-mcp/internal/keystore"
	"github.com/gnoverse/gno-mcp/internal/server"
)

// RegisterKeyDelete registers the gno_key_delete tool.
func RegisterKeyDelete(s *server.Server, ks *keystore.Keystore) {
	s.Registry().Add(&server.Tool{
		Name: "gno_key_delete",
		Description: "Permanently deletes a named testnet agent key for a profile. " +
			"IRREVERSIBLE: any ugnot the deleted address held becomes unreachable. " +
			"Testnet profiles only (local uses the built-in test1). Use to free a slot when the " +
			"profile is at its key cap, or to replace a key (delete, then gno_key_generate again). " +
			"The key arg is required — there is no default, so you cannot delete a key by omission. " +
			"Returns agent_identity_unavailable if no such key exists, and key_deletion_unsupported for a non-testnet profile.",
		InputSchema: keyDeleteInputSchema(s),
		OutputKind:  server.OutputText,
		Capability:  server.CapWrite,
		Annotations: server.Annotations{
			ReadOnly:    false,
			Destructive: true,
			Idempotent:  false,
			OpenWorld:   false,
		},
		Handler: func(ctx context.Context, args map[string]any) (server.Result, error) {
			return keyDeleteHandler(ctx, args, s, ks)
		},
	})
}

func keyDeleteHandler(
	_ context.Context,
	args map[string]any,
	s *server.Server,
	ks *keystore.Keystore,
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

	addr, err := ks.DeleteForProfile(profileName, keyName, p)
	if err != nil {
		if errors.Is(err, keystore.ErrKeyGenTestnetOnly) {
			return server.Result{}, &server.ToolError{
				Code:    "key_deletion_unsupported",
				Message: fmt.Sprintf("gno_key_delete is testnet-only; profile %q is not a testnet profile", profileName),
				Extra:   map[string]any{"profile": profileName},
			}
		}
		if terr := agentKeyToolError(err, profileName, fmt.Sprintf("no key named %q to delete", keyName)); terr != nil {
			return server.Result{}, terr
		}
		return server.Result{}, fmt.Errorf("gno_key_delete: %w", err)
	}

	return server.Result{
		Text: fmt.Sprintf("Deleted key %q (address %s). Any funds it held are now unreachable.", keyName, addr),
		StructuredContent: map[string]any{
			"deleted_key":     keyName,
			"deleted_address": addr,
		},
	}, nil
}

func keyDeleteInputSchema(s *server.Server) map[string]any {
	props := map[string]any{}
	required := []string{"key"}
	addTestnetProfileArg(s, props, &required)
	addOptionalKeyArg(props)
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	}
}
