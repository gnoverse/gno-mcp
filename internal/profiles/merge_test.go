package profiles

import (
	"strings"
	"testing"
)

func TestMerge_LaterOverridesByName(t *testing.T) {
	base := BuiltinProfiles()
	overlay, err := Load(strings.NewReader(`
[testnet]
rpc-url = "https://rpc.test11.testnets.gno.land:443"
chain-id = "test11"
master-address = "g17ernafy6ctpcz6uepfsq2js8x2vz0wladh5yc3"
`))
	if err != nil {
		t.Fatalf("load overlay: %v", err)
	}
	merged := Merge(base, overlay.Profiles)
	if merged["testnet"].MasterAddress == "" {
		t.Error("overlay should have added master-address to testnet")
	}
	if _, ok := merged["local"]; !ok {
		t.Error("base 'local' should survive the merge")
	}
}
