package read

import (
	"context"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEval_returnsText(t *testing.T) {
	f := chain.NewFake()
	f.SetEval("gno.land/r/foo", "Bar()", "(42 int)")

	s := newBaseTestServer(t)
	RegisterEval(s, constResolver(f))
	res, err := s.Registry().Call(context.Background(), "gno_eval", map[string]any{
		"realm":   "gno.land/r/foo",
		"expr":    "Bar()",
		"profile": "testnet5",
	})
	require.NoError(t, err)
	assert.Equal(t, "(42 int)", res.Text)
}

func TestEval_requiresExpr(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterEval(s, constResolver(chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_eval", map[string]any{
		"realm":   "gno.land/r/foo",
		"profile": "testnet5",
	})
	require.Error(t, err, "expected error when expr is missing")
}

func TestEval_requiresRealm(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterEval(s, constResolver(chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_eval", map[string]any{
		"expr":    "Bar()",
		"profile": "testnet5",
	})
	require.Error(t, err, "expected error when realm is missing")
}

func TestEval_rejectsNonStringRealm(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterEval(s, constResolver(chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_eval", map[string]any{
		"realm":   42,
		"expr":    "Bar()",
		"profile": "testnet5",
	})
	require.Error(t, err, "expected type error when realm is not a string")
}

func TestEval_rejectsNonStringExpr(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterEval(s, constResolver(chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_eval", map[string]any{
		"realm":   "gno.land/r/foo",
		"expr":    42,
		"profile": "testnet5",
	})
	require.Error(t, err, "expected type error when expr is not a string")
}

func TestEval_unknownProfileReturnsError(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterEval(s, onlyProfileResolver("testnet5", chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_eval", map[string]any{
		"realm":   "gno.land/r/foo",
		"expr":    "Bar()",
		"profile": "ghost",
	})
	require.Error(t, err, "expected error when resolver returns nil for unknown profile")
}
