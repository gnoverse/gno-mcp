package write

import (
	"context"
	"fmt"
	"strings"

	"github.com/gnolang/gno/tm2/pkg/crypto"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/server"
	"github.com/gnoverse/gno-mcp/internal/session"
)

// RegisterSessionPropose registers the gno_session_propose tool.
// sessionMgr holds pending session state; it is shared with the write
// tools so they can pick up activated sessions on the next call. resolver
// supplies the profile's chain client: propose queries the live per-write
// gas fee to size the default spend limit and refuse limits no write could
// ever fit under.
func RegisterSessionPropose(s *server.Server, sessionMgr *session.Manager, resolver chain.Resolver) {
	s.Registry().Add(&server.Tool{
		Name: "gno_session_propose",
		Description: "Proposes a new chain-bounded session for the given profile by generating " +
			"an ephemeral session keypair locally and emitting the gnokey command the user must " +
			"run to authorize it. Use when an agent needs to perform a write but no active session " +
			"covers the target realm or MsgRun. Scope covers gno_call (allow_paths) and gno_run " +
			"(allow_run) ONLY — gno_addpkg/deploy is not session-authorizable and always signs with " +
			"the agent key, so deploying under the user's own address means the user runs gnokey. " +
			"Returns the proposed scope, the bech32 session " +
			"address, a copy-paste-ready gnokey command, and the per-write cost math (the chain " +
			"counts the full gas fee of every write against the spend limit). Does NOT broadcast " +
			"anything — the user's gnokey signs the MsgCreateSession from their own machine. " +
			"Required: profile, plus at least one of allow_paths (non-empty array of realm paths) " +
			"or allow_run=true. Optional: spend_limit (string like \"50000000ugnot\"; must cover " +
			"at least one write's gas fee at the chain's live gas price, else the proposal is " +
			"rejected with the minimum to use), expires_in (Go duration string like \"24h\").",
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
			return sessionProposeHandler(ctx, args, s, sessionMgr, resolver)
		},
	})
}

func sessionProposeHandler(
	ctx context.Context,
	args map[string]any,
	s *server.Server,
	sessionMgr *session.Manager,
	resolver chain.Resolver,
) (server.Result, error) {
	profileName, profile, err := requireProfile(args, s)
	if err != nil {
		return server.Result{}, err
	}

	allowPaths, err := server.StringSliceArg(args, "allow_paths")
	if err != nil {
		return server.Result{}, err
	}
	allowRun, err := server.BoolArg(args, "allow_run")
	if err != nil {
		return server.Result{}, err
	}

	spendLimit, err := server.StringArg(args, "spend_limit")
	if err != nil {
		return server.Result{}, err
	}
	expiresIn, err := server.StringArg(args, "expires_in")
	if err != nil {
		return server.Result{}, err
	}

	userMaster, err := server.StringArg(args, "master_address")
	if err != nil {
		return server.Result{}, err
	}

	// The session binds to a master account (the chain sets MsgCall.Caller to it).
	// It comes from the profile's master-address, or — for a writable profile that
	// has none — from a PUBLIC address the user supplies here (no file edit, no
	// restart). The address is stored on the session record, so it persists with
	// the session regardless of the profile.
	master := profile.MasterAddress
	if userMaster != "" {
		if verr := validateUserMasterAddress(userMaster); verr != nil {
			return server.Result{}, &server.ToolError{
				Code:    "invalid_master_address",
				Message: verr.Error(),
				Extra:   map[string]any{"profile": profileName},
			}
		}
		master = userMaster
	}
	if master == "" {
		return server.Result{}, &server.ToolError{
			Code: "no_master_address",
			Message: fmt.Sprintf(
				"profile %q has no master-address — pass master_address (your PUBLIC g1... address, NOT a private key or seed phrase) so the session can act as you; or set master-address in profiles.toml",
				profileName,
			),
			Extra: map[string]any{"profile": profileName},
		}
	}

	// The live per-write fee shapes the whole proposal: the chain's ante
	// counts the full offered GasFee against the session spend limit, so a
	// limit below it can never broadcast. Fail closed when the fee is
	// unknown — proposing blind could mint a dead-on-arrival session.
	c := resolver(profileName)
	if c == nil {
		return server.Result{}, fmt.Errorf("gno_session_propose: no chain client for profile %q", profileName)
	}
	feeUgnot, err := c.GasFeeUgnot(ctx)
	if err != nil {
		return server.Result{}, fmt.Errorf("gno_session_propose: query live gas fee: %w", err)
	}

	scopeArgs := session.ScopeArgs{
		AllowPaths: allowPaths,
		AllowRun:   allowRun,
		SpendLimit: spendLimit,
		ExpiresIn:  expiresIn,
	}
	scope, warnings, err := session.ResolveScope(scopeArgs, &profile, feeUgnot)
	if err != nil {
		return server.Result{}, fmt.Errorf("gno_session_propose: resolve scope: %w", err)
	}

	kp, err := session.NewKeypair()
	if err != nil {
		return server.Result{}, fmt.Errorf("gno_session_propose: generate keypair: %w", err)
	}

	if _, err := sessionMgr.AddPending(profileName, kp, scope, master); err != nil {
		return server.Result{}, fmt.Errorf("gno_session_propose: persist pending session: %w", err)
	}

	cmd := session.FormatGnokeyCreateCommand(&profile, kp.PubkeyBech32(), scope, feeUgnot)

	var b strings.Builder
	fmt.Fprintf(&b, "Session proposed for profile %q.\n\n", profileName)
	fmt.Fprintf(&b, "Proposed scope:\n")
	fmt.Fprintf(&b, "  - allow_paths: %s\n", strings.Join(scope.AllowPaths, ", "))
	fmt.Fprintf(&b, "  - allow_run: %t\n", scope.AllowRun)
	if scope.SpendLimit != "" {
		fmt.Fprintf(&b, "  - spend_limit: %s\n", scope.SpendLimit)
	}
	if scope.ExpiresIn > 0 {
		fmt.Fprintf(&b, "  - expires_in: %s\n", scope.ExpiresIn)
	}
	fmt.Fprintf(&b, "  - session_address: %s\n", kp.Address())
	if writes, ok := scope.WritesAtFee(feeUgnot); ok {
		fmt.Fprintf(&b,
			"\nSpend math: each write consumes its full offered gas fee — currently %dugnot per light write at this chain's live gas price — from the spend limit, so %s covers ~%d light write(s). Writes heavy enough to size above the floor gas limit cost proportionally more; every write is re-checked against the remaining limit before broadcast.\n",
			feeUgnot, scope.SpendLimit, writes,
		)
	}
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

	sc := map[string]any{
		"state":           "pending",
		"profile":         profileName,
		"session_address": kp.Address(),
		"session_pubkey":  kp.PubkeyBech32(),
		"scope": map[string]any{
			"allow_paths": scope.AllowPaths,
			"allow_run":   scope.AllowRun,
			"spend_limit": scope.SpendLimit,
			"expires_in":  scope.ExpiresIn.String(),
		},
		"auth_command":   cmd,
		"clamp_warnings": warnings,
		// The spend math must live here as well as in the text: clients that
		// surface structuredContent never see the Text rendering.
		"per_write_fee_ugnot": feeUgnot,
	}
	if writes, ok := scope.WritesAtFee(feeUgnot); ok {
		sc["writes_budget"] = writes
	}

	return server.Result{
		Text:              b.String(),
		StructuredContent: sc,
	}, nil
}

