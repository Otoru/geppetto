package profile

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The same engine, with no code changes, runs the four genre configurations by
// swapping only consideration tables, providers, tags, and parameters. Loads
// the four real profiles and verifies that each defines its own considerations
// and tuning.
func TestGenreProfilesUseSameMotor(t *testing.T) {
	for _, name := range []string{"social-life", "tactical-stealth", "survival-crafting", "open-world-rpg"} {
		t.Run(name, func(t *testing.T) {
			profile, err := Load(filepath.Join("..", "configs", name+".json"))
			require.NoError(t, err)
			assert.NotEmpty(t, profile.Considerations)
			assert.NotZero(t, profile.Tuning.SelectionTemperature)
		})
	}
}

func TestProfilePreservesConsiderationBounds(t *testing.T) {
	profile, err := Load(filepath.Join("..", "configs", "tactical-stealth.json"))
	require.NoError(t, err)
	require.NotEmpty(t, profile.Considerations)
	assert.InDelta(t, -100, profile.Considerations[0].Min, 0.0001)
	assert.InDelta(t, 0, profile.Considerations[0].Value, 0.0001)
	assert.InDelta(t, 100, profile.Considerations[0].Max, 0.0001)
}

// The social-life profile declares its LOD reconstruction table as data: a
// "meal" aggregated event restores HUNGER when an agent returns to full
// detail.
func TestProfileLoadsAggregatedEventEffects(t *testing.T) {
	profile, err := Load(filepath.Join("..", "configs", "social-life.json"))
	require.NoError(t, err)
	require.NotEmpty(t, profile.AggregatedEventEffects)
	assert.Equal(t, "meal", profile.AggregatedEventEffects[0].Kind)
	assert.InDelta(t, 120, profile.AggregatedEventEffects[0].Deltas["HUNGER"], 0.0001)
}

func TestLoadFS(t *testing.T) {
	profiles := fstest.MapFS{"profiles/test.json": &fstest.MapFile{Data: []byte(`{"name":"test"}`)}}
	loaded, err := LoadFS(profiles, "profiles/test.json")
	require.NoError(t, err)
	assert.Equal(t, "test", loaded.Name)
}

func TestLoadDirRejectsInvalidDocuments(t *testing.T) {
	directory := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(directory, "broken.json"), []byte("{"), 0o600))
	_, err := LoadDir(directory)
	require.ErrorIs(t, err, ErrInvalidProfile)
}
