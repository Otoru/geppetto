package geppetto

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
	tuning = tuning.WithDefaults()
	var candidates []Candidate
	if tuning.PerceptionNoise > 0 {
		candidates = Candidates(perceivedAgent(agent, tuning.PerceptionNoise, rng), providers, tuning)
	} else {
		candidates = Candidates(agent, providers, tuning)
	}
	if len(candidates) == 0 {
		return nil
	}

	chosen := sampleSoftmax(topCandidates(candidates, tuning.SelectionTopK), tuning.SelectionTemperature, rng)
	if tuning.ConventionBreakProbability > 0 && shouldBreakConvention(tuning.ConventionBreakProbability, rng) {
		if contrarian, ok := conventionBreakingCandidate(candidates, rng); ok {
			chosen = contrarian
		}
	}

	limit := tuning.ReconciliationTopK
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

// actionFreeSlots returns how many agents can still join the action at tick
// start: Capacity minus reported Occupancy, or -1 (unlimited) when Capacity
// is 0. See the warning on AdvertisedAction.Capacity.
func actionFreeSlots(action AdvertisedAction) int {
	if action.Capacity <= 0 {
		return -1
	}
	return max(action.Capacity-action.Occupancy, 0)
}

// ResolveContention distributes contested slots to the nearest agents.
// preferences[i] is agent i's ordered candidate list, as produced by
// RankedPreferences. It returns one ActionInstance per agent, nil when the
// agent exhausted its candidates.
//
// Slots are enforced at TWO levels: the provider's (Capacity minus the
// occupants reported by the client) and the action's own (Capacity minus
// Occupancy; action capacity 0 means unlimited — see the warning on
// AdvertisedAction.Capacity). An agent wins its current candidate only with
// a free slot at both levels; the stricter level wins.
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
	state := contentionState{
		agents:      agents,
		preferences: preferences,
		cursors:     cursors,
		assigned:    assigned,
	}
	// Rounds continue until one complete round has no displacement. A
	// displaced agent's cursor only moves forward through its finite list,
	// so the loop terminates.
	for state.resolveRound() {
	}
	return state.instances()
}

// contentionState tracks each agent's current preference cursor and the
// candidate index assigned so far (-1 when the agent has no slot).
type contentionState struct {
	agents      []Agent
	preferences [][]Candidate
	cursors     []int
	assigned    []int
}

// resolveRound awards the current candidate of every agent that still has
// one. It reports whether any agent was displaced and must try its next
// preference. Each agent appears in exactly one claim list per round, so map
// iteration order cannot affect the outcome.
func (state *contentionState) resolveRound() bool {
	displaced := false
	for _, claimants := range state.claims() {
		if state.award(claimants) {
			displaced = true
		}
	}
	return displaced
}

func (state *contentionState) claims() map[string][]int {
	claims := map[string][]int{}
	for agentIndex := range state.agents {
		if state.cursors[agentIndex] >= len(state.preferences[agentIndex]) {
			continue
		}
		providerID := state.preferences[agentIndex][state.cursors[agentIndex]].Provider.ID
		claims[providerID] = append(claims[providerID], agentIndex)
	}
	return claims
}

func (state *contentionState) award(claimants []int) bool {
	state.sortClaimants(claimants)
	provider := state.preferences[claimants[0]][state.cursors[claimants[0]]].Provider
	providerSlots := provider.Capacity - len(provider.Occupants)
	// Action slots are per provider instance: action "saw" on bench X does
	// not share its cap with "saw" on bench Y. -1 = unlimited.
	actionSlots := map[string]int{}
	displaced := false
	for _, agentIndex := range claimants {
		if !state.giveSlot(agentIndex, actionSlots, &providerSlots) {
			displaced = true
		}
	}
	return displaced
}

func (state *contentionState) sortClaimants(claimants []int) {
	slices.SortFunc(claimants, func(left, right int) int {
		return state.compareClaimants(left, right)
	})
}

func (state *contentionState) compareClaimants(left, right int) int {
	distanceLeft := state.agents[left].Position.Distance(state.preferences[left][state.cursors[left]].Provider.Position)
	distanceRight := state.agents[right].Position.Distance(state.preferences[right][state.cursors[right]].Provider.Position)
	if order := cmp.Compare(distanceLeft, distanceRight); order != 0 {
		return order
	}
	if order := cmp.Compare(state.agents[left].ID, state.agents[right].ID); order != 0 {
		return order
	}
	return cmp.Compare(left, right)
}

func (state *contentionState) giveSlot(agentIndex int, actionSlots map[string]int, providerSlots *int) bool {
	candidate := state.preferences[agentIndex][state.cursors[agentIndex]]
	slots := trackedActionSlots(actionSlots, candidate.Action)
	if *providerSlots > 0 && slots != 0 {
		state.assigned[agentIndex] = state.cursors[agentIndex]
		*providerSlots--
		if slots > 0 {
			actionSlots[candidate.Action.ActionID] = slots - 1
		}
		return true
	}
	state.assigned[agentIndex] = -1
	state.cursors[agentIndex]++
	return false
}

func trackedActionSlots(actionSlots map[string]int, action AdvertisedAction) int {
	slots, tracked := actionSlots[action.ActionID]
	if !tracked {
		slots = actionFreeSlots(action)
		actionSlots[action.ActionID] = slots
	}
	return slots
}

func (state *contentionState) instances() []*ActionInstance {
	result := make([]*ActionInstance, len(state.agents))
	for agentIndex := range state.agents {
		if state.assigned[agentIndex] < 0 {
			continue
		}
		chosen := state.preferences[agentIndex][state.assigned[agentIndex]]
		result[agentIndex] = &ActionInstance{
			Action:              chosen.Action,
			ProviderID:          chosen.Provider.ID,
			ContinuationUtility: chosen.Utility,
		}
	}
	return result
}
