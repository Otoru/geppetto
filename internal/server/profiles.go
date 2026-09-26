package server

import (
	"fmt"
	"path/filepath"

	"github.com/vitorhugo/geppetto/internal/config"
	"github.com/vitorhugo/geppetto/internal/engine"
	"go.uber.org/fx"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

// ProfileParams groups the dependencies of ProvideProfiles. Tests mount it by
// hand; no fx.App is required to call the constructor directly.
type ProfileParams struct {
	fx.In
	Cfg *config.Config
	Log *zap.Logger
}

// ProvideProfiles loads the profile cache from the configured directory.
func ProvideProfiles(p ProfileParams) (*ProfileCache, error) {
	cache, err := LoadProfiles(p.Cfg.ConfigDir)
	if err != nil {
		return nil, err
	}
	p.Log.Info("profiles loaded", zap.String("dir", p.Cfg.ConfigDir), zap.Int("count", len(cache.profiles)))
	return cache, nil
}

// LoadProfiles reads profiles once at process startup. Requests refer to them
// by ID and never cause file I/O on the decision path.
func LoadProfiles(directory string) (*ProfileCache, error) {
	paths, err := filepath.Glob(filepath.Join(directory, "*.json"))
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no profile JSON files found in %q", directory)
	}
	profiles := make(map[string]engine.Profile, len(paths))
	var loadErrors error
	for _, path := range paths {
		profile, err := engine.LoadProfile(path)
		if err != nil {
			loadErrors = multierr.Append(loadErrors, fmt.Errorf("load %s: %w", path, err))
			continue
		}
		if profile.Name == "" {
			loadErrors = multierr.Append(loadErrors, fmt.Errorf("profile %s has no name", path))
			continue
		}
		profiles[profile.Name] = profile
	}
	if loadErrors != nil {
		return nil, loadErrors
	}
	return NewProfileCache(profiles), nil
}
