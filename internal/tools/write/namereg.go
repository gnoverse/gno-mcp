package write

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	gnoclient "github.com/gnolang/gno/gno.land/pkg/gnoclient"
	"github.com/gnoverse/gno-mcp/internal/audit"
	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/keystore"
	"github.com/gnoverse/gno-mcp/internal/server"
	"github.com/gnoverse/gno-mcp/internal/session"
)

const (
	nameregRealm       = "gno.land/r/sys/namereg/v1"
	nameregFunc        = "Register"
	nameregDefaultSend = "200000000ugnot"
)

// namePattern is the client-side validation pattern for namereg names.
// Hyphens are explicitly excluded because gnoweb rejects them in paths.
var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// RegisterNameReg registers the gno_namereg tool.
// ks provides agent signers; sessionMgr provides active sessions; resolver
// returns the chain client for a given profile; alog writes audit entries.
func RegisterNameReg(s *server.Server, ks *keystore.Keystore, sessionMgr *session.Manager, resolver chain.Resolver, alog *audit.Log) {
	s.Registry().Add(&server.Tool{
		Name: "gno_namereg",
		Description: `Register a name in the Gno name registry (gno.land/r/sys/namereg/v1).

The name becomes a namespace for package deployment: after registration,
you can deploy to "gno.land/r/<name>/<pkg>" and it will be gnoweb-compatible.

Name requirements:
- Pattern: ^[a-z][a-z0-9_]*$ (no hyphens — gnoweb rejects them)
- Length: 5–13 chars + up to 3 trailing digits (namereg format)
- Must not already be taken

Registration costs gas (~27M) and a coin payment. The tool checks your
balance before attempting registration.

Use identity=session to register under your own address (recommended for
permanent names). identity=agent registers under the agent key.`,
		InputSchema: nameregInputSchema(s),
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
			return nameregHandler(ctx, args, s, ks, sessionMgr, resolver, alog)
		},
	})
}

