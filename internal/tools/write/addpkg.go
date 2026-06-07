package write

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/gnolang/gno/gnovm/pkg/gnolang"
	"github.com/gnolang/gno/tm2/pkg/std"

	"github.com/gnoverse/gno-mcp/internal/audit"
	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/keystore"
	"github.com/gnoverse/gno-mcp/internal/server"
)

// RegisterAddPkg registers the gno_addpkg tool.
// ks provides agent signers per profile; resolver returns the chain client for
// a given profile; alog writes audit entries on every deploy attempt.
func RegisterAddPkg(s *server.Server, ks *keystore.Keystore, resolver chain.Resolver, alog *audit.Log) {
	s.Registry().Add(&server.Tool{
		Name: "gno_addpkg",
		Description: "Deploys a new Gno package or realm to the chain via vm/MsgAddPackage. " +
			"The agent identity (test1 on local/dev chains) signs the transaction directly " +
			"without requiring an active session. If the supplied file list omits gnomod.toml " +
			"it is generated automatically. The result reports which identity signed; always " +
			"tell the user which account performed the write.",
		InputSchema: addpkgInputSchema(s),
		OutputKind:  server.OutputText,
		Capability:  server.CapWrite,
		Annotations: server.Annotations{
			ReadOnly:    false,
			Destructive: true,
			Idempotent:  false,
			OpenWorld:   true,
		},
		Handler: func(ctx context.Context, args map[string]any) (server.Result, error) {
			return addpkgHandler(ctx, args, s, ks, resolver, alog)
		},
	})
}

func addpkgHandler(
	ctx context.Context,
	args map[string]any,
	s *server.Server,
	ks *keystore.Keystore,
	resolver chain.Resolver,
	alog *audit.Log,
) (server.Result, error) {
	start := time.Now()

	// ---- Validate args

	profileName, err := stringArg(args, "profile")
	if err != nil {
		return server.Result{}, err
	}
	if profileName == "" {
		return server.Result{}, fmt.Errorf("profile: required — pick one of the configured profiles")
	}

	deployPath, err := stringArg(args, "deploy_path")
	if err != nil {
		return server.Result{}, err
	}
	if deployPath == "" {
		return server.Result{}, fmt.Errorf("deploy_path: required")
	}

	simulate, err := boolArg(args, "simulate")
	if err != nil {
		return server.Result{}, err
	}

	files, err := toMemFiles(args["files"])
	if err != nil {
		return server.Result{}, fmt.Errorf("files: %w", err)
	}

	// ---- Resolve profile

	p, ok := s.Config().Profiles[profileName]
	if !ok {
		return server.Result{}, fmt.Errorf("profile %q: not found", profileName)
	}

	// ---- Acquire agent signer

	signer, err := ks.SignerForProfile(p)
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
		return server.Result{}, fmt.Errorf("gno_addpkg: signer: %w", err)
	}

	info, err := signer.Info()
	if err != nil {
		return server.Result{}, fmt.Errorf("gno_addpkg: signer info: %w", err)
	}
	addr := info.GetAddress().String()

	// ---- Inject gnomod.toml if missing, then sort

	if !hasGnoMod(files) {
		files = append(files, &std.MemFile{
			Name: "gnomod.toml",
			Body: gnolang.GenGnoModLatest(deployPath),
		})
	}
	slices.SortFunc(files, func(a, b *std.MemFile) int {
		return strings.Compare(a.Name, b.Name)
	})

	// ---- Resolve chain client

	c := resolver(profileName)
	if c == nil {
		return server.Result{}, fmt.Errorf("profile %q: no chain client available", profileName)
	}

	// ---- Build args summary for audit

	argsSummary := fmt.Sprintf("deploy_path=%s files=%d simulate=%v", deployPath, len(files), simulate)

	// ---- Deploy

	res, deployErr := c.AddPackage(ctx, signer, deployPath, files, simulate)
	if deployErr != nil {
		result := "broadcast_err"
		errPrefix := "gno_addpkg broadcast"
		if simulate {
			result = "sim_err"
			errPrefix = "gno_addpkg simulate"
		}
		_ = alog.Append(audit.Entry{
			Tool:        "gno_addpkg",
			Profile:     profileName,
			ArgsSummary: argsSummary,
			Result:      result,
			Duration:    time.Since(start).Milliseconds(),
		})
		return server.Result{}, fmt.Errorf("%s: %w", errPrefix, deployErr)
	}

	// ---- Audit

	auditResult := "ok"
	if simulate {
		auditResult = "sim"
	}
	_ = alog.Append(audit.Entry{
		Tool:        "gno_addpkg",
		Profile:     profileName,
		ArgsSummary: argsSummary,
		Result:      auditResult,
		Duration:    time.Since(start).Milliseconds(),
	})

	// ---- Build result text

	var b strings.Builder
	fmt.Fprintln(&b, signedByLine("agent", addr, ""))
	fmt.Fprintln(&b)
	if res.Simulated {
		fmt.Fprintln(&b, "AddPackage simulated (no broadcast)")
	} else {
		fmt.Fprintln(&b, "AddPackage succeeded")
		fmt.Fprintf(&b, "TxHash:  %s\n", res.TxHash)
		fmt.Fprintf(&b, "Height:  %d\n", res.Height)
	}
	fmt.Fprintf(&b, "GasUsed: %d\n", res.GasUsed)
	if simulate {
		fmt.Fprintln(&b, "(simulate=true — transaction was not broadcast)")
	}

	return server.Result{
		Text: b.String(),
		StructuredContent: map[string]any{
			"identity":       "agent",
			"signer_address": addr,
			"tx_hash":        res.TxHash,
			"height":         res.Height,
			"gas_used":       res.GasUsed,
			"simulated":      res.Simulated,
		},
	}, nil
}

