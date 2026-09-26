package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	engine "github.com/vitorhugo/geppetto"
	gepv1 "github.com/vitorhugo/geppetto/internal/gen/go/geppetto/v1"
	"google.golang.org/protobuf/proto"
)

func TestBatchTickReturnsOnlyStateDeltas(t *testing.T) {
	service := tickService()
	request := tickRequest([]string{"a"}, []float64{-80})
	request.CurrentActionIndices = []int32{-1}

	response, err := service.BatchTick(context.Background(), request)

	require.NoError(t, err)
	assert.Equal(t, []uint32{0}, response.ActionChangeAgentIndices)
	assert.Equal(t, []int32{1}, response.CurrentActionIndices)
	assert.Empty(t, response.ConsiderationChangeAgentIndices)
	assert.Empty(t, response.QueueChangeAgentIndices)
}

func TestBatchTickSupportsMixedLevelsQueueAndCommitmentExpiry(t *testing.T) {
	service := tickService()
	request := tickRequest([]string{"queued", "expired", "lod"}, []float64{0, -80, -80})
	request.SimulationLevels = []gepv1.SimulationLevel{gepv1.SimulationLevel_SIMULATION_LEVEL_FULL, gepv1.SimulationLevel_SIMULATION_LEVEL_FULL, gepv1.SimulationLevel_SIMULATION_LEVEL_SIMPLIFIED}
	request.TargetSimulationLevels = []gepv1.SimulationLevel{gepv1.SimulationLevel_SIMULATION_LEVEL_FULL, gepv1.SimulationLevel_SIMULATION_LEVEL_FULL, gepv1.SimulationLevel_SIMULATION_LEVEL_FULL}
	request.QueueActionOffsets = []uint32{0, 1, 1, 1}
	request.QueueActionIndices = []uint32{0}
	request.CommitmentOffsets = []uint32{0, 0, 1, 1}
	request.CommitmentIds = []string{"escort"}
	request.CommitmentExpiresAt = []float64{5}
	request.CommitmentTagOffsets = []uint32{0, 1}
	request.CommitmentContradictoryTags = []string{"leave"}
	request.AggregatedEventOffsets = []uint32{0, 0, 0, 1}
	request.AggregatedEventKinds = []string{"meal"}
	request.AggregatedEventHours = []float64{4}
	request.NowHours = 5

	response, err := service.BatchTick(context.Background(), request)

	require.NoError(t, err)
	assert.Equal(t, []uint32{0, 1, 2}, response.ActionChangeAgentIndices)
	assert.Equal(t, []int32{0, 1, 1}, response.CurrentActionIndices)
	assert.Equal(t, []uint32{0}, response.QueueChangeAgentIndices)
	assert.Equal(t, []uint32{1}, response.CommitmentChangeAgentIndices)
	assert.Equal(t, []uint32{2}, response.SimulationLevelChangeAgentIndices)
	assert.Equal(t, []uint32{2}, response.AggregatedEventClearAgentIndices)
	assert.Equal(t, []uint32{2}, response.ConsiderationChangeAgentIndices)
	assert.Equal(t, []float64{40}, response.ChangedConsiderationValues)
}

func TestBatchTickRejectsMalformedArrays(t *testing.T) {
	service := tickService()
	for _, request := range []*gepv1.BatchTickRequest{
		func() *gepv1.BatchTickRequest {
			request := tickRequest([]string{"a"}, []float64{0})
			request.CurrentActionIndices = nil
			return request
		}(),
		func() *gepv1.BatchTickRequest {
			request := tickRequest([]string{"a"}, []float64{0})
			request.QueueActionOffsets = []uint32{0, 2}
			return request
		}(),
		func() *gepv1.BatchTickRequest {
			request := tickRequest([]string{"a"}, []float64{0})
			request.CommitmentOffsets = []uint32{0, 1}
			request.CommitmentIds = []string{"x"}
			request.CommitmentExpiresAt = nil
			return request
		}(),
		func() *gepv1.BatchTickRequest {
			request := tickRequest([]string{"a"}, []float64{0})
			request.PerceptionOffsets = []uint32{0, 1}
			request.PerceptionIds = []string{"x"}
			return request
		}(),
	} {
		_, err := service.BatchTick(context.Background(), request)
		require.Error(t, err)
	}
}

func TestBatchTickSparseResponseDoesNotEchoUnchangedState(t *testing.T) {
	service := tickService()
	ids := make([]string, 1_000)
	values := make([]float64, 1_000)
	for index := range ids {
		ids[index] = "agent"
	}
	request := tickRequest(ids, values)
	request.SimulationLevels = make([]gepv1.SimulationLevel, len(ids))
	request.TargetSimulationLevels = make([]gepv1.SimulationLevel, len(ids))
	for index := range ids {
		request.SimulationLevels[index] = gepv1.SimulationLevel_SIMULATION_LEVEL_SIMPLIFIED
		request.TargetSimulationLevels[index] = gepv1.SimulationLevel_SIMULATION_LEVEL_SIMPLIFIED
	}

	response, err := service.BatchTick(context.Background(), request)

	require.NoError(t, err)
	t.Logf("sparse tick: request=%d bytes response=%d bytes", proto.Size(request), proto.Size(response))
	assert.Less(t, proto.Size(response), proto.Size(request))
	assert.Empty(t, response.ConsiderationChangeAgentIndices)
	assert.Empty(t, response.ActionChangeAgentIndices)
}

func tickService() *DecisionServer {
	profile := engine.Profile{Name: "tick", Considerations: []engine.Consideration{{ID: "HUNGER", Min: -100, Max: 100, BaseWeight: 1, CriticalThreshold: -50, ResponseCurve: engine.ResponseCurve{Kind: engine.Convex, Exponent: 2}}}, Tuning: engine.DefaultTuning(), AggregatedEventEffects: []engine.AggregatedEventEffect{{Kind: "meal", Deltas: map[string]float64{"HUNGER": 120}}}}
	return NewDecisionServer(NewProfileCache(map[string]engine.Profile{"tick": profile}))
}

func tickRequest(ids []string, values []float64) *gepv1.BatchTickRequest {
	batch := &gepv1.BatchDecideRequest{ProfileId: "tick", AgentIds: ids, ConsiderationValues: values, ProviderIds: []string{"post", "exit"}, ProviderPositionsX: []float64{0, 0}, ProviderPositionsY: []float64{0, 0}, ProviderPositionsZ: []float64{0, 0}, ProviderCapacities: []uint32{10, 10}, ActionProviderIndices: []uint32{0, 1}, ActionIds: []string{"wait", "leave"}, ActionEstimatedDurations: []float64{1, 1}, ActionDomains: []string{"", ""}, ActionIntrinsicPriorities: []float64{0, 0}, ActionAdvertisementRadii: []float64{100, 100}, ActionTagOffsets: []uint32{0, 0, 1}, ActionTags: []string{"leave"}, ActionDeltaOffsets: []uint32{0, 0, 1}, DeltaConsiderationIds: []string{"HUNGER"}, DeltaValues: []float64{80}, Seed: 3}
	batch.PositionsX = make([]float64, len(ids))
	batch.PositionsY = make([]float64, len(ids))
	batch.PositionsZ = make([]float64, len(ids))
	return &gepv1.BatchTickRequest{Batch: batch, CurrentActionIndices: func() []int32 {
		values := make([]int32, len(ids))
		for index := range values {
			values[index] = -1
		}
		return values
	}()}
}
