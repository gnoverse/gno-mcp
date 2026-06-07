package profiles

import "testing"

func TestBuiltinProfiles_AllowlistAndShape(t *testing.T) {
	cfg := &Config{Profiles: BuiltinProfiles()}
	if _, err := cfg.Validate(); err != nil {
		t.Fatalf("built-in defaults must validate: %v", err)
	}
	local, ok := cfg.Profiles["local"]
	if !ok || local.ChainID != "dev" {
		t.Errorf("local default missing or wrong chain-id: %+v", local)
	}
	tn, ok := cfg.Profiles["testnet"]
	if !ok || tn.ChainID != "test11" {
		t.Errorf("testnet default missing or wrong chain-id: %+v", tn)
	}
	if local.MasterAddress != "" || tn.MasterAddress != "" {
		t.Error("built-in defaults must be read-only (no master-address)")
	}
}
