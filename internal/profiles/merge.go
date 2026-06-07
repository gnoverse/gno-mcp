package profiles

// Merge returns a new profile map where each overlay entry overrides the base
// entry of the same name (whole-profile replacement, not field-level merge).
// Base is not mutated.
func Merge(base, overlay map[string]Profile) map[string]Profile {
	out := make(map[string]Profile, len(base)+len(overlay))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overlay {
		out[k] = v
	}
	return out
}
