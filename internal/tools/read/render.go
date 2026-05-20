package read

import (
	"context"
	"fmt"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/server"
)

// ChainResolver returns the chain.Client to use for a given profile.
// The caller wires this up — typically maps profile name to a chain.Real
// instance constructed from the profile's RPC URL.
type ChainResolver func(profile string) chain.Client

func RegisterRender(s *server.Server, resolve ChainResolver) {
	s.Registry().Add(&server.Tool{
		Name: "gno_render",
		Description: "Fetches the rendered markdown of a Gno realm at an optional subpath. " +
			"Use when the user or agent needs to read realm content for display, review, or " +
			"summarization. Returns the realm-authored markdown as an MCP resource (different " +
			"trust posture in clients). Does NOT execute or evaluate code — use gno_eval for that. " +
			"Backed by vm/qrender; HEAD-only (no historical heights).",
		InputSchema: renderInputSchema(s),
		OutputKind:  server.OutputResource,
		Capability:  server.CapBaseRead,
		Handler: func(ctx context.Context, args map[string]any) (server.Result, error) {
			realm, _ := args["realm"].(string)
			if realm == "" {
				return server.Result{}, fmt.Errorf("realm is required (e.g. gno.land/r/myorg/blog)")
			}
			path, _ := args["path"].(string)
			profile, _ := args["profile"].(string)

			c := resolve(profile)
			if c == nil {
				return server.Result{}, fmt.Errorf("no chain client for profile %q", profile)
			}
			body, err := c.Render(ctx, realm, path)
			if err != nil {
				return server.Result{}, fmt.Errorf("gno_render: %w", err)
			}
			uri := "gno://" + realm
			if path != "" {
				uri += "/" + path
			}
			return server.Result{
				ResourceURI:  uri,
				ResourceBody: body,
				ResourceMIME: "text/markdown",
			}, nil
		},
	})
}

func renderInputSchema(s *server.Server) map[string]any {
	props := map[string]any{
		"realm": map[string]any{
			"type":        "string",
			"description": "Realm package path (e.g. 'gno.land/r/myorg/blog'). Required.",
			"pattern":     `^gno\.land/r/[a-z0-9_/\.]+$`,
		},
		"path": map[string]any{
			"type":        "string",
			"description": "Optional subpath within the realm's Render() router (e.g. 'post/123'). Empty = home.",
		},
	}
	required := []string{"realm"}
	addProfileArg(s, props, &required)
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	}
}

// addProfileArg adds the `profile` arg to the props map per the server's schema rules.
// Shared across all chain-bound tools in this package.
func addProfileArg(s *server.Server, props map[string]any, required *[]string) {
	ps := s.ProfileSchema()
	arg := map[string]any{
		"type": "string",
		"enum": ps.Enum,
	}
	if ps.Default != "" {
		arg["default"] = ps.Default
		arg["description"] = fmt.Sprintf("Target chain profile. Default: %q.", ps.Default)
	} else {
		arg["description"] = "Target chain profile. Required (no default — pick one explicitly)."
	}
	props["profile"] = arg
	if ps.Required {
		*required = append(*required, "profile")
	}
}
