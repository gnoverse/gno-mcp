package profiles

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// Sources locates the config files to layer over the built-in defaults.
// Precedence (low→high): built-in defaults < GlobalPath < ProjectPath <
// ExplicitPath. A missing file is skipped (never fatal). An ExplicitPath that
// is set but missing IS an error (the user named it explicitly).
type Sources struct {
	GlobalPath   string // e.g. ~/.config/gnomcp/profiles.toml
	ProjectPath  string // e.g. ./profiles.toml
	ExplicitPath string // -config flag / GNOMCP_CONFIG; "" = unset
}

// LoadResolved builds the effective config: built-in defaults overlaid by each
// present file in precedence order, then validated. Returns a single error;
// Validate's warning return is always nil so it is dropped.
func LoadResolved(s Sources) (*Config, error) {
	merged := BuiltinProfiles()

	layer := func(path string, required bool) error {
		if path == "" {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) && !required {
				return nil
			}
			return fmt.Errorf("open config %q: %w", path, err)
		}
		defer f.Close()
		c, err := Load(f)
		if err != nil {
			return err
		}
		merged = Merge(merged, c.Profiles)
		return nil
	}

	if err := layer(s.GlobalPath, false); err != nil {
		return nil, err
	}
	if err := layer(s.ProjectPath, false); err != nil {
		return nil, err
	}
	if err := layer(s.ExplicitPath, true); err != nil {
		return nil, err
	}

	cfg := &Config{Profiles: merged}
	if _, err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}
