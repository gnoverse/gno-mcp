package write

import (
	"errors"
	"fmt"
	"testing"

	"github.com/gnolang/gno/gno.land/pkg/sdk/vm"
	tmerrors "github.com/gnolang/gno/tm2/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/server"
)

// The VM keeper gates addpkg behind two checks that share one error type
// (UnauthorizedUserError) but carry different deliver-tx logs: the namespace
// gate and the CLA gate. withCLAHint must append the sign-the-CLA guidance for
// the CLA case only — misfiring on the namespace error would send users to sign
// a CLA when the real fix is a namespace registration.
//
// The log lives in a tm2-errors wrap annotation (gnoclient adds it), and reaches
// the handler under two more fmt.Errorf wraps (chain client, then handler), so
// the constructors below reproduce that exact shape.
func TestWithCLAHint(t *testing.T) {
	// deliverErr reproduces the keeper rejection as it crosses the wire: a typed
	// UnauthorizedUserError annotated by gnoclient with the deliver-tx log.
	deployFailure := func(log string) error {
		gnoclientErr := tmerrors.Wrapf(vm.UnauthorizedUserError{}, "deliver transaction failed: log:%s", log)
		chainErr := fmt.Errorf("addpackage: broadcast: %w", gnoclientErr) // internal/chain
		return fmt.Errorf("gno_addpkg broadcast: %w", chainErr)           // handler prefix
	}

	claErr := deployFailure("address g1abc has not signed the required CLA")
	nsErr := deployFailure("g1abc is not authorized to deploy packages to namespace `myns`")

	t.Run("returns a typed cla_unsigned ToolError on a CLA rejection", func(t *testing.T) {
		got := withCLAHint(claErr)
		require.Error(t, got)
		var te *server.ToolError
		require.ErrorAs(t, got, &te, "CLA rejections must surface as a typed ToolError, matching the codebase's recovery-error convention")
		assert.Equal(t, "cla_unsigned", te.Code)
		assert.Contains(t, te.Message, "Contributor License Agreement")
		assert.Contains(t, te.Message, "gno.land/r/sys/cla")
		assert.Contains(t, te.Message, "Sign")
		assert.Equal(t, "gno.land/r/sys/cla", te.Extra["sign_realm"])
	})

	t.Run("leaves the namespace rejection untouched", func(t *testing.T) {
		got := withCLAHint(nsErr)
		var te *server.ToolError
		assert.False(t, errors.As(got, &te), "namespace errors must not become a cla_unsigned ToolError")
		assert.Equal(t, nsErr.Error(), got.Error(), "namespace errors must pass through unchanged")
	})

	t.Run("does not fire on the bare phrase without the typed error", func(t *testing.T) {
		// Same phrase, but not an UnauthorizedUserError — the type gate must
		// reject it (the false-positive gnokey's cla_test.go guards against).
		base := errors.New("some unrelated failure mentioning: has not signed the required CLA")
		assert.Equal(t, base, withCLAHint(base))
	})

	t.Run("passes through an UnauthorizedUserError without the CLA phrase", func(t *testing.T) {
		base := deployFailure("insufficient funds")
		assert.Equal(t, base.Error(), withCLAHint(base).Error())
	})

	t.Run("passes through nil", func(t *testing.T) {
		assert.NoError(t, withCLAHint(nil))
	})
}
