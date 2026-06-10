package profiles

import (
	"bytes"
	"fmt"

	"github.com/BurntSushi/toml"

	"github.com/gnoverse/gno-mcp/internal/fsutil"
)

// WriteFile validates the profiles and writes them to path as TOML, atomically,
// with 0600 permissions. The parent directory is created if needed. Validation
// runs first so a forbidden chain-id can never be persisted. An empty profile
// set is allowed (e.g. after removing the last user profile) — the built-in
// defaults still apply, so an empty global file is legitimate.
func WriteFile(path string, profiles map[string]Profile) error {
	if len(profiles) > 0 {
		cfg := &Config{Profiles: profiles}
		if _, err := cfg.Validate(); err != nil {
			return fmt.Errorf("refusing to write invalid config: %w", err)
		}
	}

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(profiles); err != nil {
		return fmt.Errorf("encode toml: %w", err)
	}

	if err := fsutil.WriteFileAtomic(path, buf.Bytes(), 0o600); err != nil {
		return fmt.Errorf("write profiles: %w", err)
	}
	return nil
}
