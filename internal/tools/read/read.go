package read

import (
	"context"
	"fmt"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/server"
	"golang.org/x/tools/txtar"
)

// RegisterRead wires the gno_read tool into s. The resolver maps a profile
// name to the chain.Client used to satisfy calls.
//
// When file is non-empty, gno_read returns that single source file
// (MIME text/x-gno). When file is omitted, it returns the WHOLE package as a
// txtar archive (MIME text/plain) — every file's name and body in one call.
func RegisterRead(s *server.Server, resolve chain.Resolver) {
	s.Registry().Add(&server.Tool{
		Name: "gno_read",
		Description: "Fetches Gno package source. " +
			"When 'file' is provided, returns the raw source of that file (MIME text/x-gno). " +
			"When 'file' is omitted, returns the WHOLE package as a txtar archive — every file's " +
			"name (in '-- name --' headers) and body in one call (MIME text/plain). " +
			"Works for any realm (gno.land/r/...) or pure package (gno.land/p/...). " +
			"For the API surface only use gno_inspect; for rendered output use gno_render. " +
			"Returns an MCP resource. Backed by vm/qfile; HEAD-only.",
		InputSchema: readInputSchema(s),
		OutputKind:  server.OutputResource,
		Capability:  server.CapBaseRead,
		Handler:     readHandler(s, resolve),
	})
}

func readHandler(s *server.Server, resolve chain.Resolver) server.Handler {
	return func(ctx context.Context, args map[string]any) (server.Result, error) {
		path, err := server.StringArg(args, "path")
		if err != nil {
			return server.Result{}, err
		}
		if path == "" {
			return server.Result{}, fmt.Errorf("path is required (e.g. gno.land/r/myorg/foo)")
		}
		if !chain.IsReadablePackagePath(path) {
			return server.Result{}, fmt.Errorf(
				"path must be a realm (gno.land/r/...) or pure package (gno.land/p/...); got %q", path)
		}
		file, err := server.StringArg(args, "file")
		if err != nil {
			return server.Result{}, err
		}
		profile, err := server.StringArg(args, "profile")
		if err != nil {
			return server.Result{}, err
		}

		c := resolve(profile)
		if c == nil {
			return server.Result{}, fmt.Errorf("no chain client for profile %q", profile)
		}

		gnowebURL := ""
		if p, ok := s.Config().Profiles[profile]; ok {
			gnowebURL = gnowebURLFor(p.RPCURL, path, "")
		}

		if file != "" {
			body, err := c.File(ctx, path, file)
			if err != nil {
				return server.Result{}, fmt.Errorf("gno_read: %w", err)
			}
			body, _ = budgetBody(body, gnowebURL)
			return server.Result{
				ResourceURI:  "gno://" + path + "/" + file,
				ResourceBody: body,
				ResourceMIME: "text/x-gno",
			}, nil
		}

		files, err := chain.ReadPackageFiles(ctx, c, path)
		if err != nil {
			return server.Result{}, fmt.Errorf("gno_read: %w", err)
		}
		if len(files) == 0 {
			return server.Result{}, fmt.Errorf("gno_read: no files found at %q (not a deployed package?)", path)
		}
		ar := &txtar.Archive{Files: make([]txtar.File, len(files))}
		for i, mf := range files {
			ar.Files[i] = txtar.File{Name: mf.Name, Data: []byte(mf.Body)}
		}
		body, _ := budgetBody(string(txtar.Format(ar)), gnowebURL)
		return server.Result{
			ResourceURI:  "gno://" + path,
			ResourceBody: body,
			ResourceMIME: "text/plain",
		}, nil
	}
}

func readInputSchema(s *server.Server) map[string]any {
	props := map[string]any{
		"path": map[string]any{
			"type": "string",
			"description": "Package path: a realm (gno.land/r/myorg/foo) or pure package " +
				"(gno.land/p/myorg/lib). Required.",
			// Permissive hint only; authoritative validation is in the handler.
			"pattern": `^[a-z0-9][a-z0-9._/\-]+$`,
		},
		"file": map[string]any{
			"type":        "string",
			"description": "Source file name within the package (e.g. 'foo.gno'). Omit to fetch the whole package as txtar.",
		},
	}
	required := []string{"path"}
	addProfileArg(s, props, &required)
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	}
}
