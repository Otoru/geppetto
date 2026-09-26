package service

import (
	"testing"

	engine "github.com/Otoru/geppetto"
	"github.com/stretchr/testify/assert"
)

func TestBatchRequestBuilderMaintainsOffsets(t *testing.T) {
	builder := newBatchRequest("test", 7)
	provider := builder.AddProvider("bench", engine.Position{}, 1)
	builder.AddAction(provider, engine.AdvertisedAction{ActionID: "use", Tags: []string{"work"}, Deltas: map[string]float64{"ENERGY": 1}})
	builder.AddAgent("agent", engine.Position{}, []float64{-50})
	request := builder.Build()
	assert.Equal(t, []uint32{0, 1}, request.ActionTagOffsets)
	assert.Equal(t, []uint32{0, 1}, request.ActionDeltaOffsets)
	assert.NoError(t, validateOffsets(request.ActionTagOffsets, len(request.ActionTags), len(request.ActionIds), actionTagOffsetsFieldName))
	assert.NoError(t, validateOffsets(request.ActionDeltaOffsets, len(request.DeltaValues), len(request.ActionIds), actionDeltaOffsetsFieldName))
}
