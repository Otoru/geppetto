package engine

import (
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seededRand(seed uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed, seed+1))
}

func TestRankedPreferencesKeepsStochasticPickFirst(t *testing.T) {
	a := testAgent(c("HUNGER", -70), c("FUN", -10))
	ps := []AffordanceProvider{
		provider("food", Position{}, action("eat", map[string]float64{"HUNGER": 80})),
		provider("fun", Position{}, action("play", map[string]float64{"FUN": 80})),
	}
	tuning := DefaultTuning()
	for seed := uint64(0); seed < 50; seed++ {
		ranked := RankedPreferences(a, ps, tuning, seededRand(seed))
		require.NotEmpty(t, ranked)
		picked := SelectAction(a, ps, tuning, seededRand(seed))
		require.NotNil(t, picked)
		assert.Equal(t, picked.Action.ActionID, ranked[0].Action.ActionID, "first preference must be the stochastic pick, not the argmax")
		assert.Equal(t, picked.ProviderID, ranked[0].Provider.ID)
	}
}

func TestRankedPreferencesFallbacksAreUtilityOrderedAndCapped(t *testing.T) {
	a := testAgent(c("HUNGER", -70))
	ps := []AffordanceProvider{
		provider("p1", Position{}, action("a1", map[string]float64{"HUNGER": 80})),
		provider("p2", Position{}, action("a2", map[string]float64{"HUNGER": 60})),
		provider("p3", Position{}, action("a3", map[string]float64{"HUNGER": 40})),
		provider("p4", Position{}, action("a4", map[string]float64{"HUNGER": 20})),
	}
	tuning := DefaultTuning()
	tuning.ReconciliationTopK = 3

	ranked := RankedPreferences(a, ps, tuning, seededRand(1))

	require.Len(t, ranked, 3, "retained candidates must respect the configured cap")
	for i := 2; i < len(ranked); i++ {
		assert.GreaterOrEqual(t, ranked[i-1].Utility, ranked[i].Utility, "fallbacks must follow descending utility")
	}
	for _, fallback := range ranked[1:] {
		assert.NotEqual(t, ranked[0].Action.ActionID, fallback.Action.ActionID, "fallbacks must not repeat the first pick")
	}
}

func TestRankedPreferencesDefaultsToConstantWhenTuningIsZero(t *testing.T) {
	a := testAgent(c("HUNGER", -70))
	ps := []AffordanceProvider{
		provider("p1", Position{}, action("a1", map[string]float64{"HUNGER": 80})),
		provider("p2", Position{}, action("a2", map[string]float64{"HUNGER": 60})),
	}
	tuning := DefaultTuning()
	tuning.ReconciliationTopK = 0

	ranked := RankedPreferences(a, ps, tuning, seededRand(1))

	assert.NotEmpty(t, ranked)
}

func TestResolveContention_NearestAgentWinsSingleSlot(t *testing.T) {
	chair := provider("chair", Position{}, action("sit", map[string]float64{"HUNGER": 20}))
	near := testAgent(c("HUNGER", -50))
	near.ID = "near"
	near.Position = Position{X: 1}
	far := testAgent(c("HUNGER", -50))
	far.ID = "far"
	far.Position = Position{X: 9}
	providers := []AffordanceProvider{chair}
	agents := []Agent{near, far}
	preferences := [][]Candidate{
		RankedPreferences(near, providers, DefaultTuning(), seededRand(1)),
		RankedPreferences(far, providers, DefaultTuning(), seededRand(2)),
	}

	result := ResolveContention(agents, preferences)

	require.NotNil(t, result[0])
	assert.Equal(t, "chair", result[0].ProviderID)
	assert.Nil(t, result[1], "the farther agent must lose the single slot")
}

