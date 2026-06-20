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
	"github.com/gnoverse/gno-mcp/internal/session"
)

// writeTxDispatch parameterizes the agent-or-session dispatch pipeline shared
// by the write-tx tools. The op closures capture the tool's typed chain result
// in the handler's scope; the dispatcher owns identity resolution, signer
// acquisition, error mapping, audit-result transitions, and session spend
// tracking.
type writeTxDispatch struct {
	tool        string // tool name, used in error prefixes (e.g. "gno_call")
	noKeyHint   string // tailors acquireAgentSigner's missing-key message
	profileName string
	keyName     string // agent key to sign with ("" ⇒ default)
	profile     profiles.Profile
	simulate    bool
	c           chain.Client
	ks          *keystore.Keystore
	sessionMgr  *session.Manager

	// pickSession selects an active session authorizing the tool's operation,
	// returning the session's signer and the master address it was issued under
	// (the master lives on the session record, not necessarily the profile).
	pickSession func(ctx context.Context) (chain.Signer, string, error)
	// mapPickErr converts a pickSession failure to the tool's structured
	// ToolError; returning nil falls back to the generic wrap.
	mapPickErr func(error) error
	// agentOp / sessionOp perform the chain operation with the acquired signer.
	// sessionOp receives the session's master so it signs as the right account.
	agentOp   func(ctx context.Context, signer gnoclient.Signer) error
	sessionOp func(ctx context.Context, signer chain.Signer, master string) error

	// Audit fields owned by the handler's deferred audit record; the dispatcher
	// mutates them so denials and outcomes are recorded on every return path.
	auditResult *string
	sessionAddr *string
}

// dispatchWriteTx resolves the acting identity (explicit arg, or by tier:
// local/testnet→agent, otherwise→session) and runs the corresponding branch.
// It returns the resolved identity, the signer address, and the master address
// (session identity only).
func dispatchWriteTx(ctx context.Context, identityArg string, d writeTxDispatch) (identity, signerAddr, master string, err error) {
	identity = identityArg
	if identity == "" {
		// Every allowed chain (local dev or testnet) has an agent key path, so
		// the agent is the default acting identity; sessions are opt-in via
		// identity=session.
		identity = "agent"
	}

	switch identity {
	case "agent":
		// ---- Agent branch: sign with the agent's own key (local test1 or testnet generated key)

		agentSigner, addr, aerr := acquireAgentSigner(ctx, d.ks, d.c, d.tool, d.noKeyHint, d.profileName, d.keyName, d.profile, d.simulate)
		if aerr != nil {
			return identity, "", "", aerr
		}
		signerAddr = addr

		if opErr := d.agentOp(ctx, agentSigner); opErr != nil {
			return identity, signerAddr, "", d.txError(opErr)
		}
		// No UpdateSpend — agent pays from its own balance.

	case "session":
		// ---- Session branch

		// The session path signs with the session keypair, not an agent key, so a
		// supplied `key` would be silently ignored. Fail loudly instead so the
		// caller never believes a tx went out under the named key.
		if d.keyName != "" {
			return identity, "", "", &server.ToolError{
				Code:    "key_ignored_for_session",
				Message: "the key arg selects an agent key and does not apply to identity=session (the session signer is used)",
				Extra:   map[string]any{"profile": d.profileName, "key": d.keyName},
			}
		}

		signer, sessMaster, pickErr := d.pickSession(ctx)
		if pickErr != nil {
			if terr := d.mapPickErr(pickErr); terr != nil {
				return identity, "", "", terr
			}
			return identity, "", "", fmt.Errorf("%s: pick session: %w", d.tool, pickErr)
		}

		*d.sessionAddr = signer.Address()
		signerAddr = *d.sessionAddr
		// The master is the account the session was issued under (stored on the
		// session record), which may differ from profile.MasterAddress — e.g. a
		// master-less profile whose user supplied an address at propose time.
		master = sessMaster

		if opErr := d.sessionOp(ctx, signer, master); opErr != nil {
			if d.simulate && errors.Is(opErr, chain.ErrSimulateUnsupported) {
				return identity, signerAddr, master, &server.ToolError{
					Code:    "simulate_unsupported",
					Message: "this chain client does not support simulate; retry without simulate=true",
					Extra:   map[string]any{"profile": d.profileName},
				}
			}
			return identity, signerAddr, master, d.txError(opErr)
		}

		// Update spend (simulate skips it). The chain bills the session the full
		// GasFee per tx, not GasUsed, so deduct that to keep local SpendRemaining
		// in sync with the chain (see chain.DefaultGasFeeUgnot).
		if !d.simulate {
			_ = d.sessionMgr.UpdateSpend(d.profileName, *d.sessionAddr, chain.DefaultGasFeeUgnot)
		}

	default:
		return identity, "", "", fmt.Errorf("identity: must be \"agent\" or \"session\", got %q", identity)
	}

	*d.auditResult = "ok"
	if d.simulate {
		*d.auditResult = "sim"
	}
	return identity, signerAddr, master, nil
}

// txError wraps a chain-op failure with the tool/stage prefix and records the
// matching audit outcome.
func (d writeTxDispatch) txError(err error) error {
	errPrefix := d.tool + " broadcast"
	*d.auditResult = "broadcast_err"
	if d.simulate {
		errPrefix = d.tool + " simulate"
		*d.auditResult = "sim_err"
	}
	return fmt.Errorf("%s: %w", errPrefix, err)
}

// attachGnokeyCmd appends the illustrative `gnokey maketx` equivalent of the
// write to the result — a transparency aid showing the tx gnomcp put on the
// wire. It is intentionally self-contained (names only the gnokey CLI, no skill
// or gnomcp-internal detail) so it is meaningful to any client. No-op on "".
func attachGnokeyCmd(out server.Result, cmd string) server.Result {
	if cmd == "" {
		return out
	}
	out.Text += "\n\ngnokey equivalent (illustrative — gnomcp already signed and broadcast this; " +
		"the signing key lives in gnomcp's keystore, not gnokey's):\n  " + cmd
	if out.StructuredContent == nil {
		out.StructuredContent = map[string]any{}
	}
	out.StructuredContent["gnokey_command"] = cmd
	return out
}

// decorateWriteResult prefixes the signed-by line and attaches the identity
// metadata every write result carries.
func decorateWriteResult(out server.Result, identity, signerAddr, master string, local bool) server.Result {
	out.Text = signedByLine(identity, signerAddr, master, local) + "\n\n" + out.Text
	if out.StructuredContent == nil {
		out.StructuredContent = map[string]any{}
	}
	out.StructuredContent["identity"] = identity
	out.StructuredContent["signer_address"] = signerAddr
	if identity == "session" {
		out.StructuredContent["master_address"] = master
	}
	return out
}
