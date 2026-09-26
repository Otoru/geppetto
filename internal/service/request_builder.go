package service

import (
	engine "github.com/Otoru/geppetto"
	gepv1 "github.com/Otoru/geppetto/internal/gen/go/geppetto/v1"
)

// batchRequestBuilder keeps the parallel protobuf arrays aligned while test
// fixtures and in-process clients build a BatchDecideRequest. It is internal:
// the generated protobuf message remains the transport API.
type batchRequestBuilder struct{ request gepv1.BatchDecideRequest }

func newBatchRequest(profileID string, seed uint64) *batchRequestBuilder {
	return &batchRequestBuilder{request: gepv1.BatchDecideRequest{ProfileId: profileID, Seed: seed, ActionTagOffsets: []uint32{0}, ActionDeltaOffsets: []uint32{0}}}
}

func (builder *batchRequestBuilder) AddProvider(id string, position engine.Position, capacity uint32) uint32 {
	index := uint32(len(builder.request.ProviderIds))
	builder.request.ProviderIds = append(builder.request.ProviderIds, id)
	builder.request.ProviderPositionsX = append(builder.request.ProviderPositionsX, position.X)
	builder.request.ProviderPositionsY = append(builder.request.ProviderPositionsY, position.Y)
	builder.request.ProviderPositionsZ = append(builder.request.ProviderPositionsZ, position.Z)
	builder.request.ProviderCapacities = append(builder.request.ProviderCapacities, capacity)
	return index
}

func (builder *batchRequestBuilder) AddAction(provider uint32, action engine.AdvertisedAction) {
	builder.request.ActionProviderIndices = append(builder.request.ActionProviderIndices, provider)
	builder.request.ActionIds = append(builder.request.ActionIds, action.ActionID)
	builder.request.ActionEstimatedDurations = append(builder.request.ActionEstimatedDurations, action.EstimatedDuration)
	builder.request.ActionDomains = append(builder.request.ActionDomains, action.Domain)
	builder.request.ActionIntrinsicPriorities = append(builder.request.ActionIntrinsicPriorities, action.IntrinsicPriority)
	builder.request.ActionAdvertisementRadii = append(builder.request.ActionAdvertisementRadii, action.AdvertisementRadius)
	builder.request.ActionCapacities = append(builder.request.ActionCapacities, uint32(action.Capacity))
	builder.request.ActionOccupancies = append(builder.request.ActionOccupancies, uint32(action.Occupancy))
	builder.request.ActionTags = append(builder.request.ActionTags, action.Tags...)
	builder.request.ActionTagOffsets = append(builder.request.ActionTagOffsets, uint32(len(builder.request.ActionTags)))
	for id, value := range action.Deltas {
		builder.request.DeltaConsiderationIds = append(builder.request.DeltaConsiderationIds, id)
		builder.request.DeltaValues = append(builder.request.DeltaValues, value)
	}
	builder.request.ActionDeltaOffsets = append(builder.request.ActionDeltaOffsets, uint32(len(builder.request.DeltaValues)))
}

func (builder *batchRequestBuilder) AddAgent(id string, position engine.Position, values []float64) {
	builder.request.AgentIds = append(builder.request.AgentIds, id)
	builder.request.PositionsX = append(builder.request.PositionsX, position.X)
	builder.request.PositionsY = append(builder.request.PositionsY, position.Y)
	builder.request.PositionsZ = append(builder.request.PositionsZ, position.Z)
	builder.request.ConsiderationValues = append(builder.request.ConsiderationValues, values...)
}

func (builder *batchRequestBuilder) Build() *gepv1.BatchDecideRequest { return &builder.request }
