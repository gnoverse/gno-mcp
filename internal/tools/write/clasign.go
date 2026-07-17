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

// claProfileArgDesc scopes both CLA tools' profile enums to chains the agent
// can sign on — the gate applies to deploys, which sign with the agent key.
const claProfileArgDesc = "Profile (chain) to target. Writable chains only — the CLA gate applies to deploys, which sign with the agent key."

// RegisterCLAInfo registers the gno_cla_info tool, the read-side companion of
// gno_cla_sign. resolver returns the chain client per profile.
func RegisterCLAInfo(s *server.Server, resolver chain.Resolver) {
	s.Registry().Add(&server.Tool{
		Name: "gno_cla_info",
		Description: "Reports the chain's Contributor License Agreement state from gno.land/r/sys/cla: " +
			"whether enforcement is enabled, the current required hash, and the agreement document URL — " +
			"the URL is meant for the user to review before the agreement is signed on their behalf. " +
			"Use when a deploy is blocked by the CLA gate (gno_addpkg error code cla_unsigned), or to pre-check the gate before deploying. " +
			"Returns {enabled, hash, cla_url} as structured content; a disabled gate returns enabled=false and no hash (nothing to sign). " +
			"Read-only: does NOT sign anything — signing is gno_cla_sign, which takes the hash reported here.",
		InputSchema: claInfoInputSchema(s),
		OutputKind:  server.OutputText,
		Capability:  server.CapWritePrep,
		Annotations: server.Annotations{
			ReadOnly:    true,
			Destructive: false,
			Idempotent:  true,
			OpenWorld:   true,
		},
		Handler: func(ctx context.Context, args map[string]any) (server.Result, error) {
			return claInfoHandler(ctx, args, s, resolver)
		},
	})
}

// RegisterCLASign registers the gno_cla_sign tool.
// ks provides the agent signer; resolver returns the chain client; alog writes
// audit entries on every sign attempt.
func RegisterCLASign(s *server.Server, ks *keystore.Keystore, resolver chain.Resolver, alog *audit.Log) {
	s.Registry().Add(&server.Tool{
		Name: "gno_cla_sign",
		Description: "Signs the chain's Contributor License Agreement on-chain — broadcasts gno.land/r/sys/cla Sign(hash) " +
			"with the agent key and returns the tx hash, height, gas, and signing identity. " +
			"The hash comes from gno_cla_sign's read-side companion gno_cla_info, which also reports the agreement URL " +
			"intended for the user to review and confirm before this tool is called. " +
			"Use when a deploy is blocked by the CLA gate (gno_addpkg error code cla_unsigned) and the user has confirmed the agreement — " +
			"use this tool, not a raw gno_call on r/sys/cla. " +
			"Does NOT fetch the hash or the agreement URL (that is gno_cla_info); does NOT check whether the key already signed; " +
			"does NOT sign as the user/session — the agent key signs.",
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

func claInfoInputSchema(s *server.Server) map[string]any {
	props := map[string]any{}
	required := []string{}
	addProfileArgFiltered(s, props, &required, profileWritableByAgent, claProfileArgDesc)
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	}
}

func claSignInputSchema(s *server.Server) map[string]any {
	props := map[string]any{
		"hash": map[string]any{
			"type":        "string",
			"pattern":     "^[0-9a-fA-F]+$",
			"description": "The CLA hash to sign, hex as reported by gno_cla_info. e.g. \"a1b2c3d4…\".",
		},
	}
	required := []string{"hash"}
	addProfileArgFiltered(s, props, &required, profileWritableByAgent, claProfileArgDesc)
	addOptionalKeyArg(props)
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	}
}

// claInfoHandler renders gno.land/r/sys/cla and extracts the required hash and
// document URL. A render reporting enforcement disabled is a valid "nothing to
// sign" answer, not a parse failure. The hash is regex-constrained hex; the URL
// is realm-authored text, so its inline rendering goes through the untrusted
// envelope (the structured field stays raw as the machine channel,
// docs/security.md §4).
func claInfoHandler(ctx context.Context, args map[string]any, s *server.Server, resolver chain.Resolver) (server.Result, error) {
	profileName, _, err := requireProfile(args, s)
	if err != nil {
		return server.Result{}, err
	}
	c := resolver(profileName)
	if c == nil {
		return server.Result{}, fmt.Errorf("profile %q: no chain client available", profileName)
	}

	rendered, err := c.Render(ctx, claRealm, "")
	if err != nil {
		return server.Result{}, fmt.Errorf("gno_cla_info: render %s: %w", claRealm, err)
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
		"action_required": "Show this URL to the user and ask for confirmation before signing with gno_cla_sign",
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "CLA information fetched from %s\n\n", claRealm)
	fmt.Fprintf(&sb, "Required Hash: %s\n", hash)
	if claURL != "" {
		fmt.Fprintf(&sb, "CLA document URL (realm-reported):\n%s\n\n", untrusted.Wrap(claURL, "cla_url", claRealm))
	}
	fmt.Fprintf(&sb, "ACTION REQUIRED: Show the CLA URL to the user and ask for their explicit confirmation before signing with gno_cla_sign and the hash above.\n")

	return server.Result{
		Text:              sb.String(),
		StructuredContent: structured,
	}, nil
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
	// skip its generic line. auditResult defaults to a denial; the success path
	// overwrites it. The hash is public chain state, safe in the summary.
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

	hash, err := server.StringArg(args, "hash")
	if err != nil {
		return server.Result{}, err
	}
	if hash == "" {
		return server.Result{}, &server.ToolError{
			Code:    "hash_required",
			Message: "hash is required — call gno_cla_info first to obtain the current CLA hash and the agreement URL, show the URL to the user, then sign with that hash",
		}
	}
	argsSummary = "hash=" + hash

	keyName, err := keyArg(args)
	if err != nil {
		return server.Result{}, err
	}

	c := resolver(profileName)
	if c == nil {
		return server.Result{}, fmt.Errorf("profile %q: no chain client available", profileName)
	}

	signer, signerAddr, err := acquireAgentSigner(ctx, ks, c, "gno_cla_sign",
		"run gno_key_generate first, then gno_faucet_fund", profileName, keyName, profile, false)
	if err != nil {
		return server.Result{}, err
	}

	cr, callErr := c.Call(ctx, signer, claRealm, "Sign", []string{hash}, "", false)
	if callErr != nil {
		auditResult = "broadcast_err"
		return server.Result{}, fmt.Errorf("gno_cla_sign broadcast: %w", callErr)
	}

	auditResult = "ok"

	structured := map[string]any{
		"identity":       "agent",
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
