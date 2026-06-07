package write

import (
	"context"
	"errors"
	"fmt"

	"github.com/gnoverse/gno-mcp/internal/keystore"
	"github.com/gnoverse/gno-mcp/internal/server"
)

// RegisterKeyAddress registers the gno_key_address tool.
// ks provides agent addresses per profile; only local (dev) profiles have an
// agent key in this phase.
func RegisterKeyAddress(s *server.Server, ks *keystore.Keystore) {
	s.Registry().Add(&server.Tool{
		Name: "gno_key_address",
		Description: "Returns the agent's own account address for a local/dev profile — " +
			"the account it signs with (e.g. so you can fund it). " +
			"Read-only; performs no transaction. " +
			"Returns agent_identity_unavailable for non-local profiles (testnet and mainnet are not supported in this phase).",
		InputSchema: keyAddrInputSchema(s),
		OutputKind:  server.OutputText,
		Capability:  server.CapBaseRead,
		Annotations: server.Annotations{
			ReadOnly:    true,
			Destructive: false,
			Idempotent:  true,
			OpenWorld:   true,
		},
		Handler: func(ctx context.Context, args map[string]any) (server.Result, error) {
			return keyAddrHandler(ctx, args, s, ks)
		},
	})
}

func keyAddrHandler(
	_ context.Context,
	args map[string]any,
	s *server.Server,
	ks *keystore.Keystore,
) (server.Result, error) {
	profileName, err := stringArg(args, "profile")
	if err != nil {
		return server.Result{}, err
	}
	if profileName == "" {
		return server.Result{}, fmt.Errorf("profile: required — pick one of the configured profiles")
	}

	p, ok := s.Config().Profiles[profileName]
	if !ok {
		return server.Result{}, fmt.Errorf("profile %q: not found", profileName)
	}

	addr, err := ks.AgentAddress(p)
	if err != nil {
		if errors.Is(err, keystore.ErrNoAgentKey) {
			return server.Result{}, &server.ToolError{
				Code: "agent_identity_unavailable",
				Message: fmt.Sprintf(
					"profile %q has no agent key (local/dev only in this phase)",
					profileName,
				),
				Extra: map[string]any{"profile": profileName},
			}
		}
		return server.Result{}, fmt.Errorf("gno_key_address: %w", err)
	}

	return server.Result{
		Text: addr,
		StructuredContent: map[string]any{
			"address": addr,
		},
	}, nil
}

func keyAddrInputSchema(s *server.Server) map[string]any {
	props := map[string]any{}
	required := []string{}
	addAgentProfileArg(s, props, &required)
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	}
}
