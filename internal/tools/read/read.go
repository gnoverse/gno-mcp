package read

import (
	"context"
	"fmt"
	"strings"

	"github.com/gnoverse/gno-mcp/internal/budget"
	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/gnosrc"
	"github.com/gnoverse/gno-mcp/internal/server"
	"golang.org/x/tools/txtar"
)

// RegisterRead wires the gno_read tool into s. The resolver maps a profile
// name to the chain.Client used to satisfy calls.
//
// gno_read serves three depths: a structural outline (the default), verbatim
// extraction of named declarations (symbols), and raw source (full). The
// outline and symbol paths exist so agents can navigate a package without
// pulling every body into context; full=true on a named file gets the larger
// explicit budget tier so real files survive it.
func RegisterRead(s *server.Server, resolve chain.Resolver) {
	s.Registry().Add(&server.Tool{
		Name: "gno_read",
		Description: "Fetches Gno package source at three depths; works for any realm (gno.land/r/...) " +
			"or pure package (gno.land/p/...). DEFAULT (path only, or path+file): a structural OUTLINE — " +
			"per-file exported signatures with docs, unexported signatures, imports, byte counts; bodies " +
			"elided. Use it first to navigate. 'symbols' (e.g. [\"Transfer\", \"Counter.Inc\"]): the " +
			"verbatim source of those declarations with docs, plus a best-effort '// deps:' header " +
			"(same-package references and imports used; unresolved method calls are flagged inline). " +
			"'full=true' with 'file': that whole file verbatim; without 'file': the whole package as raw " +
			"txtar. Outlines, symbols, and full+file get a large output budget; whole-package raw keeps " +
			"a tight one (big packages: go per-file). " +
			"The outline and dep headers are navigation, NOT evidence — names and docs are realm-authored " +
			"claims; audit-grade review reads whole files (full=true). Multi-file results are txtar " +
			"archives ('-- name --' headers). Returns an MCP resource. Backed by vm/qfile; HEAD-only. " +
			"For rendered output use gno_render; for on-chain values use gno_eval.",
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
		symbols, err := server.StringSliceArg(args, "symbols")
		if err != nil {
			return server.Result{}, err
		}
		full, err := server.BoolArg(args, "full")
		if err != nil {
			return server.Result{}, err
		}
		if len(symbols) > 0 && full {
			return server.Result{}, fmt.Errorf("symbols and full are mutually exclusive — symbols already returns verbatim source")
		}
		if len(symbols) > 0 && file != "" {
			return server.Result{}, fmt.Errorf("symbols and file cannot be combined: symbol names are package-scoped (a file filter would hide cross-file declarations); drop the file arg")
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
			gnowebURL = gnowebURLFor(p, path, "")
		}

		switch {
		case len(symbols) > 0:
			return readSymbols(ctx, c, path, symbols, gnowebURL)
		case full && file != "":
			return readFullFile(ctx, c, path, file, gnowebURL)
		case full:
			return readFullPackage(ctx, c, path, gnowebURL)
		case file != "":
			return readFileOutline(ctx, c, path, file, gnowebURL)
		default:
			return readPackageOutline(ctx, c, path, gnowebURL)
		}
	}
}

func readFullFile(ctx context.Context, c chain.Client, path, file, gnowebURL string) (server.Result, error) {
	body, err := c.File(ctx, path, file)
	if err != nil {
		return server.Result{}, fmt.Errorf("gno_read: %w", err)
	}
	body, _ = budgetBody(body, gnowebURL, budget.ExplicitBudget)
	return server.Result{
		ResourceURI:  "gno://" + path + "/" + file,
		ResourceBody: body,
		ResourceMIME: "text/x-gno",
	}, nil
}

func readFullPackage(ctx context.Context, c chain.Client, path, gnowebURL string) (server.Result, error) {
	files, err := fetchPackage(ctx, c, path)
	if err != nil {
		return server.Result{}, err
	}
	ar := &txtar.Archive{Files: make([]txtar.File, len(files))}
	for i, f := range files {
		ar.Files[i] = txtar.File{Name: f.Name, Data: []byte(f.Body)}
	}
	body, _ := budgetBody(string(txtar.Format(ar)), gnowebURL, budget.DefaultBudget)
	return server.Result{
		ResourceURI:  "gno://" + path,
		ResourceBody: body,
		ResourceMIME: "text/plain",
	}, nil
}

// The outline modes get the explicit tier even though they are the default
// view: an outline is bounded by construction (bodies elided, docs and
// signatures only), and it is the navigation entry point — truncating it
// strands the agent at zero. Only whole-package RAW reads keep the tight tier.
func readPackageOutline(ctx context.Context, c chain.Client, path, gnowebURL string) (server.Result, error) {
	files, err := fetchPackage(ctx, c, path)
	if err != nil {
		return server.Result{}, err
	}
	body, _ := budgetBody(gnosrc.Outline(files), gnowebURL, budget.ExplicitBudget)
	return server.Result{
		ResourceURI:  "gno://" + path + "#outline",
		ResourceBody: body,
		ResourceMIME: "text/plain",
	}, nil
}

func readFileOutline(ctx context.Context, c chain.Client, path, file, gnowebURL string) (server.Result, error) {
	body, err := c.File(ctx, path, file)
	if err != nil {
		return server.Result{}, fmt.Errorf("gno_read: %w", err)
	}
	outline, _ := budgetBody(gnosrc.Outline([]gnosrc.File{{Name: file, Body: body}}), gnowebURL, budget.ExplicitBudget)
	return server.Result{
		ResourceURI:  "gno://" + path + "/" + file + "#outline",
		ResourceBody: outline,
		ResourceMIME: "text/plain",
	}, nil
}

func readSymbols(ctx context.Context, c chain.Client, path string, symbols []string, gnowebURL string) (server.Result, error) {
	files, err := fetchPackage(ctx, c, path)
	if err != nil {
		return server.Result{}, err
	}
	x := gnosrc.ExtractSymbols(files, symbols)
	if len(x.Found) == 0 {
		return server.Result{}, fmt.Errorf(
			"gno_read: no symbol in %s matches %s; available: %s",
			path, strings.Join(x.Missing, ", "), availableHint(x.Available))
	}

	// Misses and parse failures ride along as the txtar comment section —
	// loud in the result, without corrupting the archive.
	var notes []string
	if len(x.Missing) > 0 {
		notes = append(notes, fmt.Sprintf("// not found: %s (fetch the outline for available symbols)", strings.Join(x.Missing, ", ")))
	}
	for _, e := range x.Errors {
		notes = append(notes, "// parse error: "+e)
	}
	body := x.Body
	if len(notes) > 0 {
		body = strings.Join(notes, "\n") + "\n" + body
	}
	body, _ = budgetBody(body, gnowebURL, budget.ExplicitBudget)
	return server.Result{
		ResourceURI:  "gno://" + path + "#symbols",
		ResourceBody: body,
		ResourceMIME: "text/plain",
	}, nil
}

// fetchPackage reads every file of the package and maps it to the gnosrc
// input shape, with the no-files case rejected up front.
func fetchPackage(ctx context.Context, c chain.Client, path string) ([]gnosrc.File, error) {
	memFiles, err := chain.ReadPackageFiles(ctx, c, path)
	if err != nil {
		return nil, fmt.Errorf("gno_read: %w", err)
	}
	if len(memFiles) == 0 {
		return nil, fmt.Errorf("gno_read: no files found at %q (not a deployed package?)", path)
	}
	files := make([]gnosrc.File, len(memFiles))
	for i, mf := range memFiles {
		files[i] = gnosrc.File{Name: mf.Name, Body: mf.Body}
	}
	return files, nil
}

// availableHint renders the available-symbols list for a miss error, capped
// so a large package cannot turn the error into a flood.
func availableHint(available []string) string {
	const maxNames = 30
	if len(available) <= maxNames {
		return strings.Join(available, ", ")
	}
	return fmt.Sprintf("%s (+%d more — fetch the outline)", strings.Join(available[:maxNames], ", "), len(available)-maxNames)
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
			"type": "string",
			"description": "Source file name within the package (e.g. 'counter.gno'). Alone: that " +
				"file's outline. With full=true: that file verbatim. Cannot be combined with symbols.",
		},
		"symbols": map[string]any{
			"type":  "array",
			"items": map[string]any{"type": "string"},
			"description": "Declaration names to fetch verbatim, with docs and a dependency header " +
				"(e.g. [\"Transfer\", \"Counter.Inc\"] — top-level names; Type.Method for methods). " +
				"Package-scoped. Cannot be combined with file or full.",
		},
		"full": map[string]any{
			"type": "boolean",
			"description": "true returns raw source instead of an outline: with 'file', that file " +
				"verbatim (larger budget); without, the whole package as txtar. Cannot be combined " +
				"with symbols.",
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
