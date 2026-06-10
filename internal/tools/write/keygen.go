package write

import (
	"context"
	"errors"
	"fmt"

	"github.com/gnoverse/gno-mcp/internal/keystore"
	"github.com/gnoverse/gno-mcp/internal/server"
)

// RegisterKeyGenerate registers the gno_key_generate tool.
// ks persists the generated mnemonic; testnet profiles only.
func RegisterKeyGenerate(s *server.Server, ks *keystore.Keystore) {
	s.Registry().Add(&server.Tool{
		Name: "gno_key_generate",
		Description: "Generates and persists the agent's own testnet account for a profile, " +
			"returning its bech32 g1… address. " +
			"Testnet profiles only: local profiles use the built-in test1 account, " +
			"and mainnet/prod profiles use session-based signing. " +
			"The returned address must be funded before the agent can submit transactions " +
			"(faucet support is a later phase). " +
			"Refuses to overwrite an existing key — call gno_key_address to retrieve it. " +
			"The key is stored encrypted only when GNOMCP_SESSION_PASSPHRASE is set; " +
			"otherwise it is stored as plaintext, acceptable for a dev/test hot key. " +
			"Returns key_generation_unsupported for non-testnet profiles and " +
			"key_already_exists if a key was already generated for this profile.",
		InputSchema: keyGenInputSchema(s),
		OutputKind:  server.OutputText,
		Capability:  server.CapWrite,
		Annotations: server.Annotations{
			ReadOnly:    false,
			Destructive: false,
			Idempotent:  false,
			OpenWorld:   false,
		},
		Handler: func(ctx context.Context, args map[string]any) (server.Result, error) {
			return keyGenHandler(ctx, args, s, ks)
		},
	})
}

func keyGenHandler(
	_ context.Context,
	args map[string]any,
	s *server.Server,
	ks *keystore.Keystore,
) (server.Result, error) {
	profileName, p, err := requireProfile(args, s)
	if err != nil {
		return server.Result{}, err
	}

	addr, err := ks.GenerateForProfile(profileName, p)
	if err != nil {
		if errors.Is(err, keystore.ErrKeyGenTestnetOnly) {
			return server.Result{}, &server.ToolError{
				Code: "key_generation_unsupported",
				Message: fmt.Sprintf(
					"gno_key_generate is testnet-only; profile %q is not a testnet profile (local uses the built-in test1, prod uses sessions)",
					profileName,
				),
				Extra: map[string]any{"profile": profileName},
			}
		}
		if errors.Is(err, keystore.ErrKeyExists) {
			return server.Result{}, &server.ToolError{
				Code: "key_already_exists",
				Message: fmt.Sprintf(
					"profile %q already has an agent key — use gno_key_address to get its address",
					profileName,
				),
				Extra: map[string]any{"profile": profileName},
			}
		}
		// ErrNoAgentKey cannot flow from GenerateForProfile, so the hint is unused.
		if terr := agentKeyToolError(err, profileName, ""); terr != nil {
			return server.Result{}, terr
		}
		return server.Result{}, fmt.Errorf("gno_key_generate: %w", err)
	}

	return server.Result{
		Text: addr,
		StructuredContent: map[string]any{
			"address": addr,
		},
	}, nil
}

func keyGenInputSchema(s *server.Server) map[string]any {
	props := map[string]any{}
	required := []string{}
	addTestnetProfileArg(s, props, &required)
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	}
}
