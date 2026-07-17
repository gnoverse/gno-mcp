package write

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gnolang/gno/gno.land/pkg/sdk/vm"

	"github.com/gnoverse/gno-mcp/internal/server"
)

// claRealm is the canonical CLA realm path the agent signs to clear the gate.
const claRealm = "gno.land/r/sys/cla"

// claUnsignedLog is the substring the VM keeper puts in the deliver-tx log when
// it rejects a deploy because the signer has not signed the chain's Contributor
// License Agreement (gno.land/pkg/sdk/vm/keeper.go). It is specific to the CLA
// gate: the namespace gate rejects with the same error type but a different log
// ("is not authorized to deploy packages to namespace"), so this phrase tells
// the two apart.
const claUnsignedLog = "has not signed the required CLA"

// claHint is the actionable guidance appended to a CLA rejection. It routes
// recovery through the gno_cla_info / gno_cla_sign pair — gno_cla_info reports
// the live required hash and document URL from the realm, so nothing here can
// go stale — and carries the show-the-user step that is the reason the
// dedicated tools exist.
const claHint = "This chain enforces a Contributor License Agreement (gno.land/r/sys/cla) that the signing key has not accepted.\n" +
	"Clear it, then retry the deploy:\n" +
	"  1. Call gno_cla_info to fetch the required hash and the CLA document URL.\n" +
	"  2. Show the URL to the user and get their explicit confirmation.\n" +
	"  3. Call gno_cla_sign with that hash, using the same key that deploys."

// withCLAHint converts the keeper's CLA-gate rejection into a typed
// cla_unsigned ToolError carrying actionable guidance, and returns err unchanged
// otherwise. It mirrors gnokey's isCLAError: gate on the typed error first, then
// disambiguate the CLA case from the namespace case by the deliver-tx log. The
// typed code matches the recovery-error convention used across the write tools,
// so a client gets a machine-readable signal alongside the prose hint.
func withCLAHint(err error) error {
	if err == nil || !errors.Is(err, vm.UnauthorizedUserError{}) {
		return err
	}
	if !mentionsCLA(err) {
		return err
	}
	return &server.ToolError{
		Code:    "cla_unsigned",
		Message: err.Error() + "\n\n" + claHint,
		Extra:   map[string]any{"sign_realm": claRealm, "sign_tool": "gno_cla_sign"},
	}
}

// mentionsCLA reports whether the CLA-gate log appears anywhere in err's chain.
// The tm2 errors package keeps the deliver-tx log in a wrap annotation that
// surfaces only under the %+v verb on that node; the stdlib fmt.Errorf wraps
// layered on top (chain client, then handler) do not propagate it, so each
// unwrapped node must be formatted individually.
func mentionsCLA(err error) bool {
	for e := err; e != nil; e = errors.Unwrap(e) {
		if strings.Contains(fmt.Sprintf("%+v", e), claUnsignedLog) {
			return true
		}
	}
	return false
}
