package write

import (
	"context"
	"errors"
	"fmt"

	gnoclient "github.com/gnolang/gno/gno.land/pkg/gnoclient"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/keystore"
	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/gnoverse/gno-mcp/internal/server"
)

// signedByLine renders the acting-identity line for a write result. The agent
// identity is the well-known test1 account on local (dev) chains and a
// per-profile generated key on testnet, so the label is tier-dependent.
func signedByLine(identity, signerAddr, master string, local bool) string {
	if identity == "session" {
		return fmt.Sprintf("Signed by: session %s on behalf of master %s", signerAddr, master)
	}
	if local {
		return fmt.Sprintf("Signed by: agent test1 (%s)", signerAddr)
	}
	return fmt.Sprintf("Signed by: agent (%s)", signerAddr)
}

// agentKeyToolError maps the keystore sentinel errors shared by every
// agent-identity tool to structured ToolErrors; it returns nil when err has
// no shared mapping and the caller should wrap it itself. noKeyHint tailors
// the missing-key message to the calling tool.
func agentKeyToolError(err error, profileName, noKeyHint string) error {
	if errors.Is(err, keystore.ErrNoAgentKey) {
		return &server.ToolError{
			Code:    "agent_identity_unavailable",
			Message: fmt.Sprintf("no agent key for profile %q — %s", profileName, noKeyHint),
			Extra:   map[string]any{"profile": profileName},
		}
	}
	if errors.Is(err, keystore.ErrNoKeyDir) {
		return &server.ToolError{
			Code:    "key_storage_unconfigured",
			Message: "the agent keystore has no storage directory configured; set GNOMCP_AGENT_KEYS_PATH (or use the default) so agent keys can be persisted and loaded",
			Extra:   map[string]any{"profile": profileName},
		}
	}
	return nil
}

// acquireAgentSigner returns the profile's agent signer and its bech32 address,
// mapping keystore errors to structured ToolErrors and running the testnet
// unfunded-account pre-check (skipped when simulate). tool and noKeyHint tailor
// the error messages to the calling tool.
func acquireAgentSigner(ctx context.Context, ks *keystore.Keystore, c chain.Client, tool, noKeyHint, profileName, keyName string, profile profiles.Profile, simulate bool) (gnoclient.Signer, string, error) {
	signer, addr, err := agentSigner(ks, tool, noKeyHint, profileName, keyName, profile)
	if err != nil {
		return nil, "", err
	}
	if err := requireFunded(ctx, c, tool, profileName, addr, profile, simulate); err != nil {
		return nil, "", err
	}
	return signer, addr, nil
}

// agentSigner loads the profile's agent signer and its bech32 address, mapping
// keystore errors to structured ToolErrors.
func agentSigner(ks *keystore.Keystore, tool, noKeyHint, profileName, keyName string, profile profiles.Profile) (gnoclient.Signer, string, error) {
	signer, err := ks.SignerForProfile(profileName, keyName, profile)
	if err != nil {
		if terr := agentKeyToolError(err, profileName, noKeyHint); terr != nil {
			return nil, "", terr
		}
		return nil, "", fmt.Errorf("%s: signer: %w", tool, err)
	}
	info, err := signer.Info()
	if err != nil {
		return nil, "", fmt.Errorf("%s: signer info: %w", tool, err)
	}
	return signer, info.GetAddress().String(), nil
}

// requireFunded refuses a testnet write from an agent account holding nothing,
// before it can fail on chain; simulate skips it.
func requireFunded(ctx context.Context, c chain.Client, tool, profileName, addr string, profile profiles.Profile, simulate bool) error {
	if !profile.IsTestnet() || simulate {
		return nil
	}
	bal, err := c.Balance(ctx, addr)
	if err != nil {
		return fmt.Errorf("%s: balance check: %w", tool, err)
	}
	if bal == 0 {
		return &server.ToolError{
			Code:    "insufficient_funds",
			Message: fmt.Sprintf("agent testnet account %s is unfunded — run gno_faucet_fund (or send it ugnot), then retry", addr),
			Extra:   map[string]any{"profile": profileName, "address": addr},
		}
	}
	return nil
}
