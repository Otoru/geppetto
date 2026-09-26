package geppetto

import (
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPerceptionNoiseZeroPreservesPerfectInformationSelection(t *testing.T) {
	perceived := c("THREAT", 50)
	perceived.Updater.Kind = PerceptionDriven
	agentWithPerception := testAgent(perceived, c("FUN", -30))
	agentWithPerfectInformation := testAgent(c("THREAT", 50), c("FUN", -30))
	providers := imperfectionProviders()
	tuning := DefaultTuning()
	tuning.PerceptionNoise = 0

	withPerception := SelectAction(agentWithPerception, providers, tuning, rand.New(rand.NewPCG(1, 2)))
	withPerfectInformation := SelectAction(agentWithPerfectInformation, providers, tuning, rand.New(rand.NewPCG(1, 2)))

	require.NotNil(t, withPerception)
	require.NotNil(t, withPerfectInformation)
	assert.Equal(t, withPerfectInformation, withPerception)
}

func TestPerceptionNoiseChangesSelectionReproduciblyWithoutChangingWorldState(t *testing.T) {
	threat := c("THREAT", 50)
	threat.Updater.Kind = PerceptionDriven
	agent := testAgent(threat, c("FUN", -30))
	providers := imperfectionProviders()
	perfectInformation := DefaultTuning()
	perfectInformation.SelectionTopK = 1
	noisyPerception := perfectInformation
	noisyPerception.PerceptionNoise = 30

	perfect := SelectAction(agent, providers, perfectInformation, rand.New(rand.NewPCG(8, 9)))
	firstNoisy := SelectAction(agent, providers, noisyPerception, rand.New(rand.NewPCG(8, 9)))
	secondNoisy := SelectAction(agent, providers, noisyPerception, rand.New(rand.NewPCG(8, 9)))

	require.NotNil(t, perfect)
	require.NotNil(t, firstNoisy)
	require.NotNil(t, secondNoisy)
	assert.NotEqual(t, perfect.Action.ActionID, firstNoisy.Action.ActionID)
	assert.Equal(t, firstNoisy, secondNoisy)
	assert.InDelta(t, 50, agent.Considerations["THREAT"].Value, 0.0001)
}

func TestConventionBreakProbabilityControlsContrarianChoices(t *testing.T) {
	agent := testAgent(c("DUTY", -50))
	providers := []AffordanceProvider{provider("post", Position{},
		action("follow-convention", map[string]float64{"DUTY": 80}),
		action("break-convention", map[string]float64{"DUTY": 20}),
	)}
	tuning := DefaultTuning()
	tuning.SelectionTopK = 1

	tuning.ConventionBreakProbability = 0
	for seed := uint64(0); seed < 1_000; seed++ {
		assert.Equal(t, "follow-convention", SelectAction(agent, providers, tuning, rand.New(rand.NewPCG(seed, seed+1))).Action.ActionID)
	}

	tuning.ConventionBreakProbability = 1
	for seed := uint64(0); seed < 1_000; seed++ {
		assert.Equal(t, "break-convention", SelectAction(agent, providers, tuning, rand.New(rand.NewPCG(seed, seed+1))).Action.ActionID)
	}

	tuning.ConventionBreakProbability = .25
	breaks := 0
	const samples = 10_000
	for seed := uint64(0); seed < samples; seed++ {
		chosen := SelectAction(agent, providers, tuning, rand.New(rand.NewPCG(seed, seed+1)))
		if chosen.Action.ActionID == "break-convention" {
			breaks++
		}
	}
	assert.InDelta(t, .25*samples, breaks, .02*samples)
}

func TestImperfectionsAreDeterministicWithTheSameAgentRNG(t *testing.T) {
	threat := c("THREAT", 50)
	threat.Updater.Kind = PerceptionDriven
	agent := testAgent(threat, c("FUN", -30))
	tuning := DefaultTuning()
	tuning.PerceptionNoise = 30
	tuning.ConventionBreakProbability = .4
	tuning.SelectionTopK = 1

	choices := func() []string {
		rng := rand.New(rand.NewPCG(71, 72))
		result := make([]string, 20)
		for index := range result {
			ranked := RankedPreferences(agent, imperfectionProviders(), tuning, rng)
			require.NotEmpty(t, ranked)
			result[index] = ranked[0].Action.ActionID
		}
		return result
	}

	assert.Equal(t, choices(), choices())
}

func imperfectionProviders() []AffordanceProvider {
	return []AffordanceProvider{
		provider("cover", Position{}, action("hide", map[string]float64{"THREAT": 80})),
		provider("route", Position{}, action("patrol", map[string]float64{"FUN": 25})),
	}
}