func TestResolveContention_HungrierButFartherAgentLosesToCloserOne(t *testing.T) {
	food := provider("food", Position{}, action("eat", map[string]float64{"HUNGER": 80}))
	starving := testAgent(c("HUNGER", -95))
	starving.ID = "starving"
	starving.Position = Position{X: 5}
	peckish := testAgent(c("HUNGER", -10))
	peckish.ID = "peckish"
	peckish.Position = Position{X: 1}

	starvingUtility := ScoreAction(starving, food, food.AdvertisedActions[0], DefaultTuning())
	peckishUtility := ScoreAction(peckish, food, food.AdvertisedActions[0], DefaultTuning())
	require.Greater(t, starvingUtility, peckishUtility, "premise: the starving agent scores higher, and still must lose")

	providers := []AffordanceProvider{food}
	agents := []Agent{starving, peckish}
	preferences := [][]Candidate{
		RankedPreferences(starving, providers, DefaultTuning(), seededRand(1)),
		RankedPreferences(peckish, providers, DefaultTuning(), seededRand(2)),
	}

	result := ResolveContention(agents, preferences)

	assert.Nil(t, result[0], "starving but farther: hunger does not make anyone faster")
	require.NotNil(t, result[1])
	assert.Equal(t, "food", result[1].ProviderID)
}

func TestResolveContention_LoserFallsBackToNextCandidate(t *testing.T) {
	chair := provider("chair", Position{}, action("sit", map[string]float64{"HUNGER": 50}))
	bench := provider("bench", Position{X: 20}, action("sit", map[string]float64{"HUNGER": 10}))
	near := testAgent(c("HUNGER", -50))
	near.ID = "near"
	near.Position = Position{X: 1}
	far := testAgent(c("HUNGER", -50))
	far.ID = "far"
	far.Position = Position{X: 9}
	providers := []AffordanceProvider{chair, bench}
	agents := []Agent{near, far}
	preferences := [][]Candidate{
		RankedPreferences(near, providers, DefaultTuning(), seededRand(1)),
		RankedPreferences(far, providers, DefaultTuning(), seededRand(2)),
	}
	require.Equal(t, "chair", preferences[1][0].Provider.ID, "premise: both agents prefer the chair")

	result := ResolveContention(agents, preferences)

	require.NotNil(t, result[0])
	assert.Equal(t, "chair", result[0].ProviderID)
	require.NotNil(t, result[1], "the loser must fall back to its own next candidate")
	assert.Equal(t, "bench", result[1].ProviderID)
}

func TestResolveContention_CapacityGrantsSlotsToNearestAgents(t *testing.T) {
	hall := provider("hall", Position{}, action("gather", map[string]float64{"HUNGER": 20}))
	hall.Capacity = 3
	providers := []AffordanceProvider{hall}
	agents := make([]Agent, 5)
	preferences := make([][]Candidate, 5)
	for i := range agents {
		agents[i] = testAgent(c("HUNGER", -50))
		agents[i].ID = string(rune('a' + i))
		agents[i].Position = Position{X: float64(i + 1)}
		preferences[i] = RankedPreferences(agents[i], providers, DefaultTuning(), seededRand(uint64(i)))
	}

	result := ResolveContention(agents, preferences)

	for i := 0; i < 3; i++ {
		require.NotNil(t, result[i], "agent at distance %d is among the 3 nearest", i+1)
		assert.Equal(t, "hall", result[i].ProviderID)
	}
	assert.Nil(t, result[3])
	assert.Nil(t, result[4])
}

func TestResolveContention_OccupantsConsumeSlots(t *testing.T) {
	chair := provider("chair", Position{}, action("sit", map[string]float64{"HUNGER": 20}))
	chair.Capacity = 2
	chair.Occupants = []string{"someone-else"}
	providers := []AffordanceProvider{chair}
	near := testAgent(c("HUNGER", -50))
	near.ID = "near"
	near.Position = Position{X: 1}
	far := testAgent(c("HUNGER", -50))
	far.ID = "far"
	far.Position = Position{X: 2}
	agents := []Agent{near, far}
	preferences := [][]Candidate{
		RankedPreferences(near, providers, DefaultTuning(), seededRand(1)),
		RankedPreferences(far, providers, DefaultTuning(), seededRand(2)),
	}

	result := ResolveContention(agents, preferences)

	require.NotNil(t, result[0], "one free slot remains (capacity 2 minus 1 occupant)")
	assert.Nil(t, result[1])
}