func nameregHandler(
	ctx context.Context,
	args map[string]any,
	s *server.Server,
	ks *keystore.Keystore,
	sessionMgr *session.Manager,
	resolver chain.Resolver,
	alog *audit.Log,
) (server.Result, error) {
	start := time.Now()

	var (
		profileName string
		sessionAddr string
		auditResult = "tool_err"
	)
	defer func() {
		alog.Record(audit.Entry{
			Tool:           "gno_namereg",
			Profile:        profileName,
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
	keyName, err := keyArg(args)
	if err != nil {
		return server.Result{}, err
	}

	name, err := server.StringArg(args, "name")
	if err != nil {
		return server.Result{}, err
	}
	if name == "" {
		return server.Result{}, fmt.Errorf("name: required")
	}

	// Hyphen check first: give a targeted error before the general pattern check
	if strings.Contains(name, "-") {
		return server.Result{}, &server.ToolError{
			Code:    "invalid_name",
			Message: "Name contains a hyphen which is not allowed — gnoweb paths reject hyphens. Use underscores or omit the separator.",
			Extra:   map[string]any{"name": name},
		}
	}

	if !namePattern.MatchString(name) {
		return server.Result{}, &server.ToolError{
			Code:    "invalid_name",
			Message: fmt.Sprintf("name %q does not match required pattern ^[a-z][a-z0-9_]*$ — use lowercase letters, digits, or underscores; must start with a letter", name),
			Extra:   map[string]any{"name": name},
		}
	}

	send, err := server.StringArg(args, "send")
	if err != nil {
		return server.Result{}, err
	}
	if send == "" {
		send = nameregDefaultSend
	}
	if err := chain.ValidateSendCoins(send); err != nil {
		return server.Result{}, &server.ToolError{
			Code:    "invalid_send",
			Message: fmt.Sprintf("send %q is not a valid coin amount — use an amount like %q", send, nameregDefaultSend),
			Extra:   map[string]any{"send": send},
		}
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

	// ---- Dispatch by identity

	var cr chain.CallResult
	identityArg, _ := server.StringArg(args, "identity")
	identity, signerAddr, master, err := dispatchWriteTx(ctx, identityArg, writeTxDispatch{
		tool:        "gno_namereg",
		noKeyHint:   "run gno_key_generate (or pass identity=session to register under your own address)",
		profileName: profileName,
		keyName:     keyName,
		profile:     profile,
		simulate:    simulate,
		c:           c,
		ks:          ks,
		sessionMgr:  sessionMgr,
		pickSession: func(ctx context.Context) (chain.Signer, string, error) {
			return sessionMgr.PickSessionForProfile(ctx, resolver, profileName, nameregRealm)
		},
		mapPickErr: func(pickErr error) error {
			if errors.Is(pickErr, session.ErrNoActiveSession) {
				return &server.ToolError{
					Code: "authentication_required",
					Message: fmt.Sprintf(
						"no active session for profile %q — use gno_session_propose to create one",
						profileName,
					),
					Extra: map[string]any{"profile": profileName},
				}
			}
			if scopeErr, ok := errors.AsType[*session.ErrScopeMismatch](pickErr); ok {
				return &server.ToolError{
					Code: "scope_mismatch",
					Message: fmt.Sprintf(
						"realm %q is not covered by any active session for profile %q — "+
							"use gno_session_propose with allow_paths=[%q]",
						nameregRealm, profileName, nameregRealm,
					),
					Extra: map[string]any{
						"profile":         profileName,
						"realm":           nameregRealm,
						"available_paths": scopeErr.AvailablePaths,
					},
				}
			}
			return nil
		},
		agentOp: func(ctx context.Context, signer gnoclient.Signer) error {
			var opErr error
			cr, opErr = c.Call(ctx, signer, nameregRealm, nameregFunc, []string{name}, send, simulate)
			return opErr
		},
		sessionOp: func(ctx context.Context, signer chain.Signer, master string) (int64, error) {
			var opErr error
			cr, opErr = c.CallAsUser(ctx, signer, master, nameregRealm, nameregFunc, []string{name}, send, simulate)
			return cr.GasFeeUgnot, opErr
		},
		auditResult: &auditResult,
		sessionAddr: &sessionAddr,
	})
	if err != nil {
		return server.Result{}, err
	}

	gkCmd := chain.GnokeyCmd{
		Sub: "call", PkgPath: nameregRealm, Func: nameregFunc, Args: []string{name}, Send: send,
		RPC: profile.RPCURL, ChainID: profile.ChainID,
		Signer: signerAddr, Master: master, Simulate: simulate,
		GasFeeUgnot: cr.GasFeeUgnot,
	}.String()

	namespace := "gno.land/r/" + name + "/"
	res := buildNameregResult(cr, name, namespace)
	return attachGnokeyCmd(
		decorateWriteResult(res, identity, signerAddr, master, profile.IsLocal()),
		gkCmd,
	), nil
}

// buildNameregResult constructs the server.Result from a successful namereg
// registration. On simulate it marks the result accordingly.
func buildNameregResult(cr chain.CallResult, name, namespace string) server.Result {
	var b strings.Builder
	if cr.Simulated {
		fmt.Fprintf(&b, "Simulated namereg registration\n\n")
	} else {
		fmt.Fprintf(&b, "Name registered\n\n")
		fmt.Fprintf(&b, "TxHash:    %s\n", cr.TxHash)
		fmt.Fprintf(&b, "Height:    %d\n", cr.Height)
	}
	fmt.Fprintf(&b, "GasUsed:   %d\n", cr.GasUsed)
	fmt.Fprintf(&b, "Name:      %s\n", name)
	fmt.Fprintf(&b, "Namespace: %s\n", namespace)
	fmt.Fprintf(&b, "\nYou can now deploy packages to %s<pkg>", namespace)

	return server.Result{
		Text: b.String(),
		StructuredContent: map[string]any{
			"name":      name,
			"namespace": namespace,
			"message":   "Name registered. You can now deploy packages to " + namespace + "<pkg>",
			"tx_hash":   cr.TxHash,
			"height":    cr.Height,
			"gas_used":  cr.GasUsed,
			"simulated": cr.Simulated,
		},
	}
}

func nameregInputSchema(s *server.Server) map[string]any {
	props := map[string]any{
		"name": map[string]any{
			"type": "string",
			"description": "The name to register in the Gno name registry. " +
				"Pattern: ^[a-z][a-z0-9_]*$ — lowercase letters, digits, and underscores only; must start with a letter. " +
				"No hyphens — gnoweb rejects them in paths. " +
				"Typical length 5–13 chars + up to 3 trailing digits. e.g. \"myproject\" or \"my_org\".",
		},
		"send": map[string]any{
			"type": "string",
			"description": fmt.Sprintf(
				"Coins to send with the registration call (the namereg realm charges a fee). "+
					"Default: %q. Adjust if the current registration price differs.", nameregDefaultSend),
			"default": nameregDefaultSend,
		},
		"simulate": map[string]any{
			"type":        "boolean",
			"description": "When true, dry-run the registration without broadcasting or spending gas. Useful to verify the name is accepted before paying.",
			"default":     false,
		},
		"identity": map[string]any{
			"type": "string",
			"enum": []string{"agent", "session"},
			"description": "Who signs: agent (the agent's own key) or session (act as the user via a master-bound session). " +
				"Use identity=session to register the name under your own address (recommended for permanent names). " +
				"Default: agent.",
		},
	}
	required := []string{"name"}
	addWritableProfileArg(s, props, &required)
	addOptionalKeyArg(props)
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	}
}
