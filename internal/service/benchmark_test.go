package service

import (
	"context"
	"testing"

	engine "github.com/Otoru/geppetto"
	gepv1 "github.com/Otoru/geppetto/internal/gen/go/geppetto/v1"
	"google.golang.org/protobuf/proto"
)

func benchmarkBatch() *gepv1.BatchDecideRequest {
	agents := 1_000
	request := &gepv1.BatchDecideRequest{
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
		var request gepv1.BatchDecideRequest
		if err := proto.Unmarshal(payload, &request); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkService() *DecisionServer {
	profile := engine.Profile{Name: "social-life", Tuning: engine.DefaultTuning()}
	for _, id := range []string{"HUNGER", "ENERGY", "FUN"} {
		profile.Considerations = append(profile.Considerations, engine.Consideration{
			ID: id, Min: -100, Max: 100, BaseWeight: 1,
			ResponseCurve: engine.ResponseCurve{Kind: engine.Convex, Exponent: 2},
		})
	}
	return NewDecisionServer(NewProfileCache(map[string]engine.Profile{"social-life": profile}))
}

// BenchmarkBatchDecide measures the full in-process decision path: decode,
// parallel scoring, and assignment of 1.000 agents.
func BenchmarkBatchDecide(b *testing.B) {
	service := benchmarkService()
	request := benchmarkBatch()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := service.BatchDecide(context.Background(), request); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkBatchDecideContended stresses the reconciliation phase: 1.000
// agents dispute 400 slots spread over 4 providers, so losers must walk their
// fallback lists across several rounds.
func BenchmarkBatchDecideContended(b *testing.B) {
	service := benchmarkService()
	request := &gepv1.BatchDecideRequest{
		ProfileId: "social-life",
		Seed:      1,
	}
	for providerIndex := 0; providerIndex < 4; providerIndex++ {
		request.ProviderIds = append(request.ProviderIds, string(rune('a'+providerIndex)))
		request.ProviderPositionsX = append(request.ProviderPositionsX, float64(providerIndex*10))
		request.ProviderPositionsY = append(request.ProviderPositionsY, 0)
		request.ProviderPositionsZ = append(request.ProviderPositionsZ, 0)
		request.ProviderCapacities = append(request.ProviderCapacities, 100)
		request.ActionProviderIndices = append(request.ActionProviderIndices, uint32(providerIndex))
		request.ActionIds = append(request.ActionIds, "use")
		request.ActionEstimatedDurations = append(request.ActionEstimatedDurations, 1)
		request.ActionDomains = append(request.ActionDomains, "")
		request.ActionIntrinsicPriorities = append(request.ActionIntrinsicPriorities, 0)
		request.ActionAdvertisementRadii = append(request.ActionAdvertisementRadii, 1000)
		request.ActionTagOffsets = append(request.ActionTagOffsets, 0)
		request.ActionDeltaOffsets = append(request.ActionDeltaOffsets, uint32(providerIndex))
		request.DeltaConsiderationIds = append(request.DeltaConsiderationIds, "HUNGER")
		request.DeltaValues = append(request.DeltaValues, 80-float64(providerIndex*10))
	}
	request.ActionTagOffsets = append(request.ActionTagOffsets, 0)
	request.ActionDeltaOffsets = append(request.ActionDeltaOffsets, 4)
	for agentIndex := 0; agentIndex < 1_000; agentIndex++ {
		request.AgentIds = append(request.AgentIds, string(rune(agentIndex)))
		request.PositionsX = append(request.PositionsX, float64(agentIndex%40))
		request.PositionsY = append(request.PositionsY, 0)
		request.PositionsZ = append(request.PositionsZ, 0)
		request.ConsiderationValues = append(request.ConsiderationValues, -50, -50, -50)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := service.BatchDecide(context.Background(), request); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkTick() *gepv1.BatchTickRequest {
	batch := benchmarkBatch()
	batch.ProfileId = "social-life"
	batch.ActionIds = []string{"rest"}
	batch.ActionProviderIndices = []uint32{0}
	batch.ActionEstimatedDurations = []float64{1}
	batch.ActionDomains = []string{""}
	batch.ActionIntrinsicPriorities = []float64{0}
	batch.ActionAdvertisementRadii = []float64{10}
	batch.ActionTagOffsets = []uint32{0, 0}
	batch.ActionDeltaOffsets = []uint32{0, 1}
	batch.DeltaConsiderationIds = []string{"ENERGY"}
	batch.DeltaValues = []float64{80}
	current := make([]int32, len(batch.AgentIds))
	for index := range current {
		current[index] = -1
	}
	return &gepv1.BatchTickRequest{Batch: batch, CurrentActionIndices: current}
}

func BenchmarkBatchTickProtoEncode(b *testing.B) {
	request := benchmarkTick()
	b.ReportAllocs()
	for b.Loop() {
		_, _ = proto.Marshal(request)
	}
}

func BenchmarkBatchTickProtoDecode(b *testing.B) {
	payload, err := proto.Marshal(benchmarkTick())
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		var request gepv1.BatchTickRequest
		if err := proto.Unmarshal(payload, &request); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBatchTick(b *testing.B) {
	service := benchmarkService()
	request := benchmarkTick()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := service.BatchTick(context.Background(), request); err != nil {
			b.Fatal(err)
		}
	}
}
