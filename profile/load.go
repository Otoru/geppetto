// Package profile loads and validates geppetto JSON profiles.
package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"

	"github.com/Otoru/geppetto"
)

// ErrInvalidProfile identifies malformed or semantically incomplete profile
// documents.
var ErrInvalidProfile = errors.New("invalid profile")

// ErrNoProfiles identifies a directory with no JSON profile documents.
var ErrNoProfiles = errors.New("no profiles")

// Load reads and validates one JSON profile from path.
func Load(path string) (geppetto.Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return geppetto.Profile{}, err
	}
	return decode(data)
}

// LoadFS reads and validates one JSON profile from an fs.FS, allowing callers
// to load profiles embedded with go:embed.
func LoadFS(files fs.FS, name string) (geppetto.Profile, error) {
	data, err := fs.ReadFile(files, name)
	if err != nil {
		return geppetto.Profile{}, err
	}
	return decode(data)
}

// LoadDir reads every JSON profile immediately within directory, indexed by
// profile name. All documents are attempted so a caller receives every load
// failure in one result.
func LoadDir(directory string) (map[string]geppetto.Profile, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && path.Ext(entry.Name()) == ".json" {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return nil, fmt.Errorf("%w in %q", ErrNoProfiles, directory)
	}
	profiles := make(map[string]geppetto.Profile, len(names))
	var loadErrors []error
	for _, name := range names {
		loaded, loadErr := Load(path.Join(directory, name))
		if loadErr != nil {
			loadErrors = append(loadErrors, fmt.Errorf("load %s: %w", path.Join(directory, name), loadErr))
			continue
		}
		profiles[loaded.Name] = loaded
	}
	if len(loadErrors) != 0 {
		return nil, errors.Join(loadErrors...)
	}
	return profiles, nil
}

// Validate checks the invariants required to identify a loaded profile. The
// engine deliberately permits sparse tuning and consideration definitions, so
// their zero values retain the same defaulting behavior as direct Go use.
func Validate(profile geppetto.Profile) error {
	if profile.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidProfile)
	}
	return nil
}

func decode(data []byte) (geppetto.Profile, error) {
	var profile geppetto.Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		return geppetto.Profile{}, fmt.Errorf("%w: %v", ErrInvalidProfile, err)
	}
	if err := Validate(profile); err != nil {
		return geppetto.Profile{}, err
	}
	return profile, nil
}
