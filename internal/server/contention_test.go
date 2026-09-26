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

func TestCA15_ContendedProviderGoesToNearestAgent(t *testing.T) {
	service := contendedBedService()

	response, err := service.BatchDecide(context.Background(), contendedBedRequest(1))

	require.NoError(t, err)
	assert.Equal(t, int32(-1), response.SelectedActionIndices[0], "far agent must lose the single slot")
	assert.Empty(t, response.ProviderIds[0])
	assert.Equal(t, int32(0), response.SelectedActionIndices[1], "near agent wins the slot")
	assert.Equal(t, "bed", response.ProviderIds[1])
}

func TestCA17_CapacityTwoAdmitsBothAgents(t *testing.T) {
	service := contendedBedService()

	response, err := service.BatchDecide(context.Background(), contendedBedRequest(2))

	require.NoError(t, err)
	for agentIndex := range response.SelectedActionIndices {
		assert.Equal(t, int32(0), response.SelectedActionIndices[agentIndex])
		assert.Equal(t, "bed", response.ProviderIds[agentIndex])
	}
}

// workbenchRequest builds the bancada scenario: one provider with capacity 4
// advertising "saw" (action capacity 1) and "hammer" (no own limit), disputed
// by 5 agents queued nearest-first.
func workbenchRequest() *gepv1.BatchDecideRequest {
	request := &gepv1.BatchDecideRequest{
		ProfileId:                 "test",
		ProviderIds:               []string{"bench"},
		ProviderPositionsX:        []float64{0},
		ProviderPositionsY:        []float64{0},
		ProviderPositionsZ:        []float64{0},
		ProviderCapacities:        []uint32{4},
		ActionProviderIndices:     []uint32{0, 0},
		ActionIds:                 []string{"saw", "hammer"},
		ActionEstimatedDurations:  []float64{1, 1},
		ActionDomains:             []string{"", ""},
		ActionIntrinsicPriorities: []float64{0, 0},
		ActionAdvertisementRadii:  []float64{100, 100},
		ActionCapacities:          []uint32{1, 0},
		ActionTagOffsets:          []uint32{0, 0, 0},
		ActionDeltaOffsets:        []uint32{0, 1, 2},
		DeltaConsiderationIds:     []string{"ENERGY", "ENERGY"},
		DeltaValues:               []float64{80, 40},
		Seed:                      7,
	}
	for agentIndex := 0; agentIndex < 5; agentIndex++ {
		request.AgentIds = append(request.AgentIds, string(rune('a'+agentIndex)))
		request.PositionsX = append(request.PositionsX, float64(agentIndex+1))
		request.PositionsY = append(request.PositionsY, 0)
		request.PositionsZ = append(request.PositionsZ, 0)
		request.ConsiderationValues = append(request.ConsiderationValues, -50)
	}
	return request
}

func TestCA22_ActionCapacityLimitsPerAction(t *testing.T) {
	service := contendedBedService()

	response, err := service.BatchDecide(context.Background(), workbenchRequest())

	require.NoError(t, err)
	saw, assigned := 0, 0
	for agentIndex := range response.SelectedActionIndices {
		if response.SelectedActionIndices[agentIndex] < 0 {
			continue
		}
		assigned++
		if response.ActionIds[agentIndex] == "saw" {
			saw++
		}
	}
	assert.Equal(t, 1, saw, "exactly one agent may operate the saw")
	assert.Equal(t, 4, assigned, "provider capacity 4 admits the 4 nearest agents")
	assert.Equal(t, int32(-1), response.SelectedActionIndices[4], "the farthest agent is left out")
	assert.Equal(t, "saw", response.ActionIds[0], "the nearest agent operates the saw")
}

func TestCA23_OmittedActionCapacitiesMeansUnlimited(t *testing.T) {
	service := contendedBedService()
	request := workbenchRequest()
	request.ActionCapacities = nil // old client: field absent
	request.ProviderCapacities = []uint32{5}

	response, err := service.BatchDecide(context.Background(), request)

	require.NoError(t, err)
	saw := 0
	for agentIndex := range response.SelectedActionIndices {
		assert.GreaterOrEqual(t, response.SelectedActionIndices[agentIndex], int32(0))
		if response.ActionIds[agentIndex] == "saw" {
			saw++
		}
	}
	assert.Equal(t, 5, saw, "absent action capacities must read as unlimited, never as blocked")
}
