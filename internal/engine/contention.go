package engine

import (
	"cmp"
	"math/rand/v2"
	"slices"
)

// RankedPreferences returns the agent's candidates ordered for contention
// resolution: the stochastic softmax pick first (identical to what
// SelectAction would choose with the same rng), then the remaining candidates
// in descending utility as fallbacks, capped at the configured limit.
func RankedPreferences(agent Agent, providers []AffordanceProvider, tuning Tuning, rng *rand.Rand) []Candidate {
	candidates := Candidates(agent, providers, tuning)
	if len(candidates) == 0 {
		return nil
	}

	temperature := tuning.SelectionTemperature
	if temperature <= 0 {
		temperature = SELECTION_TEMPERATURE
	}
	chosen := sampleSoftmax(topCandidates(candidates, tuning.SelectionTopK), temperature, rng)

	limit := tuning.ReconciliationTopK
	if limit <= 0 {
		limit = RECONCILIATION_TOP_K
	}
	ranked := make([]Candidate, 0, min(len(candidates), limit))
	ranked = append(ranked, chosen)
	skipped := false
	for _, candidate := range candidates {
		if len(ranked) >= cap(ranked) {
			break
		}
		if !skipped && candidate.Action.ActionID == chosen.Action.ActionID && candidate.Provider.ID == chosen.Provider.ID {
			skipped = true
			continue
		}
		ranked = append(ranked, candidate)
	}
	return ranked
}

// ResolveContention distributes contested provider slots to the nearest
// agents. preferences[i] is agent i's ordered candidate list, as produced by
// RankedPreferences. It returns one ActionInstance per agent, nil when the
// agent exhausted its candidates. Free slots are Capacity minus the occupants
// reported by the client.
//
// The loop terminates because a displaced agent's cursor only moves forward
// through its own finite list: each agent is displaced from a given candidate
// at most once, so total displacements are bounded by the sum of the list
// lengths.
func ResolveContention(agents []Agent, preferences [][]Candidate) []*ActionInstance {
	cursors := make([]int, len(agents))
	assigned := make([]int, len(agents))
	for i := range assigned {
		assigned[i] = -1
	}

	for {
		displaced := false
		claims := map[string][]int{}
		for i := range agents {
			if cursors[i] >= len(preferences[i]) {
				continue
			}
			providerID := preferences[i][cursors[i]].Provider.ID
			claims[providerID] = append(claims[providerID], i)
		}
		// Each agent appears in exactly one claim list per round, so map
		// iteration order cannot affect the outcome.
		for _, claimants := range claims {
			slices.SortFunc(claimants, func(a, b int) int {
				distanceA := agents[a].Position.Distance(preferences[a][cursors[a]].Provider.Position)
				distanceB := agents[b].Position.Distance(preferences[b][cursors[b]].Provider.Position)
				if d := cmp.Compare(distanceA, distanceB); d != 0 {
					return d
				}
				if d := cmp.Compare(agents[a].ID, agents[b].ID); d != 0 {
					return d
				}
				return cmp.Compare(a, b)
			})
			provider := preferences[claimants[0]][cursors[claimants[0]]].Provider
			slots := provider.Capacity - len(provider.Occupants)
			for rank, agentIndex := range claimants {
				if rank < slots {
					assigned[agentIndex] = cursors[agentIndex]
				} else {
					assigned[agentIndex] = -1
					cursors[agentIndex]++
					displaced = true
				}
			}
		}
		if !displaced {
			break
		}
	}

	result := make([]*ActionInstance, len(agents))
	for i := range agents {
		if assigned[i] < 0 {
			continue
		}
		chosen := preferences[i][assigned[i]]
		result[i] = &ActionInstance{
			Action:              chosen.Action,
			ProviderID:          chosen.Provider.ID,
			ContinuationUtility: chosen.Utility,
		}
	}
	return result
}
