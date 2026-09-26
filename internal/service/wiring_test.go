package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Otoru/geppetto/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestProvideProfilesFromParams(t *testing.T) {
	directory := t.TempDir()
	profile := []byte(`{"name":"test","considerations":[{"id":"ENERGY","value":100,"min":-100,"max":100,"base_weight":1,"critical_threshold":-50,"response_curve":{"kind":"convex","exponent":2}}],"tuning":{"SELECTION_TEMPERATURE":1}}`)
	require.NoError(t, os.WriteFile(filepath.Join(directory, "test.json"), profile, 0o600))
	cache, err := ProvideProfiles(ProfileParams{Cfg: &config.Config{ConfigDir: directory}, Log: zap.NewNop()})
	require.NoError(t, err)
	assert.NotNil(t, cache)
}

func TestProvideProfilesPropagatesLoadErrors(t *testing.T) {
	_, err := ProvideProfiles(ProfileParams{Cfg: &config.Config{ConfigDir: t.TempDir()}, Log: zap.NewNop()})
	assert.Error(t, err)
}

func TestLoadProfilesReportsEveryInvalidProfile(t *testing.T) {
	directory := t.TempDir()
	firstProfilePath := filepath.Join(directory, "first-invalid.json")
	secondProfilePath := filepath.Join(directory, "second-invalid.json")
	require.NoError(t, os.WriteFile(firstProfilePath, []byte("{"), 0o600))
	require.NoError(t, os.WriteFile(secondProfilePath, []byte("{"), 0o600))

	_, err := LoadProfiles(directory)

	require.Error(t, err)
	assert.ErrorContains(t, err, firstProfilePath)
	assert.ErrorContains(t, err, secondProfilePath)
}

func TestNewGRPCServerFromParams(t *testing.T) {
	cache := NewProfileCache(nil)
	result := NewGRPCServer(GRPCParams{Log: zap.NewNop(), Profiles: cache})
	assert.NotNil(t, result.Server)
	assert.NotNil(t, result.Health)
}
