package profiles

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuiltinProfiles_AllowlistAndShape(t *testing.T) {
	cfg := &Config{Profiles: BuiltinProfiles()}
	_, err := cfg.Validate()
	require.NoError(t, err, "built-in defaults must validate")

	local, ok := cfg.Profiles["local"]
	if !ok || local.ChainID != "dev" {
		assert.Fail(t, "local default missing or wrong chain-id", "%+v", local)
	}
	tn, ok := cfg.Profiles["testnet"]
	if !ok || tn.ChainID != "test11" {
		assert.Fail(t, "testnet default missing or wrong chain-id", "%+v", tn)
	}
	assert.Empty(t, local.MasterAddress, "built-in local must be read-only (no master-address)")
	assert.Empty(t, tn.MasterAddress, "built-in testnet must be read-only (no master-address)")
}
