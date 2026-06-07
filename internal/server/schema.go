package server

import (
	"sort"

	"github.com/gnoverse/gno-mcp/internal/profiles"
)

// ProfileSchema describes how the `profile` arg should appear on a tool.
type ProfileSchema struct {
	Enum     []string // sorted list of loaded profile names
	Default  string   // empty = no default; profile name = optional with this default
	Required bool     // if true, no default; agent must supply
}

// ProfileArgSchema builds the profile parameter schema from the loaded profiles
// and the result of local-gnodev discovery (discovered = profile name or "").
//
// Rules (per multichain ADR):
//   - Zero profiles loaded: required (no enum, no default possible).
//   - Local discovered (and present in cfg): optional, default to discovered name.
//   - Otherwise: optional, default to "testnet" if present, else first profile.
func ProfileArgSchema(cfg *profiles.Config, discoveredLocal string) ProfileSchema {
	names := make([]string, 0, len(cfg.Profiles))
	for n := range cfg.Profiles {
		names = append(names, n)
	}
	sort.Strings(names)

	// Defensive: if discovery returned a name that is not actually loaded,
	// treat as not discovered rather than emit a default outside the enum.
	if discoveredLocal != "" {
		if _, ok := cfg.Profiles[discoveredLocal]; !ok {
			discoveredLocal = ""
		}
	}

	switch {
	case len(names) == 0:
		return ProfileSchema{Required: true}
	case discoveredLocal != "":
		return ProfileSchema{Enum: names, Default: discoveredLocal}
	default:
		// Prefer "testnet" as the standing default; else the first profile.
		def := names[0]
		for _, n := range names {
			if n == "testnet" {
				def = "testnet"
				break
			}
		}
		return ProfileSchema{Enum: names, Default: def}
	}
}
