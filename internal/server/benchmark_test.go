package server

import (
	"testing"

	npcv1 "github.com/vitorhugo/npcai/gen/go/npcai/v1"
	"google.golang.org/protobuf/proto"
)

func benchmarkBatch() *npcv1.BatchDecideRequest {
	agents := 1_000
	request := &npcv1.BatchDecideRequest{
		ProfileId: "social-life", ProviderIds: []string{"bed"}, ProviderPositionsX: []float64{0}, ProviderPositionsY: []float64{0}, ProviderPositionsZ: []float64{0}, ProviderCapacities: []uint32{1_000},
		ActionProviderIndices: []uint32{0}, ActionIds: []string{"rest"}, ActionEstimatedDurations: []float64{1}, ActionDomains: []string{""}, ActionIntrinsicPriorities: []float64{0}, ActionAdvertisementRadii: []float64{10}, ActionTagOffsets: []uint32{0, 0}, ActionDeltaOffsets: []uint32{0, 1}, DeltaConsiderationIds: []string{"ENERGY"}, DeltaValues: []float64{80},
	}
	for index := 0; index < agents; index++ {
		request.AgentIds = append(request.AgentIds, "agent")
		request.PositionsX = append(request.PositionsX, 0)
		request.PositionsY = append(request.PositionsY, 0)
		request.PositionsZ = append(request.PositionsZ, 0)
		// social-life has three considerations, in profile declaration order.
		request.ConsiderationValues = append(request.ConsiderationValues, 0, -50, -50)
	}
	return request
}

func BenchmarkBatchProtoEncode(b *testing.B) {
	request := benchmarkBatch()
	b.ReportAllocs()
	for b.Loop() {
		_, _ = proto.Marshal(request)
	}
}

func BenchmarkBatchProtoDecode(b *testing.B) {
	payload, err := proto.Marshal(benchmarkBatch())
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		var request npcv1.BatchDecideRequest
		if err := proto.Unmarshal(payload, &request); err != nil {
			b.Fatal(err)
		}
	}
}
