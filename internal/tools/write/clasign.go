package write

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gnoverse/gno-mcp/internal/audit"
	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/keystore"
	"github.com/gnoverse/gno-mcp/internal/server"
	"github.com/gnoverse/gno-mcp/internal/untrusted"
)

// claHashRe extracts the Required Hash value from the CLA realm's rendered
// markdown table row: | **Required Hash** | `<hex>` |
var claHashRe = regexp.MustCompile(`\*\*Required Hash\*\*\s*\|\s*` + "`" + `([0-9a-fA-F]+)` + "`")

// claURLRe extracts the document URL from the CLA realm's rendered markdown:
// the first markdown link whose target is an absolute http(s) URL. The realm's
// only other link (the Sign helplink) uses a relative $help target, so it never
// matches. The target must be paren- and whitespace-free so a hostile render
// cannot smuggle multi-line text through the match.
var claURLRe = regexp.MustCompile(`\[.*?\]\((https?://[^)\s]+)\)`)

// claDisabledMarker is the phrase the realm renders when requiredHash is empty
// (enforcement off — the default on fresh local chains).
const claDisabledMarker = "CLA enforcement is currently DISABLED"

// RegisterCLASign registers the gno_cla_sign tool.
// ks provides the agent signer; resolver returns the chain client; alog writes
// audit entries on every fetch and sign attempt.
func RegisterCLASign(s *server.Server, ks *keystore.Keystore, resolver chain.Resolver, alog *audit.Log) {
	s.Registry().Add(&server.Tool{
		Name: "gno_cla_sign",
		Description: `Signs the chain's Contributor License Agreement (gno.land/r/sys/cla) with the agent key, in two steps.

Step 1 — call without confirmed: returns the current required hash, the CLA document URL, and whether enforcement is enabled (a disabled chain needs no signature). Step 2 — call with confirmed=true and the hash from step 1: broadcasts Sign(hash) signed by the agent key and returns the tx hash.

IMPORTANT: between the two steps you MUST show the CLA URL to the user and get their explicit confirmation. Never pass confirmed=true without it — the agreement covers deployments made on the user's behalf.

Use when a deploy is blocked by the CLA gate (gno_addpkg error code cla_unsigned). Does NOT check whether the key already signed; does NOT sign as the user/session — the agent key signs.`,
		InputSchema: claSignInputSchema(s),
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
			return claSignHandler(ctx, args, s, ks, resolver, alog)
		},
	})
}

func claSignInputSchema(s *server.Server) map[string]any {
	props := map[string]any{
		"hash": map[string]any{
			"type":        "string",
			"pattern":     "^[0-9a-fA-F]+$",
			"description": "The CLA hash to sign, hex as reported by step 1 (call without confirmed). Required when confirmed=true. e.g. \"a1b2c3d4…\".",
		},
		"confirmed": map[string]any{
			"type":        "boolean",
			"description": "Must be true to actually broadcast the Sign transaction. Only set after the user has seen and confirmed the CLA URL returned by step 1.",
			"default":     false,
		},
	}
	required := []string{}
	addAgentProfileArg(s, props, &required)
	addOptionalKeyArg(props)
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	}
}

