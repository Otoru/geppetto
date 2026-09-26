package geppetto

import (
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdvanceFollowsFullTickConsiderationTrajectory(t *testing.T) {
	energy := c("ENERGY", 0)
	energy.Updater = ConsiderationUpdater{Kind: LinearDecay, Rate: 2}
	agent := testAgent(energy)
	agent.CurrentAction = &ActionInstance{Action: action("recover", map[string]float64{"ENERGY": 9})}
	agent.CurrentAction.Action.EstimatedDuration = 3
	profile := testTickProfile()
	profile.Tuning.FullTickHours = 1

	for tick, wantEnergy := range []float64{1, 2, 3} {
		result := Advance(&agent, profile, nil, TickRequest{NowHours: float64(tick + 1), TargetLevel: Full}, rand.New(rand.NewPCG(1, 2)))

		assert.Equal(t, Full, result.SimulationLevel)
		assert.InDelta(t, 1, result.ElapsedHours, 0.0001)
		assert.InDelta(t, wantEnergy, agent.Considerations["ENERGY"].Value, 0.0001)
	}
	assert.Nil(t, agent.CurrentAction)
}

func TestAdvancePreemptsAndPreservesPendingQueue(t *testing.T) {
	agent := testAgent(c("HUNGER", -60))
	agent.CurrentAction = &ActionInstance{Action: action("leisure", nil), ContinuationUtility: 1}
	agent.CurrentAction.Action.EstimatedDuration = 100
	EnqueueAction(&agent, ActionInstance{Action: action("return-to-post", nil)})
	profile := testTickProfile()
	profile.Tuning.FullTickHours = 1
	providers := []AffordanceProvider{provider("food", Position{}, action("eat", map[string]float64{"HUNGER": 80}))}

	Advance(&agent, profile, providers, TickRequest{NowHours: 1, TargetLevel: Full}, rand.New(rand.NewPCG(1, 2)))

	require.NotNil(t, agent.CurrentAction)
	assert.Equal(t, "eat", agent.CurrentAction.Action.ActionID)
	require.Len(t, agent.ActionQueue, 1)
	assert.Equal(t, "return-to-post", agent.ActionQueue[0].Action.ActionID)
}

func TestAdvanceTransitionsThroughSimplifiedLODAndReconstructs(t *testing.T) {
	agent := testAgent(c("HUNGER", -80))
	profile := testTickProfile()
	profile.Tuning.FullTickHours = .5
	profile.Tuning.SimplifiedTickHours = 2
	profile.AggregatedEventEffects = []AggregatedEventEffect{{Kind: "meal", Deltas: map[string]float64{"HUNGER": 120}}}

	simplified := Advance(&agent, profile, nil, TickRequest{
		NowHours:        12,
		TargetLevel:     Simplified,
		AggregatedEvent: &AggregatedEvent{Kind: "meal"},
	}, rand.New(rand.NewPCG(1, 2)))

	assert.Equal(t, Simplified, simplified.SimulationLevel)
	assert.InDelta(t, 2, simplified.ElapsedHours, 0.0001)
	require.Len(t, agent.AggregatedEvents, 1)
	assert.InDelta(t, 12, agent.AggregatedEvents[0].AtHour, 0.0001)

	full := Advance(&agent, profile, nil, TickRequest{NowHours: 13, TargetLevel: Full}, rand.New(rand.NewPCG(1, 2)))

	assert.Equal(t, Full, full.SimulationLevel)
	assert.InDelta(t, .5, full.ElapsedHours, 0.0001)
	assert.InDelta(t, 40, agent.Considerations["HUNGER"].Value, 0.0001)
	assert.Empty(t, agent.AggregatedEvents)
}

func TestAdvanceExpiresNarrativeCommitmentsBeforeSelectingAnAction(t *testing.T) {
	agent := testAgent(c("FUN", -80))
	agent.NarrativeCommitments = []NarrativeCommitment{{
		ID:                "escort",
		ContradictoryTags: map[string]bool{"leave": true},
		ExpiresAt:         5,
	}}
	profile := testTickProfile()
	providers := []AffordanceProvider{provider("exit", Position{}, action("leave", map[string]float64{"FUN": 80}, "leave"))}

	Advance(&agent, profile, providers, TickRequest{NowHours: 4, TargetLevel: Full}, rand.New(rand.NewPCG(1, 2)))
	assert.Nil(t, agent.CurrentAction)

	Advance(&agent, profile, providers, TickRequest{NowHours: 5, TargetLevel: Full}, rand.New(rand.NewPCG(1, 2)))

	require.NotNil(t, agent.CurrentAction)
	assert.Equal(t, "leave", agent.CurrentAction.Action.ActionID)
	assert.Empty(t, agent.NarrativeCommitments)
}

func TestAdvanceDequeuesActionsInFIFOOrder(t *testing.T) {
	agent := testAgent(c("ENERGY", 0))
	first := ActionInstance{Action: action("first", map[string]float64{"ENERGY": 10})}
	first.Action.EstimatedDuration = 1
	second := ActionInstance{Action: action("second", map[string]float64{"ENERGY": 20})}
	second.Action.EstimatedDuration = 1
	EnqueueAction(&agent, first)
	EnqueueAction(&agent, second)
	profile := testTickProfile()
	profile.Tuning.FullTickHours = 1

	Advance(&agent, profile, nil, TickRequest{NowHours: 1, TargetLevel: Full}, rand.New(rand.NewPCG(1, 2)))
	require.NotNil(t, agent.CurrentAction)
	assert.Equal(t, "first", agent.CurrentAction.Action.ActionID)
	assert.True(t, agent.CurrentAction.PlayerQueued)
	require.Len(t, agent.ActionQueue, 1)

	Advance(&agent, profile, nil, TickRequest{NowHours: 2, TargetLevel: Full}, rand.New(rand.NewPCG(1, 2)))
	require.NotNil(t, agent.CurrentAction)
	assert.Equal(t, "second", agent.CurrentAction.Action.ActionID)
	assert.Empty(t, agent.ActionQueue)
	assert.InDelta(t, 10, agent.Considerations["ENERGY"].Value, 0.0001)

	Advance(&agent, profile, nil, TickRequest{NowHours: 3, TargetLevel: Full}, rand.New(rand.NewPCG(1, 2)))
	assert.Nil(t, agent.CurrentAction)
	assert.InDelta(t, 30, agent.Considerations["ENERGY"].Value, 0.0001)
}

func testTickProfile() Profile {
	return Profile{Tuning: DefaultTuning()}
}
