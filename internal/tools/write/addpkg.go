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
	"github.com/gnoverse/gno-mcp/internal/tools/parked"
	"github.com/gnoverse/gno-mcp/internal/untrusted"
)

// On an inert chain the deploy parks until a package approver enables it.
// onyx's approver enables a good package about five seconds after the deploy
// commits; the wait covers that plus the approver's ten-second verify budget.
const (
	parkWaitTimeout  = 30 * time.Second
	parkPollInterval = 2 * time.Second
)

// parkWait bounds the wait for a parked package to go live.
type parkWait struct {
	timeout, interval time.Duration
}

// RegisterAddPkg registers the gno_addpkg tool.
// ks provides agent signers per profile; resolver returns the chain client for
// a given profile; alog writes audit entries on every deploy attempt.
func RegisterAddPkg(s *server.Server, ks *keystore.Keystore, resolver chain.Resolver, alog *audit.Log) {
	registerAddPkg(s, ks, resolver, alog, parkWait{timeout: parkWaitTimeout, interval: parkPollInterval})
}

func registerAddPkg(s *server.Server, ks *keystore.Keystore, resolver chain.Resolver, alog *audit.Log, wait parkWait) {
	s.Registry().Add(&server.Tool{
		Name: "gno_addpkg",
		Description: "Deploys a new Gno package or realm to the chain via vm/MsgAddPackage. " +
			"Gno's semantics and API surface are its own, not Go's — when authoring the .gno source " +
			"to deploy, study existing on-chain packages via gno_read rather than writing from Go intuition. " +
			"The agent identity signs the transaction directly without requiring an active session: " +
			"local profiles use the built-in test1 key; testnet profiles use a key generated via " +
			"gno_key_generate (run that first if no key exists). " +
			"If the supplied file list omits gnomod.toml it is generated automatically. " +
			"Every broadcast deploy reports package_status, and the deployed code is callable only when it is live. " +
			"On a chain running the inert code-submission policy the deploy parks until the chain's package approver " +
			"enables it: the tool waits up to 30s and reports live, inert (PARKED, with the chain's reason " +
			"and how to recover), redeploy_parked (the previous version still serves) or unknown. There the result also " +
			"carries code_submission_policy, which describes the chain rather than this package, and a simulation " +
			"does not type-check the code. " +
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
			return addpkgHandler(ctx, args, s, ks, resolver, alog, wait)
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
	wait parkWait,
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

	// A short deploy_path expands to the agent's own-address namespace below,
	// which requires regenerating gnomod.toml for the expanded path. A
	// caller-supplied gnomod.toml names a module path this handler must not
	// rewrite, so the combination would reach the chain with a module line that
	// contradicts the deploy path — refuse it before anything is signed.
	shortName := !strings.Contains(deployPath, "/")
	if shortName && hasGnoMod(files) {
		return server.Result{}, fmt.Errorf(
			"deploy_path %q: a short package name cannot be combined with a caller-supplied gnomod.toml — pass the full package path, or omit gnomod.toml to have it generated",
			deployPath)
	}

	// ---- Resolve chain client

	c := resolver(profileName)
	if c == nil {
		return server.Result{}, fmt.Errorf("profile %q: no chain client available", profileName)
	}

	// ---- Inject gnomod.toml if missing, then sort
	//
	// Inject before signer acquisition so the file count is correct in the audit
	// summary. If deploy_path is a short name that gets expanded below, the
	// generated body is regenerated with the expanded path (a caller-supplied
	// gnomod.toml was already refused for that case).

	if !hasGnoMod(files) {
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

	// ---- Read the code-submission policy (before anything is signed)

	policy, err := c.SubmissionPolicy(ctx)
	if err != nil {
		return server.Result{}, fmt.Errorf("gno_addpkg: read the chain's code submission policy (nothing signed): %w", err)
	}
	inert := policy == chain.SubmissionPolicyInert

	// ---- Acquire agent signer (with the testnet unfunded pre-check)

	signer, addr, aerr := acquireAgentSigner(ctx, ks, c, "gno_addpkg",
		"run gno_key_generate to create one", profileName, keyName, p, simulate)
	if aerr != nil {
		return server.Result{}, aerr
	}

	// ---- Expand short deploy_path to address-based namespace
	//
	// A short name (e.g. "hello") expands to "gno.land/r/<agentAddr>/<name>",
	// which is always authorized (no namespace registration required) and
	// gnoweb-compatible (bech32 addresses are lowercase alphanumeric — gnoweb's
	// path regex rejects hyphens, so registered names can 404 where an address
	// path works). Full paths (containing "/") pass through unchanged.
	if shortName {
		deployPath = "gno.land/r/" + addr + "/" + deployPath
		argsSummary = fmt.Sprintf("deploy_path=%s files=%d simulate=%v", deployPath, len(files), simulate)
		for _, f := range files {
			if f.Name == "gnomod.toml" {
				f.Body = gnolang.GenGnoModLatest(deployPath)
				break
			}
		}
	}

	// ---- Validate before broadcast
	//
	// A failed addpkg broadcast still burns gas — the node charges for the
	// type-check or deploy-gate rejection at DeliverTx — which can strand a
	// freshly-funded key. Simulate first so authoring bugs and unmet deploy
	// gates (CLA, namespace) fail at zero cost; only then broadcast. An inert
	// chain parks the package without type-checking it, so there the
	// simulation catches gate and funding failures but not authoring bugs.
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

	// ---- On an inert chain, wait for the approver to enable the package

	var (
		status string // stays "" on a dry run
		meta   chain.PackageMeta
	)
	switch {
	case res.Simulated:
	case inert:
		var werr error
		meta, werr = waitLive(ctx, c, deployPath, wait)
		status = deployStatus(meta, werr)
	default:
		// A chain that runs what it accepts parks nothing.
		status = chain.PackageLive
	}

	switch {
	case simulate:
		auditResult = "sim"
	case status == chain.PackageInert, status == packageStatusRedeployParked:
		auditResult = "parked"
	case status == packageStatusUnknown:
		auditResult = "status_unknown"
	default:
		auditResult = "ok"
	}

	// ---- Build result text

	var b strings.Builder
	fmt.Fprintln(&b, signedByLine("agent", addr, "", p.IsLocal()))
	fmt.Fprintln(&b)
	switch {
	case res.Simulated:
		fmt.Fprintln(&b, "AddPackage simulated (no broadcast)")
	case status == chain.PackageInert:
		fmt.Fprintln(&b, "AddPackage submitted: PARKED, not live")
	case status == packageStatusRedeployParked:
		fmt.Fprintln(&b, "AddPackage submitted: redeploy PARKED; the previous version is still live")
	case status == packageStatusUnknown:
		fmt.Fprintln(&b, "AddPackage submitted: status unknown")
	default:
		fmt.Fprintln(&b, "AddPackage succeeded")
	}
	if !res.Simulated {
		fmt.Fprintf(&b, "TxHash:  %s\n", res.TxHash)
		fmt.Fprintf(&b, "Height:  %d\n", res.Height)
	}
	fmt.Fprintf(&b, "GasUsed: %d\n", res.GasUsed)
	// nextSteps also goes into the structured content: a client may hand the
	// model that alone.
	var nextSteps string
	if simulate {
		fmt.Fprintln(&b, "(simulate=true — transaction was not broadcast)")
		if inert {
			nextSteps = "This chain parks deploys (code submission policy inert) without type-checking them, " +
				"so this simulation did not type-check the code: lint it against the chain's release before deploying."
			fmt.Fprintln(&b, nextSteps)
		}
	}
	switch status {
	case chain.PackageLive:
		if inert {
			fmt.Fprintln(&b, "Package: live, enabled by the chain's package approver")
		} else {
			fmt.Fprintln(&b, "Package: live")
		}
	case chain.PackageInert:
		fmt.Fprintf(&b, "Package: parked. The chain accepted the deploy and had not enabled it after %s.\n", wait.timeout)
		writeReason(&b, meta.Reason, deployPath)
		nextSteps = parked.NextSteps + " Until it is enabled, every read and call answers it like a package that was never deployed; " +
			"gno_read on the path reports package_parked while it waits."
		fmt.Fprintln(&b, nextSteps)
	case packageStatusRedeployParked:
		fmt.Fprintf(&b, "Package: the chain accepted this redeploy and had not enabled it after %s.\n", wait.timeout)
		writeReason(&b, meta.Reason, deployPath)
		nextSteps = "Reads and calls still reach the previous version. Unless the reason says this submission can never be enabled: " +
			parked.NextSteps
		fmt.Fprintln(&b, nextSteps)
	case packageStatusUnknown:
		nextSteps = "The chain did not report the package as live or parked; this chain parks deploys " +
			"until an approver enables them, so read the path with gno_read before calling it."
		fmt.Fprintln(&b, "Package: status unknown. "+nextSteps)
	}

	// Hand the agent the exact gnoweb URL of the deployed realm so it need not
	// guess the host. Only on a real deploy that is live, and only when the
	// profile has a usable gnoweb host (a local node has none).
	var viewURL string
	if status == chain.PackageLive {
		viewURL = p.RealmViewURL(deployPath)
	}
	if viewURL != "" {
		fmt.Fprintf(&b, "View:    %s\n", viewURL)
	}

	gkCmd := chain.GnokeyCmd{
		Sub: "addpkg", PkgPath: deployPath,
		MaxDeposit: fmt.Sprintf("%dugnot", chain.DefaultMaxDepositUgnot),
		RPC:        p.RPCURL, ChainID: p.ChainID, Signer: addr, Simulate: simulate,
		GasFeeUgnot: res.GasFeeUgnot, GasWanted: res.GasWanted,
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
	if inert {
		sc["code_submission_policy"] = policy
	}
	if status != "" {
		sc["package_status"] = status
	}
	if meta.Reason != "" && (status == chain.PackageInert || status == packageStatusRedeployParked) {
		sc["package_reason"] = meta.Reason
	}
	if nextSteps != "" {
		sc["next_steps"] = nextSteps
	}
	return attachGnokeyCmd(server.Result{Text: b.String(), StructuredContent: sc}, gkCmd), nil
}

// Deploy outcomes on an inert chain beyond the chain's own live and inert.
const (
	// packageStatusRedeployParked: a redeploy parked over a live private realm,
	// whose previous version keeps serving.
	packageStatusRedeployParked = "redeploy_parked"
	// packageStatusUnknown: the chain reported the package as neither live nor
	// parked, or never answered.
	packageStatusUnknown = "unknown"
)

// deployStatus classifies a landed deploy on an inert chain from the last
// package status the chain gave (err set when it gave none).
func deployStatus(m chain.PackageMeta, err error) string {
	switch {
	case err != nil:
		return packageStatusUnknown
	case m.Live():
		return chain.PackageLive
	case m.Status == chain.PackageInert:
		return chain.PackageInert
	case m.Status == chain.PackageLive && m.Pending:
		return packageStatusRedeployParked
	default:
		return packageStatusUnknown
	}
}

// writeReason writes the chain's reason a package is not live, enveloped as
// the chain-authored text it is; the chain may give none.
func writeReason(b *strings.Builder, reason, path string) {
	if reason != "" {
		fmt.Fprintf(b, "Reason:\n%s\n", untrusted.Wrap(reason, "package_reason", path))
	}
}

// waitLive polls the package at path until it is live, w.timeout passes or ctx
// ends, and returns the last status the chain gave. It errors only when the
// chain answered none of the polls.
func waitLive(ctx context.Context, c chain.Client, path string, w parkWait) (chain.PackageMeta, error) {
	deadline := time.Now().Add(w.timeout)
	var (
		last     chain.PackageMeta
		answered bool
		lastErr  error
	)
	settle := func() (chain.PackageMeta, error) {
		if !answered {
			return chain.PackageMeta{}, lastErr
		}
		return last, nil
	}
	for {
		m, err := c.PackageMeta(ctx, path)
		if err != nil {
			lastErr = err
		} else {
			last, answered = m, true
			if m.Live() {
				return last, nil
			}
		}
		left := time.Until(deadline)
		if left <= 0 {
			return settle()
		}
		select {
		case <-ctx.Done():
			return settle()
		case <-time.After(min(w.interval, left)):
		}
	}
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
			"description": "When true, dry-run the deployment without broadcasting or spending gas. On a chain running the inert code-submission policy the dry run does not type-check the code.",
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