// toMemFiles converts the raw JSON-decoded "files" arg into []*std.MemFile.
// Expects []any of map[string]any{"name": string, "body": string}.
func toMemFiles(raw any) ([]*std.MemFile, error) {
	if raw == nil {
		return nil, fmt.Errorf("required: provide at least one file")
	}
	rawSlice, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("expected array, got %T", raw)
	}
	if len(rawSlice) == 0 {
		return nil, fmt.Errorf("required: provide at least one file")
	}
	out := make([]*std.MemFile, 0, len(rawSlice))
	for i, elem := range rawSlice {
		m, ok := elem.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("[%d]: expected object, got %T", i, elem)
		}
		name, ok := m["name"].(string)
		if !ok {
			return nil, fmt.Errorf("[%d].name: expected string", i)
		}
		body, ok := m["body"].(string)
		if !ok {
			return nil, fmt.Errorf("[%d].body: expected string", i)
		}
		out = append(out, &std.MemFile{Name: name, Body: body})
	}
	return out, nil
}

// hasGnoMod reports whether any file in files has Name == "gnomod.toml".
func hasGnoMod(files []*std.MemFile) bool {
	for _, f := range files {
		if f.Name == "gnomod.toml" {
			return true
		}
	}
	return false
}

func addpkgInputSchema(s *server.Server) map[string]any {
	props := map[string]any{
		"deploy_path": map[string]any{
			"type":        "string",
			"description": "Fully-qualified package path (e.g. \"gno.land/r/<ns>/<pkg>\").",
		},
		"files": map[string]any{
			"type":        "array",
			"description": "Source files to deploy. Each element must have \"name\" and \"body\" string fields. gnomod.toml is generated automatically if omitted.",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
					"body": map[string]any{"type": "string"},
				},
				"required": []string{"name", "body"},
			},
		},
		"simulate": map[string]any{
			"type":        "boolean",
			"description": "When true, dry-run the deployment without broadcasting or spending gas.",
			"default":     false,
		},
	}
	required := []string{"deploy_path", "files"}
	addAgentProfileArg(s, props, &required)
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	}
}
