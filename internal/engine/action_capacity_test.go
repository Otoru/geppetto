package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// workbench builds a provider with capacity 4 advertising "saw" (limited)
// and "hammer" (unlimited), plus agents queued nearest-first.
func workbench(sawCapacity int, agentCount int) ([]Agent, []AffordanceProvider) {
	bench := provider("bench", Position{},
		action("saw", map[string]float64{"HUNGER": 80}),
		action("hammer", map[string]float64{"HUNGER": 40}))
	bench.Capacity = 4
	bench.AdvertisedActions[0].Capacity = sawCapacity
	providers := []AffordanceProvider{bench}
	agents := make([]Agent, agentCount)
	for i := range agents {
		agents[i] = testAgent(c("HUNGER", -50))
		agents[i].ID = string(rune('a' + i))
		agents[i].Position = Position{X: float64(i + 1)}
	}
	return agents, providers
}

func rankedForAll(agents []Agent, providers []AffordanceProvider) [][]Candidate {
	preferences := make([][]Candidate, len(agents))
	for i, agent := range agents {
		preferences[i] = RankedPreferences(agent, providers, DefaultTuning(), seededRand(uint64(i+1)))
	}
	return preferences
}

// An action with capacity C admits at most C agents per batch, even with free
// slots left on the provider: a workbench with 4 slots and a saw with capacity
// 1 gets exactly 1 agent on the saw and at most 4 in total.
func TestActionCapacityLimitsPerAction(t *testing.T) {
	agents, providers := workbench(1, 5)
	preferences := rankedForAll(agents, providers)
	for i, preference := range preferences {
		require.Equal(t, "saw", preference[0].Action.ActionID, "premise: every agent prefers saw (agent %d)", i)
	}

	result := ResolveContention(agents, preferences)

	saw, hammer, unassigned := 0, 0, 0
	for _, instance := range result {
		switch {
		case instance == nil:
			unassigned++
		case instance.Action.ActionID == "saw":
			saw++
		case instance.Action.ActionID == "hammer":
			hammer++
		}
	}
	assert.Equal(t, 1, saw, "exactly one agent may operate the saw")
	assert.LessOrEqual(t, saw+hammer, 4, "provider capacity 4 bounds the total")
	assert.Equal(t, 1, unassigned, "5 agents, 4 provider slots: one is left out")
	require.NotNil(t, result[0])
	assert.Equal(t, "saw", result[0].Action.ActionID, "the nearest agent operates the saw")
}

// An action with capacity 0 imposes no limit of its own and never blocks
// candidates; only the provider's limit applies. (proto3 encodes an absent
// field as 0, so 0 must read as "no own limit" — otherwise every existing
// action would brick.)
func TestZeroActionCapacityMeansUnlimited(t *testing.T) {
	hall := provider("hall", Position{}, action("gather", map[string]float64{"HUNGER": 20}))
	hall.Capacity = 10
	// Capacity 0 on the action: proto3 encodes an absent field as 0, so 0 MUST
	// mean "no own limit" — otherwise every existing action would brick.
	hall.AdvertisedActions[0].Capacity = 0
	providers := []AffordanceProvider{hall}
	agents, _ := workbench(0, 0)
	agents = make([]Agent, 5)
	for i := range agents {
		agents[i] = testAgent(c("HUNGER", -50))
		agents[i].ID = string(rune('a' + i))
		agents[i].Position = Position{X: float64(i + 1)}
	}
	preferences := rankedForAll(agents, providers)

	result := ResolveContention(agents, preferences)

	for i, instance := range result {
		require.NotNil(t, instance, "agent %d: an uncapped action must not block anyone", i)
		assert.Equal(t, "gather", instance.Action.ActionID)
	}
}

// An action with capacity 0 remains eligible among the scored candidates: 0
// reads as unlimited, never as blocked.
func TestZeroActionCapacityStaysEligible(t *testing.T) {
	a := testAgent(c("HUNGER", -50))
	uncapped := action("gather", map[string]float64{"HUNGER": 20})
	uncapped.Capacity = 0

	candidates := Candidates(a, []AffordanceProvider{provider("hall", Position{}, uncapped)}, DefaultTuning())

	assert.Len(t, candidates, 1, "capacity 0 must read as unlimited, never as blocked")
}