func claSignHandler(
	ctx context.Context,
	args map[string]any,
	s *server.Server,
	ks *keystore.Keystore,
	resolver chain.Resolver,
	alog *audit.Log,
) (server.Result, error) {
	start := time.Now()

	// One audit record per invocation, written on every return path — including
	// the early validation denials — because SelfAudited makes the MCP adapter
	// skip its generic line. auditResult defaults to a denial; the success paths
	// overwrite it. argsSummary distinguishes the fetch step from the sign step.
	var (
		profileName string
		argsSummary string
		auditResult = "tool_err"
	)
	defer func() {
		alog.Record(audit.Entry{
			Tool:        "gno_cla_sign",
			Profile:     profileName,
			ArgsSummary: argsSummary,
			Result:      auditResult,
			Duration:    time.Since(start).Milliseconds(),
		})
	}()

	profileName, profile, err := requireProfile(args, s)
	if err != nil {
		return server.Result{}, err
	}

	confirmed, err := server.BoolArg(args, "confirmed")
	if err != nil {
		return server.Result{}, err
	}
	argsSummary = fmt.Sprintf("confirmed=%v", confirmed)

	hash, err := server.StringArg(args, "hash")
	if err != nil {
		return server.Result{}, err
	}

	c := resolver(profileName)
	if c == nil {
		return server.Result{}, fmt.Errorf("profile %q: no chain client available", profileName)
	}

	// ---- Step 1: fetch CLA hash and URL from the realm render

	if !confirmed {
		res, ferr := claFetchInfo(ctx, c)
		if ferr != nil {
			return server.Result{}, ferr
		}
		auditResult = "ok"
		return res, nil
	}

	// ---- Step 2: broadcast Sign(hash)

	if hash == "" {
		return server.Result{}, &server.ToolError{
			Code:    "hash_required",
			Message: "hash is required when confirmed=true — call without confirmed first to obtain the current CLA hash, show the URL to the user, then call again with confirmed=true and the hash",
		}
	}

	keyName, err := keyArg(args)
	if err != nil {
		return server.Result{}, err
	}

	signer, signerAddr, err := acquireAgentSigner(ctx, ks, c, "gno_cla_sign",
		"run gno_key_generate first, then gno_faucet_fund", profileName, keyName, profile, false)
	if err != nil {
		return server.Result{}, err
	}

	cr, callErr := c.Call(ctx, signer, claRealm, "Sign", []string{hash}, "", false)
	if callErr != nil {
		return server.Result{}, fmt.Errorf("gno_cla_sign broadcast: %w", callErr)
	}

	auditResult = "ok"

	structured := map[string]any{
		"tx_hash":        cr.TxHash,
		"height":         cr.Height,
		"gas_used":       cr.GasUsed,
		"signer_address": signerAddr,
		"hash_signed":    hash,
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%s\n\n", signedByLine("agent", signerAddr, "", profile.IsLocal()))
	fmt.Fprintf(&sb, "CLA signed successfully\n\n")
	fmt.Fprintf(&sb, "TxHash:      %s\n", cr.TxHash)
	fmt.Fprintf(&sb, "Height:      %d\n", cr.Height)
	fmt.Fprintf(&sb, "GasUsed:     %d\n", cr.GasUsed)
	fmt.Fprintf(&sb, "Hash signed: %s\n", hash)

	return server.Result{
		Text:              sb.String(),
		StructuredContent: structured,
	}, nil
}

// claFetchInfo renders gno.land/r/sys/cla and extracts the required hash and
// document URL, returning them as a structured response with an action_required
// hint that instructs the agent to show the URL to the user before signing.
// A render reporting enforcement disabled is a valid "nothing to sign" answer,
// not a parse failure. The hash is regex-constrained hex; the URL is
// realm-authored text, so its inline rendering goes through the untrusted
// envelope (the structured field stays raw as the machine channel,
// docs/security.md §4).
func claFetchInfo(ctx context.Context, c chain.Client) (server.Result, error) {
	rendered, err := c.Render(ctx, claRealm, "")
	if err != nil {
		return server.Result{}, fmt.Errorf("gno_cla_sign: render %s: %w", claRealm, err)
	}

	if strings.Contains(rendered, claDisabledMarker) {
		return server.Result{
			Text:              fmt.Sprintf("CLA enforcement is disabled on %s — no signature is required to deploy.\n", claRealm),
			StructuredContent: map[string]any{"enabled": false},
		}, nil
	}

	hash := ""
	if m := claHashRe.FindStringSubmatch(rendered); len(m) == 2 {
		hash = m[1]
	}

	claURL := ""
	if m := claURLRe.FindStringSubmatch(rendered); len(m) == 2 {
		claURL = m[1]
	}

	if hash == "" {
		return server.Result{}, &server.ToolError{
			Code:    "cla_hash_not_found",
			Message: fmt.Sprintf("could not extract the required CLA hash from %s — the realm format may have changed", claRealm),
			Extra:   map[string]any{"realm": claRealm},
		}
	}

	structured := map[string]any{
		"enabled":         true,
		"hash":            hash,
		"cla_url":         claURL,
		"action_required": "Show this URL to the user and ask for confirmation before calling with confirmed=true",
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "CLA information fetched from %s\n\n", claRealm)
	fmt.Fprintf(&sb, "Required Hash: %s\n", hash)
	if claURL != "" {
		fmt.Fprintf(&sb, "CLA document URL (realm-reported):\n%s\n\n", untrusted.Wrap(claURL, "cla_url", claRealm))
	}
	fmt.Fprintf(&sb, "ACTION REQUIRED: Show the CLA URL to the user and ask for their explicit confirmation before calling gno_cla_sign with confirmed=true and the hash above.\n")

	return server.Result{
		Text:              sb.String(),
		StructuredContent: structured,
	}, nil
}