func TestResolveContention_CascadesUntilStable(t *testing.T) {
	chair := provider("chair", Position{}, action("sit", map[string]float64{"HUNGER": 80}))
	sofa := provider("sofa", Position{}, action("lounge", map[string]float64{"HUNGER": 40}))
	chairCandidate := Candidate{Action: chair.AdvertisedActions[0], Provider: chair, Utility: 10}
	sofaCandidate := Candidate{Action: sofa.AdvertisedActions[0], Provider: sofa, Utility: 5}
	closest := Agent{ID: "closest", Position: Position{X: 0.5}}
	middle := Agent{ID: "middle", Position: Position{X: 1}}
	farthest := Agent{ID: "farthest", Position: Position{X: 2}}
	agents := []Agent{closest, middle, farthest}
	preferences := [][]Candidate{
		{chairCandidate},                // closest: only wants the chair
		{chairCandidate, sofaCandidate}, // middle: chair, then sofa
		{sofaCandidate},                 // farthest: only wants the sofa
	}

	result := ResolveContention(agents, preferences)

	require.NotNil(t, result[0])
	assert.Equal(t, "chair", result[0].ProviderID)
	require.NotNil(t, result[1], "middle loses the chair and falls back to the sofa")
	assert.Equal(t, "sofa", result[1].ProviderID)
	assert.Nil(t, result[2], "farthest is dislodged from the sofa by the cascade and has no fallback")
}

func TestResolveContention_DeterministicAcrossRuns(t *testing.T) {
	food := provider("food", Position{}, action("eat", map[string]float64{"HUNGER": 80}))
	bed := provider("bed", Position{X: 5}, action("rest", map[string]float64{"HUNGER": 30}))
	providers := []AffordanceProvider{food, bed}
	run := func() []string {
		agents := []Agent{
			{ID: "a", Considerations: considerations([]Consideration{c("HUNGER", -90)}), Position: Position{X: 8}},
			{ID: "b", Considerations: considerations([]Consideration{c("HUNGER", -50)}), Position: Position{X: 2}},
			{ID: "c", Considerations: considerations([]Consideration{c("HUNGER", -70)}), Position: Position{X: 4}},
		}
		preferences := make([][]Candidate, len(agents))
		for i, agent := range agents {
			preferences[i] = RankedPreferences(agent, providers, DefaultTuning(), seededRand(42+uint64(i)))
		}
		result := ResolveContention(agents, preferences)
		assignments := make([]string, len(result))
		for i, instance := range result {
			if instance != nil {
				assignments[i] = instance.ProviderID + "/" + instance.Action.ActionID
			}
		}
		return assignments
	}

	first := run()
	for i := 0; i < 10; i++ {
		assert.Equal(t, first, run(), "same input and same seeds must produce the same assignment")
	}
}

func TestResolveContention_NeverExceedsProviderSlots(t *testing.T) {
	providers := []AffordanceProvider{
		provider("one", Position{}, action("use1", map[string]float64{"HUNGER": 50})),
		provider("two", Position{X: 10}, action("use2", map[string]float64{"HUNGER": 40})),
		provider("three", Position{X: 20}, action("use3", map[string]float64{"HUNGER": 30})),
	}
	providers[1].Capacity = 2
	providers[2].Capacity = 3
	capacities := map[string]int{"one": 1, "two": 2, "three": 3}

	for seed := uint64(0); seed < 20; seed++ {
		agents := make([]Agent, 20)
		preferences := make([][]Candidate, 20)
		for i := range agents {
			agents[i] = testAgent(c("HUNGER", -float64(10+i*4)))
			agents[i].ID = string(rune('a'+i)) + "!"
			agents[i].Position = Position{X: float64((i*7 + int(seed)) % 25)}
			preferences[i] = RankedPreferences(agents[i], providers, DefaultTuning(), seededRand(seed*100+uint64(i)))
		}

		result := ResolveContention(agents, preferences)

		assigned := map[string]int{}
		for _, instance := range result {
			if instance != nil {
				assigned[instance.ProviderID]++
			}
		}
		for providerID, count := range assigned {
			assert.LessOrEqual(t, count, capacities[providerID], "provider %s received more agents than slots (seed %d)", providerID, seed)
		}
	}
}
