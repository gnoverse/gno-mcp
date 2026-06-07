package profiles

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
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

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir config dir: %w", err)
	}
	// Unique temp file in the same dir → atomic rename, no fixed-name race and
	// no leftover on failure.
	tmp, err := os.CreateTemp(dir, ".profiles-*.toml")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod temp: %w", err)
	}
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp: %w", err)
	}
	return nil
}
