package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Dispatch + usage errors are exercised without a chain (the gnoclient query
// wiring is verified live by the e2e harness running real scenarios). Usage
// errors must not even construct a client — they return 2 before any RPC.

func TestRun_noArgs_printsUsage(t *testing.T) {
	var out, errb bytes.Buffer
	code := run([]string{}, &out, &errb)
	require.Equal(t, 2, code)
	assert.Contains(t, errb.String(), "usage")
}

func TestRun_renderMissingPkgPath_usage(t *testing.T) {
	var out, errb bytes.Buffer
	code := run([]string{"render"}, &out, &errb)
	require.Equal(t, 2, code, "render with no pkgpath is a usage error")
	assert.Contains(t, errb.String(), "render")
}

func TestRun_evalMissingExpr_usage(t *testing.T) {
	var out, errb bytes.Buffer
	code := run([]string{"eval", "gno.land/r/test/counter"}, &out, &errb)
	require.Equal(t, 2, code, "eval needs both pkgpath and expression")
}

func TestRun_unknownSubcommand_usage(t *testing.T) {
	var out, errb bytes.Buffer
	code := run([]string{"frobnicate"}, &out, &errb)
	require.Equal(t, 2, code)
	assert.Empty(t, out.String(), "nothing on stdout for a bad subcommand")
}

func TestRun_balanceMalformedAddr_usage(t *testing.T) {
	var out, errb bytes.Buffer
	code := run([]string{"balance", "not-a-bech32"}, &out, &errb)
	require.Equal(t, 2, code, "a malformed address is a usage error, not an RPC error")
}
