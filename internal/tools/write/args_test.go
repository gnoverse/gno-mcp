package write

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The shared arg helpers (server.StringArg & co.) are tested in
// internal/server/args_test.go.

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
