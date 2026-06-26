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
)

// claHashRe extracts the Required Hash value from the CLA realm's rendered
// markdown table row: | **Required Hash** | `<hex>` |
var claHashRe = regexp.MustCompile(`\*\*Required Hash\*\*\s*\|\s*` + "`" + `([0-9a-fA-F]+)` + "`")

// claURLRe extracts the document URL from the CLA realm's rendered markdown
// link: [<display>](<url>) — the first markdown link whose text contains "http".
var claURLRe = regexp.MustCompile(`\[.*?\]\((https?://[^)]+)\)`)

// RegisterCLASign registers the gno_cla_sign tool.
// ks provides the agent signer; resolver returns the chain client; alog writes
// audit entries on every sign attempt.
func RegisterCLASign(s *server.Server, ks *keystore.Keystore, resolver chain.Resolver, alog *audit.Log) {
	s.Registry().Add(&server.Tool{
		Name: "gno_cla_sign",
		Description: `Sign the Gno CLA (Contributor License Agreement) on-chain.

IMPORTANT: You MUST present the CLA URL to the user and ask for their explicit confirmation before calling this tool with confirmed=true. Never auto-sign on the user's behalf.

Two-step flow:
1. Call without confirmed to fetch the current CLA hash and URL.
2. Show the URL to the user, ask them to confirm. Only after confirmation, call again with confirmed=true and the hash.`,
		InputSchema: claSignInputSchema(s),
		OutputKind:  server.OutputText,
		Capability:  server.CapWrite,
		SelfAudited: true,
		Annotations: server.Annotations{
			ReadOnly:    false,
			Destructive: false,
			Idempotent:  true,
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
			"description": "The CLA hash to sign. Required when confirmed=true. Obtain it from step 1 (call without confirmed).",
		},
		"confirmed": map[string]any{
			"type":        "boolean",
			"description": "Must be true to actually broadcast the Sign transaction. Only set after the user has seen and confirmed the CLA URL.",
			"default":     false,
		},
	}
	required := []string{}
	addWritableProfileArg(s, props, &required)
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

	var (
		profileName string
		auditResult = "tool_err"
	)
	defer func() {
		alog.Record(audit.Entry{
			Tool:        "gno_cla_sign",
			Profile:     profileName,
			ArgsSummary: "",
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
		auditResult = "ok"
		return claFetchInfo(ctx, c)
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
		return server.Result{}, withCLAHint(fmt.Errorf("gno_cla_sign broadcast: %w", callErr))
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
	fmt.Fprintf(&sb, "Signed by: agent (%s)\n\n", signerAddr)
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
func claFetchInfo(ctx context.Context, c chain.Client) (server.Result, error) {
	rendered, err := c.Render(ctx, claRealm, "")
	if err != nil {
		return server.Result{}, fmt.Errorf("gno_cla_sign: render %s: %w", claRealm, err)
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
		"hash":            hash,
		"cla_url":         claURL,
		"action_required": "Show this URL to the user and ask for confirmation before calling with confirmed=true",
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "CLA information fetched from %s\n\n", claRealm)
	fmt.Fprintf(&sb, "Required Hash: %s\n", hash)
	if claURL != "" {
		fmt.Fprintf(&sb, "CLA URL:       %s\n\n", claURL)
	}
	fmt.Fprintf(&sb, "ACTION REQUIRED: Show the CLA URL to the user and ask for their explicit confirmation before calling gno_cla_sign with confirmed=true and the hash above.\n")

	return server.Result{
		Text:              sb.String(),
		StructuredContent: structured,
	}, nil
}
