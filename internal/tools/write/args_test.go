package write

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- stringArg tests

func Test_stringArg_present(t *testing.T) {
	v, err := stringArg(map[string]any{"realm": "gno.land/r/x"}, "realm")
	require.NoError(t, err)
	assert.Equal(t, "gno.land/r/x", v)
}

func Test_stringArg_missing(t *testing.T) {
	v, err := stringArg(map[string]any{}, "realm")
	require.NoError(t, err)
	assert.Equal(t, "", v)
}

func Test_stringArg_wrongType(t *testing.T) {
	_, err := stringArg(map[string]any{"realm": 42}, "realm")
	require.Error(t, err, "expected error for non-string value")
}

// ---- boolArg tests

func Test_boolArg_present_true(t *testing.T) {
	v, err := boolArg(map[string]any{"simulate": true}, "simulate")
	require.NoError(t, err)
	assert.True(t, v)
}

func Test_boolArg_missing_returnsFalse(t *testing.T) {
	v, err := boolArg(map[string]any{}, "simulate")
	require.NoError(t, err)
	assert.False(t, v, "expected false for missing key")
}

func Test_boolArg_wrongType_errors(t *testing.T) {
	_, err := boolArg(map[string]any{"simulate": "yes"}, "simulate")
	require.Error(t, err, "expected error for non-bool value")
}

// ---- stringSliceArg tests

func Test_stringSliceArg_present_validStrings(t *testing.T) {
	v, err := stringSliceArg(map[string]any{
		"allow_paths": []any{"gno.land/r/x", "gno.land/r/y"},
	}, "allow_paths")
	require.NoError(t, err)
	assert.Equal(t, []string{"gno.land/r/x", "gno.land/r/y"}, v)
}

func Test_stringSliceArg_missing_returnsNil(t *testing.T) {
	v, err := stringSliceArg(map[string]any{}, "allow_paths")
	require.NoError(t, err)
	assert.Nil(t, v)
}

func Test_stringSliceArg_nonStringElement_errors(t *testing.T) {
	_, err := stringSliceArg(map[string]any{
		"allow_paths": []any{"gno.land/r/x", 42},
	}, "allow_paths")
	require.Error(t, err, "expected error for non-string element")
}

// ---- addProfileArg tests

func Test_addProfileArg_filtersToWritable(t *testing.T) {
	s := newBaseTestServer(t) // has "testnet5" with a master-address (writable)
	// The enum must contain exactly the writable profiles.
	props := map[string]any{}
	var required []string
	addProfileArg(s, props, &required)

	profileProp, ok := props["profile"].(map[string]any)
	require.True(t, ok, "profile prop missing or wrong type")
	enum, ok := profileProp["enum"].([]string)
	require.True(t, ok, "enum field missing or wrong type")
	// Base server has one writable profile: "testnet5"
	assert.Equal(t, []string{"testnet5"}, enum)
}
