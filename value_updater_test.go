package geppetto_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vitorhugo/geppetto"
)

// A configured Value function takes precedence over the World map, observes
// the agent being updated, and its result is clamped to the consideration's
// own bounds like any other updater result.
func TestValueFunctionOverridesWorldMapAndClamps(t *testing.T) {
	var seenAgent geppetto.Agent
	threat := geppetto.Consideration{
		ID: "THREAT", Min: -100, Max: 100, BaseWeight: 1,
		Updater: geppetto.ConsiderationUpdater{
			Kind: geppetto.PerceptionDriven,
			Value: func(agent geppetto.Agent, world geppetto.World) float64 {
				seenAgent = agent
				return 2 * world.Perception["DISTRACTION"]
			},
		},
	}
	agent := geppetto.Agent{
		ID:             "guard",
		Considerations: map[string]geppetto.Consideration{"THREAT": threat},
	}
	world := geppetto.World{Perception: map[string]float64{
		"THREAT":      7, // ignored: the Value function wins over the map entry
		"DISTRACTION": 80,
	}}

	geppetto.UpdateConsiderations(&agent, world, 1)

	assert.Equal(t, "guard", seenAgent.ID)
	// 2*80 = 160, clamped to the consideration maximum.
	assert.InDelta(t, 100, agent.Considerations["THREAT"].Value, 0.0001)
}

// The Value extension point composes with the public decision path: a
// consideration derived from game state drives selection exactly like one
// loaded from a profile.
func TestValueFunctionDrivesSelection(t *testing.T) {
	visibleEnemy := true
	threat := geppetto.Consideration{
		ID: "THREAT", Min: -100, Max: 100, BaseWeight: 1,
		ResponseCurve: geppetto.ResponseCurve{Kind: geppetto.Convex, Exponent: 2},
		Updater: geppetto.ConsiderationUpdater{
			Kind: geppetto.PerceptionDriven,
			Value: func(geppetto.Agent, geppetto.World) float64 {
				if visibleEnemy {
					return -90
				}
				return 0
			},
		},
	}
	agent := geppetto.Agent{
		ID:             "guard",
		Considerations: map[string]geppetto.Consideration{"THREAT": threat},
	}
	providers := []geppetto.AffordanceProvider{{
		ID: "wall", Capacity: 1,
		AdvertisedActions: []geppetto.AdvertisedAction{{
			ActionID: "take_cover", Deltas: map[string]float64{"THREAT": 60},
			EstimatedDuration: 1, AdvertisementRadius: 10,
		}},
	}}
	decider := geppetto.NewDecider(geppetto.WithTuning(geppetto.DefaultTuning()))

	geppetto.UpdateConsiderations(&agent, geppetto.World{}, 1)
	chosen := decider.Decide(agent, providers, 7)
	require.NotNil(t, chosen)
	assert.Equal(t, "take_cover", chosen.Action.ActionID)

	visibleEnemy = false
	geppetto.UpdateConsiderations(&agent, geppetto.World{}, 1)
	assert.InDelta(t, 0, agent.Considerations["THREAT"].Value, 0.0001)
}
