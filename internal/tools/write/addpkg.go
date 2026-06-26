package write

import (
	"context"
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
			"Gno's semantics and API surface are its own, not Go's — when authoring the .gno source " +
			"to deploy, study existing on-chain packages via gno_read rather than writing from Go intuition. " +
			"The agent identity signs the transaction directly without requiring an active session: " +
			"local profiles use the built-in test1 key; testnet profiles use a key generated via " +
			"gno_key_generate (run that first if no key exists). " +
			"If the supplied file list omits gnomod.toml it is generated automatically. " +
			"The result reports which identity signed (tell the user which account performed the write) and an " +
			"equivalent gnokey command for transparency — illustrative only, since gnomcp already signed and broadcast the tx.",
		InputSchema: addpkgInputSchema(s),
		OutputKind:  server.OutputText,
		Capability:  server.CapWrite,
		SelfAudited: true,
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

	// One audit record per invocation, written on every return path — including the
	// early validation and pre-check denials — because SelfAudited makes the MCP
	// adapter skip its generic line. auditResult defaults to a denial; the dispatch
	// paths overwrite it.
	var (
		profileName string
		argsSummary string
		auditResult = "tool_err"
	)
	defer func() {
		alog.Record(audit.Entry{
			Tool:        "gno_addpkg",
			Profile:     profileName,
			ArgsSummary: argsSummary,
			Result:      auditResult,
			Duration:    time.Since(start).Milliseconds(),
		})
	}()

	// ---- Validate args

	profileName, p, err := requireProfile(args, s)
	if err != nil {
		return server.Result{}, err
	}
	keyName, err := keyArg(args)
	if err != nil {
		return server.Result{}, err
	}

	deployPath, err := server.StringArg(args, "deploy_path")
	if err != nil {
		return server.Result{}, err
	}
	if deployPath == "" {
		return server.Result{}, fmt.Errorf("deploy_path: required")
	}

	simulate, err := server.BoolArg(args, "simulate")
	if err != nil {
		return server.Result{}, err
	}

	files, err := toMemFiles(args["files"])
	if err != nil {
		return server.Result{}, fmt.Errorf("files: %w", err)
	}

	// ---- Resolve chain client

	c := resolver(profileName)
	if c == nil {
		return server.Result{}, fmt.Errorf("profile %q: no chain client available", profileName)
	}

	// ---- Inject gnomod.toml if missing, then sort
	//
	// Inject before signer acquisition so the file count is correct in the audit
	// summary. If deploy_path is a short name that gets expanded below, we
	// regenerate the gnomod.toml body with the expanded path.

	autoGnoMod := !hasGnoMod(files)
	if autoGnoMod {
		files = append(files, &std.MemFile{
			Name: "gnomod.toml",
			Body: gnolang.GenGnoModLatest(deployPath),
		})
	}
	slices.SortFunc(files, func(a, b *std.MemFile) int {
		return strings.Compare(a.Name, b.Name)
	})

	// ---- Build args summary for audit (before the signer pre-check so denials carry it)

	argsSummary = fmt.Sprintf("deploy_path=%s files=%d simulate=%v", deployPath, len(files), simulate)

	// ---- Acquire agent signer (with the testnet unfunded pre-check)

	signer, addr, aerr := acquireAgentSigner(ctx, ks, c, "gno_addpkg",
		"run gno_key_generate to create one", profileName, keyName, p, simulate)
	if aerr != nil {
		return server.Result{}, aerr
	}

	// ---- Expand short deploy_path to address-based namespace
	//
	// When deploy_path contains no "/" it is a short package name (e.g. "hello").
	// Expand it to "gno.land/r/<agentAddr>/<name>" which is always authorized
	// (no namespace registration required) and gnoweb-compatible (hex address,
	// no hyphens). Full paths (containing "/") are passed through unchanged.
	if !strings.Contains(deployPath, "/") {
		deployPath = "gno.land/r/" + addr + "/" + deployPath
		argsSummary = fmt.Sprintf("deploy_path=%s files=%d simulate=%v", deployPath, len(files), simulate)
		if autoGnoMod {
			for _, f := range files {
				if f.Name == "gnomod.toml" {
					f.Body = gnolang.GenGnoModLatest(deployPath)
					break
				}
			}
		}
	}

	// ---- Validate before broadcast
	//
	// A failed addpkg broadcast still burns gas — the node charges for the
	// type-check or deploy-gate rejection at DeliverTx — which can strand a
	// freshly-funded key. Simulate first so authoring bugs and unmet deploy
	// gates (CLA, namespace) fail at zero cost; only then broadcast.
	if !simulate {
		if _, verr := c.AddPackage(ctx, signer, deployPath, files, true); verr != nil {
			auditResult = "validate_err"
			return server.Result{}, withCLAHint(fmt.Errorf("gno_addpkg validation (no gas spent): %w", verr))
		}
	}

	// ---- Deploy

	res, deployErr := c.AddPackage(ctx, signer, deployPath, files, simulate)
	if deployErr != nil {
		errPrefix := "gno_addpkg broadcast"
		auditResult = "broadcast_err"
		if simulate {
			errPrefix = "gno_addpkg simulate"
			auditResult = "sim_err"
		}
		return server.Result{}, withCLAHint(fmt.Errorf("%s: %w", errPrefix, deployErr))
	}

	auditResult = "ok"
	if simulate {
		auditResult = "sim"
	}

	// ---- Build result text

	var b strings.Builder
	fmt.Fprintln(&b, signedByLine("agent", addr, "", p.IsLocal()))
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

	// Hand the agent the exact gnoweb URL of the deployed realm so it need not
	// guess the host. Only on a real deploy and only when the profile has a
	// usable gnoweb host (a local node has none).
	var viewURL string
	if !res.Simulated {
		viewURL = p.RealmViewURL(deployPath)
	}
	if viewURL != "" {
		fmt.Fprintf(&b, "View:    %s\n", viewURL)
	}

	gkCmd := chain.GnokeyCmd{
		Sub: "addpkg", PkgPath: deployPath,
		MaxDeposit: fmt.Sprintf("%dugnot", chain.DefaultMaxDepositUgnot),
		RPC:        p.RPCURL, ChainID: p.ChainID, Signer: addr, Simulate: simulate,
		GasFeeUgnot: res.GasFeeUgnot,
	}.String()
	sc := map[string]any{
		"identity":       "agent",
		"signer_address": addr,
		"tx_hash":        res.TxHash,
		"height":         res.Height,
		"gas_used":       res.GasUsed,
		"simulated":      res.Simulated,
	}
	if viewURL != "" {
		sc["gnoweb_url"] = viewURL
	}
	return attachGnokeyCmd(server.Result{Text: b.String(), StructuredContent: sc}, gkCmd), nil
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
			"type": "string",
			"description": "Full package path (e.g. \"gno.land/r/myname/hello\") or a short package name (e.g. \"hello\"). " +
				"When a short name is given (no \"/\"), the path is automatically expanded to " +
				"\"gno.land/r/<agent_address>/<name>\" — this is always authorized and gnoweb-compatible. " +
				"Use a full path only when deploying to a registered namespace.",
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
	addOptionalKeyArg(props)
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	}
}
