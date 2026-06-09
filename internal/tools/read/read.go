package read

import (
	"context"
	"fmt"
	"strings"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/server"
)

// RegisterRead wires the gno_read tool into s. The resolver maps a
// profile name to the chain.Client used to satisfy calls.
//
// When file is non-empty, gno_read fetches a single source file from the
// realm (MIME text/x-gno). When file is empty, it returns the realm's
// file listing joined by newlines (MIME text/plain).
func RegisterRead(s *server.Server, resolve chain.Resolver) {
	s.Registry().Add(&server.Tool{
		Name: "gno_read",
		Description: "Fetches realm source code or a file listing from a Gno realm. " +
			"When 'file' is provided, returns the raw source of that file (MIME text/x-gno). " +
			"When 'file' is omitted, returns the newline-separated list of source file names " +
			"that make up the realm (MIME text/plain). " +
			"Returns an MCP resource. Backed by vm/qfile; HEAD-only.",
		InputSchema: readInputSchema(s),
		OutputKind:  server.OutputResource,
		Capability:  server.CapBaseRead,
		Handler:     readHandler(s, resolve),
	})
}

func readHandler(s *server.Server, resolve chain.Resolver) server.Handler {
	return func(ctx context.Context, args map[string]any) (server.Result, error) {
		realm, err := stringArg(args, "realm")
		if err != nil {
			return server.Result{}, err
		}
		if realm == "" {
			return server.Result{}, fmt.Errorf("realm is required (e.g. gno.land/r/myorg/foo)")
		}
		file, err := stringArg(args, "file")
		if err != nil {
			return server.Result{}, err
		}
		profile, err := stringArg(args, "profile")
		if err != nil {
			return server.Result{}, err
		}

		c := resolve(profile)
		if c == nil {
			return server.Result{}, fmt.Errorf("no chain client for profile %q", profile)
		}

		if file != "" {
			body, err := c.File(ctx, realm, file)
			if err != nil {
				return server.Result{}, fmt.Errorf("gno_read: %w", err)
			}
			gnowebURL := ""
			if p, ok := s.Config().Profiles[profile]; ok {
				gnowebURL = gnowebURLFor(p.RPCURL, realm, "")
			}
			body, _ = budgetBody(body, gnowebURL)
			return server.Result{
				ResourceURI:  "gno://" + realm + "/" + file,
				ResourceBody: body,
				ResourceMIME: "text/x-gno",
			}, nil
		}

		names, err := c.ListFiles(ctx, realm)
		if err != nil {
			return server.Result{}, fmt.Errorf("gno_read: %w", err)
		}
		listing := strings.Join(names, "\n") + "\n"
		gnowebURL := ""
		if p, ok := s.Config().Profiles[profile]; ok {
			gnowebURL = gnowebURLFor(p.RPCURL, realm, "")
		}
		listing, _ = budgetBody(listing, gnowebURL)
		return server.Result{
			ResourceURI:  "gno://" + realm,
			ResourceBody: listing,
			ResourceMIME: "text/plain",
		}, nil
	}
}

func readInputSchema(s *server.Server) map[string]any {
	props := map[string]any{
		"realm": map[string]any{
			"type": "string",
			"description": "Realm or pure-package path (e.g. 'gno.land/r/myorg/foo' for a realm " +
				"or 'gno.land/p/myorg/lib' for a pure package). Required.",
			// Allow /r/ realms and /p/ pure packages; lowercase letters, digits,
			// underscore, dot, hyphen, and slash inside the path.
			"pattern": `^gno\.land/[rp]/[a-z0-9_\-/\.]+$`,
		},
		"file": map[string]any{
			"type":        "string",
			"description": "Source file name within the realm (e.g. 'foo.gno'). Omit to list all files.",
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
