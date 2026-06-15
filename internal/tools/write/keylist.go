package write

import (
	"context"
	"fmt"
	"strings"

	"github.com/gnoverse/gno-mcp/internal/keystore"
	"github.com/gnoverse/gno-mcp/internal/server"
)

// RegisterKeyList registers the gno_key_list tool.
// ks enumerates the profile's persisted agent keys and their addresses.
func RegisterKeyList(s *server.Server, ks *keystore.Keystore) {
	s.Registry().Add(&server.Tool{
		Name: "gno_key_list",
		Description: "Lists the agent's own account keys for a profile with their bech32 g1… addresses. " +
			"Use to discover keys generated in earlier sessions before selecting one with the key arg on " +
			"gno_call, gno_faucet_fund, or gno_key_send. Read-only; performs no transaction. " +
			"Local profiles report a single \"default\" entry (the built-in test1); testnet profiles list " +
			"every generated key. Returns an array of {name, address}; an empty list means no key has been " +
			"generated yet (run gno_key_generate).",
		InputSchema: keyListInputSchema(s),
		OutputKind:  server.OutputText,
		Capability:  server.CapBaseRead,
		Annotations: server.Annotations{
			ReadOnly:    true,
			Destructive: false,
			Idempotent:  true,
			OpenWorld:   false,
		},
		Handler: func(ctx context.Context, args map[string]any) (server.Result, error) {
			return keyListHandler(ctx, args, s, ks)
		},
	})
}

func keyListHandler(
	_ context.Context,
	args map[string]any,
	s *server.Server,
	ks *keystore.Keystore,
) (server.Result, error) {
	profileName, p, err := requireProfile(args, s)
	if err != nil {
		return server.Result{}, err
	}

	keys, err := ks.ListKeys(profileName, p)
	if err != nil {
		if terr := agentKeyToolError(err, profileName, "run gno_key_generate to create one"); terr != nil {
			return server.Result{}, terr
		}
		return server.Result{}, fmt.Errorf("gno_key_list: %w", err)
	}

	list := make([]map[string]any, len(keys))
	var b strings.Builder
	if len(keys) == 0 {
		b.WriteString("No agent keys for this profile yet — run gno_key_generate.")
	}
	for i, k := range keys {
		entry := map[string]any{"name": k.Name}
		if k.Err != "" {
			fmt.Fprintf(&b, "%s\t(unreadable: %s)\n", k.Name, k.Err)
			entry["error"] = k.Err
		} else {
			fmt.Fprintf(&b, "%s\t%s\n", k.Name, k.Address)
			entry["address"] = k.Address
		}
		list[i] = entry
	}

	return server.Result{
		Text:              strings.TrimRight(b.String(), "\n"),
		StructuredContent: map[string]any{"keys": list},
	}, nil
}

func keyListInputSchema(s *server.Server) map[string]any {
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
