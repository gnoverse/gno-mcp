// Package parked explains a read or call that fails because the package at its
// path is parked: deployed to a chain running the inert code-submission policy
// and not yet enabled by a package approver. The chain answers a parked path
// exactly like one never deployed, so without this the agent reads "not found"
// for code it just deployed.
package parked

import (
	"context"
	"errors"
	"fmt"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/server"
)

// Code is the ToolError code for an operation on a parked package.
const Code = "package_parked"

// NextSteps is the recovery for a package that stays parked.
const NextSteps = "If it is still parked a minute after its deploy, the chain's package approver is not going to enable it: " +
	"lint it against the chain's release (a package that fails the type check is never enabled), " +
	"check the deploying key can pay the storage deposit, " +
	"then redeploy it to the same path with the same key."

// Explain returns a package_parked ToolError when err came from an operation on
// path and the chain holds a parked submission there, and err unchanged in every
// other case — including when the chain cannot say. A typed ToolError passes
// through without a query: it already names what went wrong.
func Explain(ctx context.Context, c chain.Client, path string, err error) error {
	if err == nil {
		return nil
	}
	if _, typed := errors.AsType[*server.ToolError](err); typed {
		return err
	}
	meta, metaErr := c.PackageMeta(ctx, path)
	if metaErr != nil || meta.Status != chain.PackageInert {
		return err
	}
	return &server.ToolError{
		Code: Code,
		Message: fmt.Sprintf("%s is parked, not live: the chain accepted its deploy and has not enabled it ([untrusted chain reason] %s). "+
			"Until it is enabled, every read and call answers it like a package that was never deployed.\n\n"+
			"For its deployer: %s\n\nOriginal error: %v",
			path, meta.Reason, NextSteps, err),
		Extra: map[string]any{"path": path, "reason": meta.Reason},
	}
}
