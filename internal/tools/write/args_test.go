package write

import (
	"testing"
)

// ---- stringArg tests

func Test_stringArg_present(t *testing.T) {
	v, err := stringArg(map[string]any{"realm": "gno.land/r/x"}, "realm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "gno.land/r/x" {
		t.Errorf("got %q", v)
	}
}

func Test_stringArg_missing(t *testing.T) {
	v, err := stringArg(map[string]any{}, "realm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "" {
		t.Errorf("got %q, want empty", v)
	}
}

func Test_stringArg_wrongType(t *testing.T) {
	_, err := stringArg(map[string]any{"realm": 42}, "realm")
	if err == nil {
		t.Fatal("expected error for non-string value")
	}
}

// ---- boolArg tests

func Test_boolArg_present_true(t *testing.T) {
	v, err := boolArg(map[string]any{"simulate": true}, "simulate")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !v {
		t.Error("expected true")
	}
}

func Test_boolArg_missing_returnsFalse(t *testing.T) {
	v, err := boolArg(map[string]any{}, "simulate")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v {
		t.Error("expected false for missing key")
	}
}

func Test_boolArg_wrongType_errors(t *testing.T) {
	_, err := boolArg(map[string]any{"simulate": "yes"}, "simulate")
	if err == nil {
		t.Fatal("expected error for non-bool value")
	}
}

// ---- stringSliceArg tests

func Test_stringSliceArg_present_validStrings(t *testing.T) {
	v, err := stringSliceArg(map[string]any{
		"allow_paths": []any{"gno.land/r/x", "gno.land/r/y"},
	}, "allow_paths")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(v) != 2 || v[0] != "gno.land/r/x" || v[1] != "gno.land/r/y" {
		t.Errorf("got %v", v)
	}
}

func Test_stringSliceArg_missing_returnsNil(t *testing.T) {
	v, err := stringSliceArg(map[string]any{}, "allow_paths")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != nil {
		t.Errorf("expected nil, got %v", v)
	}
}

func Test_stringSliceArg_nonStringElement_errors(t *testing.T) {
	_, err := stringSliceArg(map[string]any{
		"allow_paths": []any{"gno.land/r/x", 42},
	}, "allow_paths")
	if err == nil {
		t.Fatal("expected error for non-string element")
	}
}

// ---- addProfileArg tests

func Test_addProfileArg_filtersToWritable(t *testing.T) {
	s := newBaseTestServer(t) // has "testnet5" with a master-address (writable)
	// The enum must contain exactly the writable profiles.
	props := map[string]any{}
	var required []string
	addProfileArg(s, props, &required)

	profileProp, ok := props["profile"].(map[string]any)
	if !ok {
		t.Fatal("profile prop missing or wrong type")
	}
	enum, ok := profileProp["enum"].([]string)
	if !ok {
		t.Fatal("enum field missing or wrong type")
	}
	// Base server has one writable profile: "testnet5"
	if len(enum) != 1 || enum[0] != "testnet5" {
		t.Errorf("enum = %v, want [testnet5]", enum)
	}
}