// An action with capacity 3 on a provider with capacity 1 admits exactly 1
// agent: the stricter capacity level wins.
func TestStricterCapacityLevelWins(t *testing.T) {
	// Action allows 3, provider allows 1: the provider wins.
	room := provider("room", Position{}, action("perform", map[string]float64{"HUNGER": 20}))
	room.Capacity = 1
	room.AdvertisedActions[0].Capacity = 3
	providers := []AffordanceProvider{room}
	agents := make([]Agent, 3)
	for i := range agents {
		agents[i] = testAgent(c("HUNGER", -50))
		agents[i].ID = string(rune('a' + i))
		agents[i].Position = Position{X: float64(i + 1)}
	}
	preferences := rankedForAll(agents, providers)

	result := ResolveContention(agents, preferences)

	require.NotNil(t, result[0])
	assert.Equal(t, "perform", result[0].Action.ActionID)
	assert.Nil(t, result[1], "provider capacity 1 beats action capacity 3")
	assert.Nil(t, result[2])
}

// An action with occupancy equal to its capacity is not eligible to anyone on
// that tick.
func TestFullyOccupiedActionIsNotEligible(t *testing.T) {
	a := testAgent(c("HUNGER", -50))
	saw := action("saw", map[string]float64{"HUNGER": 80})
	saw.Capacity = 1
	saw.Occupancy = 1

	candidates := Candidates(a, []AffordanceProvider{provider("bench", Position{}, saw)}, DefaultTuning())

	assert.Empty(t, candidates, "an action at its occupancy limit must not be advertised to this tick")
}

// Client-reported occupancy consumes action slots before the batch: an action
// with capacity 2 and occupancy 1 admits only 1 more agent.
func TestActionOccupancyConsumesSlots(t *testing.T) {
	agents, providers := workbench(2, 2)
	providers[0].AdvertisedActions[0].Occupancy = 1 // one of the 2 saw slots is already taken
	preferences := rankedForAll(agents, providers)

	result := ResolveContention(agents, preferences)

	require.NotNil(t, result[0])
	assert.Equal(t, "saw", result[0].Action.ActionID, "the nearest agent gets the remaining saw slot")
	require.NotNil(t, result[1])
	assert.Equal(t, "hammer", result[1].Action.ActionID, "the second agent falls back: saw cap 2 minus 1 occupant leaves 1 slot")
}

// Random seed sweep: in no result does the per-action count exceed the
// action's capacity (when > 0), nor the per-provider count exceed the
// provider's capacity.
func TestTwoLevelCapacityInvariant(t *testing.T) {
	for seed := uint64(0); seed < 20; seed++ {
		bench := provider("bench", Position{},
			action("saw", map[string]float64{"HUNGER": 80}),
			action("hammer", map[string]float64{"HUNGER": 40}))
		bench.Capacity = 4
		bench.AdvertisedActions[0].Capacity = 1 + int(seed%2)
		providers := []AffordanceProvider{bench}
		agents := make([]Agent, 8)
		for i := range agents {
			agents[i] = testAgent(c("HUNGER", -float64(20+i*5)))
			agents[i].ID = string(rune('a'+i)) + "!"
			agents[i].Position = Position{X: float64((i*3 + int(seed)) % 10)}
			agents[i].Considerations["HUNGER"] = c("HUNGER", -float64(20+i*5))
		}
		preferences := make([][]Candidate, len(agents))
		for i, agent := range agents {
			preferences[i] = RankedPreferences(agent, providers, DefaultTuning(), seededRand(seed*100+uint64(i)))
		}

		result := ResolveContention(agents, preferences)

		perAction := map[string]int{}
		total := 0
		for _, instance := range result {
			if instance != nil {
				perAction[instance.Action.ActionID]++
				total++
			}
		}
		assert.LessOrEqual(t, perAction["saw"], bench.AdvertisedActions[0].Capacity, "seed %d: saw slots exceeded", seed)
		assert.LessOrEqual(t, total, 4, "seed %d: provider slots exceeded", seed)
	}
}

// The same input and the same seed produce exactly the same assignment,
// including disputes at both capacity levels (action and provider).
func TestTwoLevelContentionIsDeterministic(t *testing.T) {
	run := func() []string {
		agents, providers := workbench(1, 5)
		preferences := rankedForAll(agents, providers)
		result := ResolveContention(agents, preferences)
		assignments := make([]string, len(result))
		for i, instance := range result {
			if instance != nil {
				assignments[i] = instance.Action.ActionID
			}
		}
		return assignments
	}

	first := run()
	for i := 0; i < 10; i++ {
		assert.Equal(t, first, run(), "two-level contention must resolve deterministically")
	}
}
