package profiles

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMerge_LaterOverridesByName(t *testing.T) {
	base := BuiltinProfiles()
	overlay, err := Load(strings.NewReader(`
[testnet]
rpc-url = "https://rpc.test11.testnets.gno.land:443"
chain-id = "test11"
master-address = "g17ernafy6ctpcz6uepfsq2js8x2vz0wladh5yc3"
`))
	require.NoError(t, err, "load overlay")

	merged := Merge(base, overlay.Profiles)
	assert.NotEmpty(t, merged["testnet"].MasterAddress, "overlay should have added master-address to testnet")
	assert.Contains(t, merged, "local", "base 'local' should survive the merge")
}
