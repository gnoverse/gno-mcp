// Package server wires the MCP server scaffolding, tool Registry, and profile-conditional schema.
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
//   - Single profile loaded: optional, default to the only profile.
//   - Multiple profiles loaded, local discovered (and "local" is among them):
//     optional, default to "local".
//   - Multiple profiles loaded, no local discovered: required.
func ProfileArgSchema(cfg *profiles.Config, discoveredLocal string) ProfileSchema {
	names := make([]string, 0, len(cfg.Profiles))
	for n := range cfg.Profiles {
		names = append(names, n)
	}
	sort.Strings(names)

	switch {
	case len(names) == 1:
		return ProfileSchema{Enum: names, Default: names[0]}
	case discoveredLocal != "":
		return ProfileSchema{Enum: names, Default: discoveredLocal}
	default:
		return ProfileSchema{Enum: names, Required: true}
	}
}
