package service

import (
	"context"
	"testing"

	engine "github.com/Otoru/geppetto"
	gepv1 "github.com/Otoru/geppetto/internal/gen/go/geppetto/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRankOneDerivesPerAgentRNGFromBatchSeedAndIndex(t *testing.T) {
	request := &gepv1.BatchDecideRequest{
		Seed:       41,
		AgentIds:   []string{"agent-0", "agent-1", "agent-2", "agent-3", "agent-4", "agent-5", "agent-6", "agent-7", "agent-8", "agent-9", "agent-10", "agent-11"},
		PositionsX: make([]float64, 12),
		PositionsY: make([]float64, 12),
		PositionsZ: make([]float64, 12),
	}
	providers := []engine.AffordanceProvider{{
		ID:       "choice-table",
		Capacity: 12,
		AdvertisedActions: []engine.AdvertisedAction{
			{ActionID: "choice-0", AdvertisementRadius: 1},
			{ActionID: "choice-1", AdvertisementRadius: 1},
			{ActionID: "choice-2", AdvertisementRadius: 1},
			{ActionID: "choice-3", AdvertisementRadius: 1},
			{ActionID: "choice-4", AdvertisementRadius: 1},
			{ActionID: "choice-5", AdvertisementRadius: 1},
			{ActionID: "choice-6", AdvertisementRadius: 1},
			{ActionID: "choice-7", AdvertisementRadius: 1},
		},
	}}
	tuning := engine.DefaultTuning()
	tuning.SelectionTopK = 8
	tuning.ReconciliationTopK = 8
	profile := engine.Profile{Tuning: tuning}
	server := NewDecisionServer(NewProfileCache(nil))

	choices := func() []string {
		preferences := make([][]engine.Candidate, len(request.AgentIds))
		for agentIndex := range request.AgentIds {
			server.rankOne(request, profile, providers, agentIndex, preferences)
		}
		result := make([]string, len(preferences))
		for agentIndex, ranked := range preferences {
			result[agentIndex] = ranked[0].Action.ActionID
		}
		return result
	}

	first := choices()
	second := choices()
	require.Equal(t, first, second, "the same batch seed and agent indices must reproduce the RNG sequence")
	assert.Equal(t, []string{
		"choice-3", "choice-3", "choice-5", "choice-2", "choice-3", "choice-2",
		"choice-5", "choice-0", "choice-1", "choice-6", "choice-0", "choice-7",
	}, first, "agent-indexed choices freeze the server's PCG seed derivation")
}

func TestBatchDecideGoldenRepresentativeContention(t *testing.T) {
	profile := engine.Profile{
		Name: "golden",
		Considerations: []engine.Consideration{
			{ID: "HUNGER", Min: -100, Max: 100, BaseWeight: 1, ResponseCurve: engine.ResponseCurve{Kind: engine.Convex, Exponent: 2}},
			{ID: "ENERGY", Min: -100, Max: 100, BaseWeight: 1, ResponseCurve: engine.ResponseCurve{Kind: engine.Convex, Exponent: 2}},
			{ID: "FUN", Min: -100, Max: 100, BaseWeight: 1, ResponseCurve: engine.ResponseCurve{Kind: engine.Convex, Exponent: 2}},
		},
		Tuning: engine.DefaultTuning(),
	}
	service := NewDecisionServer(NewProfileCache(map[string]engine.Profile{"golden": profile}))
	request := &gepv1.BatchDecideRequest{
		ProfileId:                 "golden",
		Seed:                      17,
		AgentIds:                  []string{"alix", "bea", "cy", "drew", "em", "finn"},
		PositionsX:                []float64{0, 1, 5, 6, 10, 9},
		PositionsY:                make([]float64, 6),
		PositionsZ:                make([]float64, 6),
		ConsiderationValues:       []float64{-80, -10, -10, -75, -20, -20, -10, -85, -25, -10, -80, -30, -20, -20, -90, -30, -40, -75},
		ProviderIds:               []string{"kitchen", "workshop", "arcade"},
		ProviderPositionsX:        []float64{0, 5, 10},
		ProviderPositionsY:        []float64{0, 0, 0},
		ProviderPositionsZ:        []float64{0, 0, 0},
		ProviderCapacities:        []uint32{1, 3, 2},
		ActionProviderIndices:     []uint32{0, 0, 1, 1, 2, 2},
		ActionIds:                 []string{"meal", "snack", "repair", "stretch", "play", "socialize"},
		ActionEstimatedDurations:  []float64{1, 1, 1, 1, 1, 1},
		ActionDomains:             []string{"", "", "", "", "", ""},
		ActionIntrinsicPriorities: []float64{0, 6, 0, 3, 0, 2},
		ActionAdvertisementRadii:  []float64{100, 100, 100, 100, 100, 100},
		ActionCapacities:          []uint32{0, 0, 1, 0, 0, 0},
		ActionTagOffsets:          []uint32{0, 0, 0, 0, 0, 0, 0},
		ActionDeltaOffsets:        []uint32{0, 1, 2, 3, 4, 5, 6},
		DeltaConsiderationIds:     []string{"HUNGER", "HUNGER", "ENERGY", "ENERGY", "FUN", "FUN"},
		DeltaValues:               []float64{80, 35, 80, 35, 80, 40},
	}

	response, err := service.BatchDecide(context.Background(), request)
	require.NoError(t, err)
	got := decisionsByAgent(request.AgentIds, response)
	want := []goldenDecision{
		{AgentID: "alix", ActionID: "meal", ProviderID: "kitchen"},
		{AgentID: "bea", ActionID: "stretch", ProviderID: "workshop"},
		{AgentID: "cy", ActionID: "repair", ProviderID: "workshop"},
		{AgentID: "drew", ActionID: "stretch", ProviderID: "workshop"},
		{AgentID: "em", ActionID: "play", ProviderID: "arcade"},
		{AgentID: "finn", ActionID: "play", ProviderID: "arcade"},
	}

	assert.Equal(t, want, got)
}

type goldenDecision struct {
	AgentID    string
	ActionID   string
	ProviderID string
}

func decisionsByAgent(agentIDs []string, response *gepv1.BatchDecideResponse) []goldenDecision {
	decisions := make([]goldenDecision, len(agentIDs))
	for agentIndex, agentID := range agentIDs {
		decisions[agentIndex] = goldenDecision{
			AgentID:    agentID,
			ActionID:   response.ActionIds[agentIndex],
			ProviderID: response.ProviderIds[agentIndex],
		}
	}
	return decisions
}
