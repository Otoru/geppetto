package server

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	npcv1 "github.com/vitorhugo/npcai/gen/go/npcai/v1"
	"github.com/vitorhugo/npcai/internal/engine"
)

func TestBatchDecide_SoABatchUsesCachedProfile(t *testing.T) {
	profile := engine.Profile{
		Name: "test",
		Considerations: []engine.Consideration{{
			ID: "ENERGY", Value: 100, Min: -100, Max: 100, BaseWeight: 1,
			ResponseCurve: engine.ResponseCurve{Kind: engine.Convex, Exponent: 2},
		}},
		Tuning: engine.DefaultTuning(),
	}
	service := NewDecisionServer(NewProfileCache(map[string]engine.Profile{"test": profile}))
	response, err := service.BatchDecide(context.Background(), &npcv1.BatchDecideRequest{
		ProfileId:                 "test",
		AgentIds:                  []string{"a", "b"},
		PositionsX:                []float64{0, 0},
		PositionsY:                []float64{0, 0},
		PositionsZ:                []float64{0, 0},
		ConsiderationValues:       []float64{-90, 80},
		ProviderIds:               []string{"bed", "game"},
		ProviderPositionsX:        []float64{0, 0},
		ProviderPositionsY:        []float64{0, 0},
		ProviderPositionsZ:        []float64{0, 0},
		ProviderCapacities:        []uint32{2, 2},
		ActionProviderIndices:     []uint32{0, 1},
		ActionIds:                 []string{"rest", "play"},
		ActionEstimatedDurations:  []float64{1, 1},
		ActionDomains:             []string{"", ""},
		ActionIntrinsicPriorities: []float64{0, 1},
		ActionAdvertisementRadii:  []float64{10, 10},
		ActionTagOffsets:          []uint32{0, 0, 0},
		ActionDeltaOffsets:        []uint32{0, 1, 2},
		DeltaConsiderationIds:     []string{"ENERGY", "ENERGY"},
		DeltaValues:               []float64{80, 1},
		Seed:                      7,
	})
	require.NoError(t, err)
	require.Len(t, response.SelectedActionIndices, 2)
	assert.Equal(t, int32(0), response.SelectedActionIndices[0])
	assert.Equal(t, "rest", response.ActionIds[0])
	assert.Equal(t, "bed", response.ProviderIds[0])
	assert.InDelta(t, 72.2, response.Utilities[0], 0.0001)
}

func TestBatchDecide_RejectsMalformedParallelArrays(t *testing.T) {
	service := NewDecisionServer(NewProfileCache(map[string]engine.Profile{"test": {Name: "test"}}))
	_, err := service.BatchDecide(context.Background(), &npcv1.BatchDecideRequest{ProfileId: "test", AgentIds: []string{"a"}})
	require.Error(t, err)
}
