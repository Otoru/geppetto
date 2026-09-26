package engine

import (
	"encoding/json"
	"os"
)

// Profile defines the consideration order and tuning shared by a decision batch.
type Profile struct {
	Name           string          `json:"name"`
	Considerations []Consideration `json:"considerations"`
	Tuning         Tuning          `json:"tuning"`
	// AggregatedEventEffects is the LOD reconstruction table: how each
	// aggregated event kind rebuilds considerations on TransitionToFull. A
	// profile without it reconstructs nothing.
	AggregatedEventEffects []AggregatedEventEffect `json:"aggregated_event_effects,omitempty"`
}

// LoadProfile reads a JSON profile from path.
func LoadProfile(path string) (Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Profile{}, err
	}

	var profile Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		return Profile{}, err
	}
	return profile, nil
}
