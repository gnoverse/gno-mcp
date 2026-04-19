package config

import (
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundtrip(t *testing.T) {
	t.Setenv("GNO_MCP_CONFIG", filepath.Join(t.TempDir(), "c.json"))
	if err := Save(&Config{DefaultKey: "moul", DefaultNetwork: "gno.land", GasBuffer: 25}); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.DefaultKey != "moul" || got.GasBuffer != 25 {
		t.Errorf("mismatch: %+v", got)
	}
}
