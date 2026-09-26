package server

import (
	"fmt"
	"path/filepath"

	"github.com/vitorhugo/npcai/internal/config"
	"github.com/vitorhugo/npcai/internal/engine"
	"go.uber.org/fx"
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
	for _, path := range paths {
		profile, err := engine.LoadProfile(path)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", path, err)
		}
		if profile.Name == "" {
			return nil, fmt.Errorf("profile %s has no name", path)
		}
		profiles[profile.Name] = profile
	}
	return NewProfileCache(profiles), nil
}
