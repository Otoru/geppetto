package service

import (
	"github.com/vitorhugo/geppetto/internal/config"
	profileloader "github.com/vitorhugo/geppetto/profile"
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
	profiles, err := profileloader.LoadDir(directory)
	if err != nil {
		return nil, err
	}
	return NewProfileCache(profiles), nil
}
