package geppetto_test

import (
	"testing"

	"github.com/Otoru/geppetto"
	"github.com/stretchr/testify/require"
)

func TestDeciderUsesExplicitSeedForReproducibleDecisions(t *testing.T) {
	agent := geppetto.Agent{
		ID: "agent",
		Considerations: map[string]geppetto.Consideration{
			"HUNGER": {ID: "HUNGER", Value: -80, Min: -100, Max: 100, BaseWeight: 1, ResponseCurve: geppetto.ResponseCurve{Kind: geppetto.Convex, Exponent: 2}},
		},
	}
	providers := []geppetto.AffordanceProvider{{
		ID:       "kitchen",
		Capacity: 1,
		AdvertisedActions: []geppetto.AdvertisedAction{{
			ActionID:            "eat",
			AdvertisementRadius: 1,
			Deltas:              map[string]float64{"HUNGER": 40},
		}},
	}}
	decider := geppetto.NewDecider(geppetto.WithTuning(geppetto.DefaultTuning()))
	first := decider.Decide(agent, providers, 41)
	second := decider.Decide(agent, providers, 41)
	require.NotNil(t, first)
	require.Equal(t, first, second)
}

func TestSimulatorAdvancesWithExplicitSeed(t *testing.T) {
	profile := geppetto.Profile{Tuning: geppetto.DefaultTuning()}
	agent := geppetto.Agent{Considerations: map[string]geppetto.Consideration{
		"ENERGY": {ID: "ENERGY", Value: 10, Min: -100, Max: 100, Updater: geppetto.ConsiderationUpdater{Kind: geppetto.LinearRegen, Rate: 60}},
	}}
	simulator := geppetto.NewSimulator(geppetto.WithProfile(profile))
	result := simulator.Advance(&agent, nil, geppetto.TickRequest{}, 99)
	require.Equal(t, geppetto.Full, result.SimulationLevel)
	require.Equal(t, 11.0, agent.Considerations["ENERGY"].Value)
}
