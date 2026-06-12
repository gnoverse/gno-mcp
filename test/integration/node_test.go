//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIntegration_counterRoundTrip(t *testing.T) {
	c := newNodeBackedReal(t)
	ctx := context.Background()

	_, err := c.Render(ctx, "gno.land/r/test/counter", "")
	require.NoError(t, err, "render")

	_, err = c.Call(ctx, test1Signer(t), "gno.land/r/test/counter", "Increment", nil, "", false)
	require.NoError(t, err, "call increment")

	out, err := c.Eval(ctx, "gno.land/r/test/counter", "Total()")
	require.NoError(t, err, "eval count")
	require.Contains(t, out, "1")
}
