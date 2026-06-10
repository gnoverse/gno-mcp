package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- StringArg tests

func TestStringArg_present(t *testing.T) {
	v, err := StringArg(map[string]any{"realm": "gno.land/r/x"}, "realm")
	require.NoError(t, err)
	assert.Equal(t, "gno.land/r/x", v)
}

func TestStringArg_missing(t *testing.T) {
	v, err := StringArg(map[string]any{}, "realm")
	require.NoError(t, err)
	assert.Equal(t, "", v)
}

func TestStringArg_wrongType(t *testing.T) {
	_, err := StringArg(map[string]any{"realm": 42}, "realm")
	require.Error(t, err, "expected error for non-string value")
}

// ---- BoolArg tests

func TestBoolArg_present_true(t *testing.T) {
	v, err := BoolArg(map[string]any{"simulate": true}, "simulate")
	require.NoError(t, err)
	assert.True(t, v)
}

func TestBoolArg_missing_returnsFalse(t *testing.T) {
	v, err := BoolArg(map[string]any{}, "simulate")
	require.NoError(t, err)
	assert.False(t, v, "expected false for missing key")
}

func TestBoolArg_wrongType_errors(t *testing.T) {
	_, err := BoolArg(map[string]any{"simulate": "yes"}, "simulate")
	require.Error(t, err, "expected error for non-bool value")
}

// ---- StringSliceArg tests

func TestStringSliceArg_present_validStrings(t *testing.T) {
	v, err := StringSliceArg(map[string]any{
		"allow_paths": []any{"gno.land/r/x", "gno.land/r/y"},
	}, "allow_paths")
	require.NoError(t, err)
	assert.Equal(t, []string{"gno.land/r/x", "gno.land/r/y"}, v)
}

func TestStringSliceArg_missing_returnsNil(t *testing.T) {
	v, err := StringSliceArg(map[string]any{}, "allow_paths")
	require.NoError(t, err)
	assert.Nil(t, v)
}

func TestStringSliceArg_nonStringElement_errors(t *testing.T) {
	_, err := StringSliceArg(map[string]any{
		"allow_paths": []any{"gno.land/r/x", 42},
	}, "allow_paths")
	require.Error(t, err, "expected error for non-string element")
}
