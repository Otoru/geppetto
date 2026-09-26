package server

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gepv1 "github.com/vitorhugo/geppetto/gen/go/geppetto/v1"
	"github.com/vitorhugo/geppetto/internal/engine"
)

func contendedBedRequest(capacity uint32) *gepv1.BatchDecideRequest {
	return &gepv1.BatchDecideRequest{
		ProfileId:                 "test",
		AgentIds:                  []string{"far", "near"},
		PositionsX:                []float64{9, 1},
		PositionsY:                []float64{0, 0},
		PositionsZ:                []float64{0, 0},
		ConsiderationValues:       []float64{-50, -50},
		ProviderIds:               []string{"bed"},
		ProviderPositionsX:        []float64{0},
		ProviderPositionsY:        []float64{0},
		ProviderPositionsZ:        []float64{0},
		ProviderCapacities:        []uint32{capacity},
		ActionProviderIndices:     []uint32{0},
		ActionIds:                 []string{"rest"},
		ActionEstimatedDurations:  []float64{1},
		ActionDomains:             []string{""},
		ActionIntrinsicPriorities: []float64{0},
		ActionAdvertisementRadii:  []float64{100},
		ActionTagOffsets:          []uint32{0, 0},
		ActionDeltaOffsets:        []uint32{0, 1},
		DeltaConsiderationIds:     []string{"ENERGY"},
		DeltaValues:               []float64{80},
		Seed:                      7,
	}
}

func contendedBedService() *DecisionServer {
	profile := engine.Profile{
		Name: "test",
		Considerations: []engine.Consideration{{
			ID: "ENERGY", Value: 100, Min: -100, Max: 100, BaseWeight: 1,
			ResponseCurve: engine.ResponseCurve{Kind: engine.Convex, Exponent: 2},
		}},
		Tuning: engine.DefaultTuning(),
	}
	return NewDecisionServer(NewProfileCache(map[string]engine.Profile{"test": profile}))
}

func TestBatchDecide_ContendedProviderGoesToNearestAgent(t *testing.T) {
	service := contendedBedService()

	response, err := service.BatchDecide(context.Background(), contendedBedRequest(1))

	require.NoError(t, err)
	assert.Equal(t, int32(-1), response.SelectedActionIndices[0], "far agent must lose the single slot")
	assert.Empty(t, response.ProviderIds[0])
	assert.Equal(t, int32(0), response.SelectedActionIndices[1], "near agent wins the slot")
	assert.Equal(t, "bed", response.ProviderIds[1])
}

func TestBatchDecide_CapacityTwoAdmitsBothAgents(t *testing.T) {
	service := contendedBedService()

	response, err := service.BatchDecide(context.Background(), contendedBedRequest(2))

	require.NoError(t, err)
	for agentIndex := range response.SelectedActionIndices {
		assert.Equal(t, int32(0), response.SelectedActionIndices[agentIndex])
		assert.Equal(t, "bed", response.ProviderIds[agentIndex])
	}
}
