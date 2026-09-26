package geppetto_test

import (
	"fmt"
	"math"

	"github.com/Otoru/geppetto"
)

// villageProfile is the smallest profile that still shows pressure, decay,
// and competing advertisements.
func villageProfile() geppetto.Profile {
	consideration := func(id string, value float64) geppetto.Consideration {
		return geppetto.Consideration{
			ID: id, Value: value, Min: -100, Max: 100, BaseWeight: 1,
			CriticalThreshold: -50,
			Updater:           geppetto.ConsiderationUpdater{Kind: geppetto.LinearDecay, Rate: 5},
			ResponseCurve:     geppetto.ResponseCurve{Kind: geppetto.Convex, Exponent: 2},
		}
	}
	return geppetto.Profile{
		Name: "village",
		Considerations: []geppetto.Consideration{
			consideration("HUNGER", 0),
			consideration("ENERGY", 0),
		},
		Tuning: geppetto.DefaultTuning(),
	}
}

// A hungry villager scores the advertisements around it and the kitchen wins.
// The same seed always reproduces the same choice.
func ExampleDecider_Decide() {
	profile := villageProfile()
	decider := geppetto.NewDecider(geppetto.WithProfile(profile))

	villager := geppetto.Agent{
		ID: "bram",
		Considerations: map[string]geppetto.Consideration{
			"HUNGER": {ID: "HUNGER", Value: -80, Min: -100, Max: 100, BaseWeight: 1, ResponseCurve: geppetto.ResponseCurve{Kind: geppetto.Convex, Exponent: 2}},
			"ENERGY": {ID: "ENERGY", Value: 50, Min: -100, Max: 100, BaseWeight: 1, ResponseCurve: geppetto.ResponseCurve{Kind: geppetto.Convex, Exponent: 2}},
		},
	}
	providers := []geppetto.AffordanceProvider{
		{
			ID: "kitchen", Capacity: 2,
			AdvertisedActions: []geppetto.AdvertisedAction{{
				ActionID: "eat", Deltas: map[string]float64{"HUNGER": 40},
				EstimatedDuration: 1, AdvertisementRadius: 50,
			}},
		},
		{
			ID: "bench", Capacity: 1,
			AdvertisedActions: []geppetto.AdvertisedAction{{
				ActionID: "rest", Deltas: map[string]float64{"ENERGY": 30},
				EstimatedDuration: 1, AdvertisementRadius: 50,
			}},
		},
	}

	chosen := decider.Decide(villager, providers, 42)
	fmt.Printf("bram chooses %s at %s\n", chosen.Action.ActionID, chosen.ProviderID)

	// Output:
	// bram chooses eat at kitchen
}

// One hour per tick: hunger and energy decay, the agent eats, then rests,
// then eats again as pressures shift. Every tick passes its own seed.
func ExampleSimulator_Advance() {
	profile := villageProfile()
	profile.Tuning.FullTickHours = 1
	simulator := geppetto.NewSimulator(geppetto.WithProfile(profile))

	villager := geppetto.Agent{
		ID: "marta",
		Considerations: map[string]geppetto.Consideration{
			"HUNGER": {ID: "HUNGER", Value: -70, Min: -100, Max: 100, BaseWeight: 1, CriticalThreshold: -50, Updater: geppetto.ConsiderationUpdater{Kind: geppetto.LinearDecay, Rate: 5}, ResponseCurve: geppetto.ResponseCurve{Kind: geppetto.Convex, Exponent: 2}},
			"ENERGY": {ID: "ENERGY", Value: -40, Min: -100, Max: 100, BaseWeight: 1, CriticalThreshold: -50, Updater: geppetto.ConsiderationUpdater{Kind: geppetto.LinearDecay, Rate: 5}, ResponseCurve: geppetto.ResponseCurve{Kind: geppetto.Convex, Exponent: 2}},
		},
	}
	providers := []geppetto.AffordanceProvider{
		{
			ID: "kitchen", Capacity: 1,
			AdvertisedActions: []geppetto.AdvertisedAction{{
				ActionID: "eat", Deltas: map[string]float64{"HUNGER": 70},
				EstimatedDuration: 1, AdvertisementRadius: 50,
			}},
		},
		{
			ID: "bench", Capacity: 1,
			AdvertisedActions: []geppetto.AdvertisedAction{{
				ActionID: "rest", Deltas: map[string]float64{"ENERGY": 60},
				EstimatedDuration: 1, AdvertisementRadius: 50,
			}},
		},
	}

	for tick := 1; tick <= 3; tick++ {
		simulator.Advance(&villager, providers, geppetto.TickRequest{NowHours: float64(tick)}, 42+uint64(tick))
		fmt.Printf("tick %d: HUNGER=%.0f ENERGY=%.0f action=%s\n",
			tick,
			villager.Considerations["HUNGER"].Value,
			villager.Considerations["ENERGY"].Value,
			villager.CurrentAction.Action.ActionID)
	}

	// Output:
	// tick 1: HUNGER=-75 ENERGY=-45 action=eat
	// tick 2: HUNGER=-10 ENERGY=-50 action=rest
	// tick 3: HUNGER=-15 ENERGY=5 action=eat
}

// A Value function derives THREAT from line-of-sight state that no JSON
// profile can express: the game closes over its own enemy list and the tick
// calls the function instead of reading a World map.
func ExampleConsiderationUpdater_Value() {
	enemies := []geppetto.Position{{X: 3}, {X: 40}}
	threat := geppetto.Consideration{
		ID: "THREAT", Min: -100, Max: 100, BaseWeight: 1,
		Updater: geppetto.ConsiderationUpdater{
			Kind: geppetto.PerceptionDriven,
			Value: func(agent geppetto.Agent, _ geppetto.World) float64 {
				nearest := math.Inf(1)
				for _, enemy := range enemies {
					nearest = math.Min(nearest, agent.Position.Distance(enemy))
				}
				return -nearest
			},
		},
	}
	guard := geppetto.Agent{
		ID:             "aldric",
		Considerations: map[string]geppetto.Consideration{"THREAT": threat},
	}

	geppetto.UpdateConsiderations(&guard, geppetto.World{}, 1)
	fmt.Printf("THREAT=%.0f\n", guard.Considerations["THREAT"].Value)

	// Output:
	// THREAT=-3
}
