package write

import (
	"context"
	"fmt"
	"strings"

	"github.com/gnoverse/gno-mcp/internal/server"
	"github.com/gnoverse/gno-mcp/internal/session"
)

// RegisterSessionPropose registers the gno_session_propose tool.
// sessionMgr holds pending session state; it is shared with the write
// tools so they can pick up activated sessions on the next call.
func RegisterSessionPropose(s *server.Server, sessionMgr *session.Manager) {
	s.Registry().Add(&server.Tool{
		Name: "gno_session_propose",
		Description: "Proposes a new chain-bounded session for the given profile by generating " +
			"an ephemeral session keypair locally and emitting the gnokey command the user must " +
			"run to authorize it. Use when an agent needs to perform a write but no active session " +
			"covers the target realm. Returns the proposed scope, the bech32 session address, and " +
			"a copy-paste-ready gnokey command. Does NOT broadcast anything — the user's gnokey " +
			"signs the MsgCreateSession from their own machine. Required args: profile, allow_paths " +
			"(non-empty array). Optional: spend_limit (string like \"1000000ugnot\"), expires_in " +
			"(Go duration string like \"24h\").",
		InputSchema: sessionProposeInputSchema(s),
		OutputKind:  server.OutputText,
		Capability:  server.CapWritePrep,
		Annotations: server.Annotations{
			ReadOnly:    false,
			Destructive: false,
			Idempotent:  true,
			OpenWorld:   false,
		},
		Handler: func(ctx context.Context, args map[string]any) (server.Result, error) {
			return sessionProposeHandler(ctx, args, s, sessionMgr)
		},
	})
}

func sessionProposeHandler(
	ctx context.Context,
	args map[string]any,
	s *server.Server,
	sessionMgr *session.Manager,
) (server.Result, error) {
	profileName, err := stringArg(args, "profile")
	if err != nil {
		return server.Result{}, err
	}
	if profileName == "" {
		return server.Result{}, fmt.Errorf("profile: required — pick one of the configured profiles")
	}

	allowPaths, err := stringSliceArg(args, "allow_paths")
	if err != nil {
		return server.Result{}, err
	}
	if len(allowPaths) == 0 {
		return server.Result{}, fmt.Errorf(
			"allow_paths: at least one realm path required (e.g. [\"gno.land/r/myorg/blog\"])",
		)
	}

	spendLimit, err := stringArg(args, "spend_limit")
	if err != nil {
		return server.Result{}, err
	}
	expiresIn, err := stringArg(args, "expires_in")
	if err != nil {
		return server.Result{}, err
	}

	profile, ok := s.Config().Profiles[profileName]
	if !ok {
		return server.Result{}, fmt.Errorf("profile %q: not found", profileName)
	}
	if !profile.AllowDangerousTools {
		return server.Result{}, &server.ToolError{
			Code:    "dangerous_disabled",
			Message: fmt.Sprintf("profile %q: allow-dangerous-tools is not set — edit profiles.toml to enable write tools", profileName),
			Extra:   map[string]any{"profile": profileName},
		}
	}

	scopeArgs := session.ScopeArgs{
		AllowPaths: allowPaths,
		SpendLimit: spendLimit,
		ExpiresIn:  expiresIn,
	}
	scope, warnings, err := session.ResolveScope(scopeArgs, &profile)
	if err != nil {
		return server.Result{}, fmt.Errorf("gno_session_propose: resolve scope: %w", err)
	}

	kp, err := session.NewKeypair()
	if err != nil {
		return server.Result{}, fmt.Errorf("gno_session_propose: generate keypair: %w", err)
	}

	if _, err := sessionMgr.AddPending(profileName, kp, scope); err != nil {
		return server.Result{}, fmt.Errorf("gno_session_propose: persist pending session: %w", err)
	}

	cmd := session.FormatGnokeyCreateCommand(&profile, kp.PubkeyBech32(), scope)

	var b strings.Builder
	fmt.Fprintf(&b, "Session proposed for profile %q.\n\n", profileName)
	fmt.Fprintf(&b, "Proposed scope:\n")
	fmt.Fprintf(&b, "  - allow_paths: %s\n", strings.Join(scope.AllowPaths, ", "))
	if scope.SpendLimit != "" {
		fmt.Fprintf(&b, "  - spend_limit: %s\n", scope.SpendLimit)
	}
	if scope.ExpiresIn > 0 {
		fmt.Fprintf(&b, "  - expires_in: %s\n", scope.ExpiresIn)
	}
	fmt.Fprintf(&b, "  - session_address: %s\n", kp.Address())
	if len(warnings) > 0 {
		b.WriteString("\n")
		for _, w := range warnings {
			fmt.Fprintf(&b, "%s\n", w)
		}
	}
	fmt.Fprintf(&b, "\nTo authorize, run this in a terminal where your master key is available:\n\n")
	fmt.Fprintf(&b, "```\n%s\n```\n\n", cmd)
	fmt.Fprintf(&b,
		"After you run that, retry your original tool call. gnomcp will detect the active\n"+
			"session on chain and use it to sign.\n",
	)

	return server.Result{
		Text: b.String(),
		StructuredContent: map[string]any{
			"state":           "pending",
			"profile":         profileName,
			"session_address": kp.Address(),
			"session_pubkey":  kp.PubkeyBech32(),
			"scope": map[string]any{
				"allow_paths": scope.AllowPaths,
				"spend_limit": scope.SpendLimit,
				"expires_in":  scope.ExpiresIn,
			},
			"auth_command":   cmd,
			"clamp_warnings": warnings,
		},
	}, nil
}

func sessionProposeInputSchema(s *server.Server) map[string]any {
	props := map[string]any{
		"allow_paths": map[string]any{
			"type":        "array",
			"items":       map[string]any{"type": "string"},
			"description": "Realm paths the session may sign for (e.g. [\"gno.land/r/myorg/blog\"]). Required, non-empty.",
			"minItems":    1,
		},
		"spend_limit": map[string]any{
			"type":        "string",
			"description": "Maximum spend for this session (e.g. \"1000000ugnot\"). Optional; profile default used if omitted; clamped to chain-type hard limit.",
		},
		"expires_in": map[string]any{
			"type":        "string",
			"description": "Session lifetime as a Go duration string (e.g. \"24h\"). Optional; profile default used if omitted; clamped to chain-type hard limit.",
		},
	}
	required := []string{"allow_paths"}
	addProfileArg(s, props, &required)
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	}
}