// validateUserMasterAddress guards a user-supplied master_address: it must be a
// PUBLIC bech32 address, never key material. A seed phrase is whitespace-
// separated words, so anything containing whitespace is rejected WITHOUT echoing
// it back (a mnemonic must never re-enter the conversation or logs). The bech32
// case likewise does not echo the value, which could be a malformed key.
func validateUserMasterAddress(s string) error {
	if strings.ContainsAny(s, " \t\r\n") {
		return fmt.Errorf("that looks like a seed phrase, not an address — paste ONLY your PUBLIC g1... address (one token, no spaces); never a private key or seed phrase")
	}
	if _, err := crypto.AddressFromBech32(s); err != nil {
		return fmt.Errorf("invalid master_address: want your PUBLIC bech32 address (starts with g1), not a private key or seed phrase")
	}
	return nil
}

func sessionProposeInputSchema(s *server.Server) map[string]any {
	props := map[string]any{
		"allow_paths": map[string]any{
			"type":        "array",
			"items":       map[string]any{"type": "string"},
			"description": "Realm paths the session may sign vm/MsgCall for (e.g. [\"gno.land/r/myorg/blog\"]). Optional if allow_run=true; otherwise required and non-empty. When present, must contain at least one entry.",
			"minItems":    1,
		},
		"allow_run": map[string]any{
			"type":        "boolean",
			"description": "When true, the session can broadcast MsgRun (ad-hoc gno scripts) in addition to any realm calls in allow_paths. Optional; default false. At least one of allow_paths (non-empty) or allow_run=true must be requested.",
			"default":     false,
		},
		"spend_limit": map[string]any{
			"type":        "string",
			"description": "Maximum spend for this session (e.g. \"50000000ugnot\"). Every write consumes its full gas fee from this limit, so it must cover at least one write at the chain's live gas price — a lower value is rejected with the minimum to use. Optional; when omitted, sized to ~10 writes at the live fee (or the profile default); clamped to the per-chain hard limit.",
		},
		"expires_in": map[string]any{
			"type":        "string",
			"description": "Session lifetime as a Go duration string (e.g. \"24h\"). Optional; profile default used if omitted; clamped to the per-chain hard limit.",
		},
		"master_address": map[string]any{
			"type": "string",
			"description": "The user's PUBLIC account address — bech32, starts with g1 (e.g. \"g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5\"). " +
				"Required when the target (writable testnet/local) profile has no master-address; the session binds to this account. " +
				"It is PUBLIC and safe to share — NEVER a private key or seed phrase. Ask the user for it with that distinction made explicit; do not edit profiles.toml on their behalf.",
		},
	}
	var required []string
	addProfileArg(s, props, &required)
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	}
}
