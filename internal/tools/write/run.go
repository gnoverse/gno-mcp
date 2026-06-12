package write

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	gnoclient "github.com/gnolang/gno/gno.land/pkg/gnoclient"
	"github.com/gnoverse/gno-mcp/internal/audit"
	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/keystore"
	"github.com/gnoverse/gno-mcp/internal/server"
	"github.com/gnoverse/gno-mcp/internal/session"
	"github.com/gnoverse/gno-mcp/internal/untrusted"
)

// RegisterRun registers the gno_run tool.
// ks provides agent signers for local profiles; sessionMgr provides active
// sessions for signing; resolver returns the chain client for a given profile;
// alog writes audit entries on every run attempt.
func RegisterRun(s *server.Server, ks *keystore.Keystore, sessionMgr *session.Manager, resolver chain.Resolver, alog *audit.Log) {
	s.Registry().Add(&server.Tool{
		Name: "gno_run",
		Description: "Executes ad-hoc Gno code via vm/MsgRun. The code must be a valid Gno package " +
			"(package main with a main() entry point). " +
			"On local and testnet profiles the agent key signs by default (local: the built-in test1 account; " +
			"testnet: a key from gno_key_generate, funded via gno_faucet_fund). " +
			"Pass identity=session to act as the user instead — that requires an active gnomcp session with allow_run=true (use gno_session_propose with allow_run=true). " +
			"Pass simulate=true to dry-run without spending gas. Required args: " +
			"profile, code. Optional: simulate (bool), identity (\"agent\" or \"session\"). " +
			"The result reports which identity signed; always tell the user which account performed the write.",
		InputSchema: runInputSchema(s),
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
			return runHandler(ctx, args, s, ks, sessionMgr, resolver, alog)
		},
	})
}

func runHandler(
	ctx context.Context,
	args map[string]any,
	s *server.Server,
	ks *keystore.Keystore,
	sessionMgr *session.Manager,
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
		sessionAddr string
		auditResult = "tool_err"
	)
	defer func() {
		alog.Record(audit.Entry{
			Tool:           "gno_run",
			Profile:        profileName,
			ArgsSummary:    argsSummary,
			Result:         auditResult,
			Duration:       time.Since(start).Milliseconds(),
			SessionAddress: sessionAddr,
		})
	}()

	// ---- Validate args

	profileName, profile, err := requireProfile(args, s)
	if err != nil {
		return server.Result{}, err
	}

	code, err := server.StringArg(args, "code")
	if err != nil {
		return server.Result{}, err
	}
	if code == "" {
		return server.Result{}, fmt.Errorf("code: required")
	}

	simulate, err := server.BoolArg(args, "simulate")
	if err != nil {
		return server.Result{}, err
	}

	// ---- Resolve chain client

	c := resolver(profileName)
	if c == nil {
		return server.Result{}, fmt.Errorf("profile %q: no chain client available", profileName)
	}

	// ---- Build args summary for audit

	argsSummary = fmt.Sprintf("code_len=%d", len(code))

	// ---- Dispatch by identity

	var rr chain.RunResult
	identityArg, _ := server.StringArg(args, "identity")
	identity, signerAddr, master, err := dispatchWriteTx(ctx, identityArg, writeTxDispatch{
		tool:        "gno_run",
		noKeyHint:   "run gno_key_generate (or pass identity=session to act as the user)",
		profileName: profileName,
		profile:     profile,
		simulate:    simulate,
		c:           c,
		ks:          ks,
		sessionMgr:  sessionMgr,
		// The session must carry allow_run=true; scope mismatch here means no
		// session authorizes MsgRun at all.
		pickSession: func(ctx context.Context) (chain.Signer, string, error) {
			return sessionMgr.PickSessionForRun(ctx, resolver, profileName)
		},
		mapPickErr: func(pickErr error) error {
			if errors.Is(pickErr, session.ErrNoActiveSession) {
				return &server.ToolError{
					Code: "authentication_required",
					Message: fmt.Sprintf(
						"no active session with allow_run=true for profile %q — use gno_session_propose with allow_run=true",
						profileName,
					),
					Extra: map[string]any{"profile": profileName},
				}
			}
			if _, ok := errors.AsType[*session.ErrScopeMismatch](pickErr); ok {
				return &server.ToolError{
					Code: "authentication_required",
					Message: fmt.Sprintf(
						"active sessions for profile %q do not authorize MsgRun — use gno_session_propose with allow_run=true",
						profileName,
					),
					Extra: map[string]any{"profile": profileName},
				}
			}
			return nil
		},
		agentOp: func(ctx context.Context, signer gnoclient.Signer) error {
			var opErr error
			rr, opErr = c.Run(ctx, signer, code, simulate)
			return opErr
		},
		sessionOp: func(ctx context.Context, signer chain.Signer, master string) error {
			var opErr error
			rr, opErr = c.RunAsUser(ctx, signer, master, code, simulate)
			return opErr
		},
		auditResult: &auditResult,
		sessionAddr: &sessionAddr,
	})
	if err != nil {
		return server.Result{}, err
	}

	return decorateWriteResult(buildRunResult(rr, profileName), identity, signerAddr, master, profile.IsLocal()), nil
}

// buildRunResult constructs the server.Result from a chain.RunResult. MsgRun
// stdout can echo realm-authored state, so the text rendering wraps it in the
// untrusted envelope (source = the profile whose chain ran the code); the
// structured "output" field stays raw (the machine-readable channel,
// documented in docs/security.md §4).
func buildRunResult(rr chain.RunResult, profileName string) server.Result {
	var b strings.Builder
	if rr.Simulated {
		fmt.Fprintf(&b, "Simulated run result\n\n")
	} else {
		fmt.Fprintf(&b, "Run succeeded\n\n")
		fmt.Fprintf(&b, "TxHash:  %s\n", rr.TxHash)
		fmt.Fprintf(&b, "Height:  %d\n", rr.Height)
	}
	fmt.Fprintf(&b, "GasUsed: %d\n", rr.GasUsed)
	if rr.Output != "" {
		fmt.Fprintf(&b, "Output:\n%s\n", untrusted.Wrap(rr.Output, "run_output", profileName))
	}

	return server.Result{
		Text: b.String(),
		StructuredContent: map[string]any{
			"tx_hash":   rr.TxHash,
			"height":    rr.Height,
			"output":    rr.Output,
			"gas_used":  rr.GasUsed,
			"simulated": rr.Simulated,
		},
	}
}

func runInputSchema(s *server.Server) map[string]any {
	props := map[string]any{
		"code": map[string]any{
			"type":        "string",
			"description": "Gno source code to execute. Must be a valid Gno package with a package main declaration and a main() entry point.",
		},
		"simulate": map[string]any{
			"type":        "boolean",
			"description": "When true, dry-run the execution without broadcasting or spending gas. The acting identity still signs (agent key or session), but nothing is broadcast.",
			"default":     false,
		},
		"identity": map[string]any{
			"type":        "string",
			"enum":        []string{"agent", "session"},
			"description": "Who signs: agent (the agent's own key — local test1 or testnet generated key) or session (act as the user via a master-bound session). Default: agent; pass session to act as the user.",
		},
	}
	required := []string{"code"}
	addWritableProfileArg(s, props, &required)
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	}
}
