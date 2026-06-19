package write

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gnolang/gno/gno.land/pkg/sdk/vm"
)

// claUnsignedLog is the substring the VM keeper puts in the deliver-tx log when
// it rejects a deploy because the signer has not signed the chain's Contributor
// License Agreement (gno.land/pkg/sdk/vm/keeper.go). It is specific to the CLA
// gate: the namespace gate rejects with the same error type but a different log
// ("is not authorized to deploy packages to namespace"), so this phrase tells
// the two apart.
const claUnsignedLog = "has not signed the required CLA"

// claHint is the actionable guidance appended to a CLA rejection. It names the
// realm and the two operations (read the hash, then Sign) without reciting the
// hash itself — the required hash is per-chain state that the realm's render
// reports live, and hardcoding it here would go stale.
const claHint = "This chain enforces a Contributor License Agreement (CLA) that the signing key has not accepted.\n" +
	"Sign it once from the same key, then retry the deploy:\n" +
	"  1. Read the required hash: render gno.land/r/sys/cla (its output lists \"Required Hash\").\n" +
	"  2. Call gno.land/r/sys/cla function Sign with that hash, signed by the same key."

// withCLAHint appends claHint when err is the keeper's CLA-gate rejection, and
// returns err unchanged otherwise. It mirrors gnokey's isCLAError: gate on the
// typed error first, then disambiguate the CLA case from the namespace case by
// the deliver-tx log. The original error stays wrapped.
func withCLAHint(err error) error {
	if err == nil || !errors.Is(err, vm.UnauthorizedUserError{}) {
		return err
	}
	if !mentionsCLA(err) {
		return err
	}
	return fmt.Errorf("%w\n\n%s", err, claHint)
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
