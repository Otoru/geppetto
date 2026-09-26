// Package server adapts the wire-oriented protobuf contract to the engine. It
// does not retain NPC state between requests or implement engine scoring rules.
package server

import (
	"context"
	"fmt"
	"math/rand/v2"
	"runtime"
	"sync"
	"time"

	gepv1 "github.com/vitorhugo/geppetto/gen/go/geppetto/v1"
	"github.com/vitorhugo/geppetto/internal/engine"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

// ProfileCache holds profiles loaded once at process startup.
type ProfileCache struct {
	profiles map[string]engine.Profile
}

// NewProfileCache creates a cache over profiles indexed by their profile ID.
func NewProfileCache(profiles map[string]engine.Profile) *ProfileCache {
	return &ProfileCache{profiles: profiles}
}

// DecisionServer implements the stateless DecisionService RPCs.
type DecisionServer struct {
	gepv1.UnimplementedDecisionServiceServer
	profiles *ProfileCache
	workers  int
	agents   sync.Pool
	log      *zap.Logger
}

// NewDecisionServer creates a DecisionServer with a reusable Agent pool.
func NewDecisionServer(profiles *ProfileCache) *DecisionServer {
	return &DecisionServer{
		profiles: profiles,
		workers:  runtime.GOMAXPROCS(0),
		agents: sync.Pool{New: func() any {
			return &engine.Agent{}
		}},
		log: zap.NewNop(),
	}
}

// GRPCParams groups the dependencies of NewGRPCServer. Tests mount it by hand;
// no fx.App is required to call the constructor directly.
type GRPCParams struct {
	fx.In
	Log      *zap.Logger
	Profiles *ProfileCache
}

// GRPCResult groups the servers assembled by NewGRPCServer.
type GRPCResult struct {
	fx.Out
	Server *grpc.Server
	Health *health.Server
}

// NewGRPCServer assembles the gRPC server with the decision and health
// services registered.
func NewGRPCServer(p GRPCParams) GRPCResult {
	decision := NewDecisionServer(p.Profiles)
	decision.log = p.Log
	grpcServer := grpc.NewServer()
	gepv1.RegisterDecisionServiceServer(grpcServer, decision)
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	p.Log.Debug("grpc server assembled", zap.Int("workers", decision.workers))
	return GRPCResult{Server: grpcServer, Health: healthServer}
}

// BatchDecide scores every agent in a homogeneous request batch.
func (s *DecisionServer) BatchDecide(ctx context.Context, request *gepv1.BatchDecideRequest) (*gepv1.BatchDecideResponse, error) {
	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	profile, ok := s.profiles.profiles[request.ProfileId]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "unknown profile %q", request.ProfileId)
	}
	providers, actions, err := decodeProviders(request)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := validateAgents(request, len(profile.Considerations)); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	agentCount := len(request.AgentIds)
	response := newBatchResponse(agentCount)

	started := time.Now()
	s.decideBatch(ctx, request, profile, providers, actions, response)
	if err := ctx.Err(); err != nil {
		return nil, status.FromContextError(err).Err()
	}
	// One entry per batch, never per agent: per-agent logging would blow the
	// 50k+ agents/tick budget. Debug level keeps this off by default and the
	// production sampler caps its cost when enabled.
	s.log.Debug("batch decided", zap.String("profile", request.ProfileId), zap.Int("agents", agentCount), zap.Duration("elapsed", time.Since(started)))
	return response, nil
}

func newBatchResponse(agentCount int) *gepv1.BatchDecideResponse {
	response := &gepv1.BatchDecideResponse{
		SelectedActionIndices: make([]int32, agentCount),
		ActionIds:             make([]string, agentCount),
		ProviderIds:           make([]string, agentCount),
		Utilities:             make([]float64, agentCount),
	}
	for agentIndex := range response.SelectedActionIndices {
		response.SelectedActionIndices[agentIndex] = -1
	}
	return response
}

func (s *DecisionServer) decideBatch(ctx context.Context, request *gepv1.BatchDecideRequest, profile engine.Profile, providers []engine.AffordanceProvider, actions []engine.AdvertisedAction, response *gepv1.BatchDecideResponse) {
	jobs := make(chan int)
	var workers sync.WaitGroup
	workerCount := min(s.workers, len(request.AgentIds))
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for agentIndex := range jobs {
				if ctx.Err() != nil {
					continue
				}
				s.decideOne(request, profile, providers, actions, agentIndex, response)
			}
		}()
	}

	for agentIndex := range len(request.AgentIds) {
		jobs <- agentIndex
	}
	close(jobs)
	workers.Wait()
}

// Decide scores exactly one agent through the BatchDecide implementation.
func (s *DecisionServer) Decide(ctx context.Context, request *gepv1.DecideRequest) (*gepv1.DecideResponse, error) {
	if request == nil || request.Batch == nil {
		return nil, status.Error(codes.InvalidArgument, "batch is required")
	}
	if len(request.Batch.AgentIds) != 1 {
		return nil, status.Error(codes.InvalidArgument, "Decide accepts exactly one agent")
	}
	batch, err := s.BatchDecide(ctx, request.Batch)
	if err != nil {
		return nil, err
	}
	return &gepv1.DecideResponse{
		SelectedActionIndex: batch.SelectedActionIndices[0],
		ActionId:            batch.ActionIds[0],
		ProviderId:          batch.ProviderIds[0],
		Utility:             batch.Utilities[0],
	}, nil
}

func (s *DecisionServer) decideOne(request *gepv1.BatchDecideRequest, profile engine.Profile, providers []engine.AffordanceProvider, actions []engine.AdvertisedAction, agentIndex int, response *gepv1.BatchDecideResponse) {
	agent := s.agents.Get().(*engine.Agent)
	populateAgent(agent, request, profile, agentIndex)

	rng := rand.New(rand.NewPCG(request.Seed+uint64(agentIndex), request.Seed+uint64(agentIndex)+1))
	selected := engine.SelectAction(*agent, providers, profile.Tuning, rng)
	if selected != nil {
		writeDecision(response, agentIndex, selected, actions, providers, request.ActionProviderIndices)
	}

	*agent = engine.Agent{}
	s.agents.Put(agent)
}

func populateAgent(agent *engine.Agent, request *gepv1.BatchDecideRequest, profile engine.Profile, agentIndex int) {
	agent.ID = request.AgentIds[agentIndex]
	agent.Position = engine.Position{
		X: request.PositionsX[agentIndex],
		Y: request.PositionsY[agentIndex],
		Z: request.PositionsZ[agentIndex],
	}
	agent.Considerations = considerationValuesForAgent(request, profile.Considerations, agentIndex)
}

func considerationValuesForAgent(request *gepv1.BatchDecideRequest, profileConsiderations []engine.Consideration, agentIndex int) map[string]engine.Consideration {
	considerations := make(map[string]engine.Consideration, len(profileConsiderations))
	valueOffset := agentIndex * len(profileConsiderations)
	for considerationOffset, consideration := range profileConsiderations {
		consideration.Value = request.ConsiderationValues[valueOffset+considerationOffset]
		considerations[consideration.ID] = consideration
	}
	return considerations
}

func writeDecision(response *gepv1.BatchDecideResponse, agentIndex int, selected *engine.ActionInstance, actions []engine.AdvertisedAction, providers []engine.AffordanceProvider, actionProviderIndices []uint32) {
	response.ActionIds[agentIndex] = selected.Action.ActionID
	response.ProviderIds[agentIndex] = selected.ProviderID
	response.Utilities[agentIndex] = selected.ContinuationUtility
	response.SelectedActionIndices[agentIndex] = selectedActionIndex(selected, actions, providers, actionProviderIndices)
}

func selectedActionIndex(selected *engine.ActionInstance, actions []engine.AdvertisedAction, providers []engine.AffordanceProvider, actionProviderIndices []uint32) int32 {
	for actionIndex, action := range actions {
		providerID := providers[int(actionProviderIndices[actionIndex])].ID
		if action.ActionID == selected.Action.ActionID && providerID == selected.ProviderID {
			return int32(actionIndex)
		}
	}
	return -1
}

func validateAgents(request *gepv1.BatchDecideRequest, considerationCount int) error {
	agentCount := len(request.AgentIds)
	if agentCount == 0 {
		return fmt.Errorf("at least one agent is required")
	}
	if len(request.PositionsX) != agentCount || len(request.PositionsY) != agentCount || len(request.PositionsZ) != agentCount {
		return fmt.Errorf("agent position arrays must match agent_ids")
	}
	if len(request.ConsiderationValues) != agentCount*considerationCount {
		return fmt.Errorf("consideration_values must contain one profile stride per agent")
	}
	return nil
}

func decodeProviders(request *gepv1.BatchDecideRequest) ([]engine.AffordanceProvider, []engine.AdvertisedAction, error) {
	providerCount := len(request.ProviderIds)
	if err := validateProviderArrays(request, providerCount); err != nil {
		return nil, nil, err
	}

	providers := make([]engine.AffordanceProvider, providerCount)
	for providerIndex := range providers {
		providers[providerIndex] = engine.AffordanceProvider{
			ID: request.ProviderIds[providerIndex],
			Position: engine.Position{
				X: request.ProviderPositionsX[providerIndex],
				Y: request.ProviderPositionsY[providerIndex],
				Z: request.ProviderPositionsZ[providerIndex],
			},
			Capacity: int(request.ProviderCapacities[providerIndex]),
		}
	}

	actions, err := decodeActions(request, providers)
	if err != nil {
		return nil, nil, err
	}
	return providers, actions, nil
}

func validateProviderArrays(request *gepv1.BatchDecideRequest, providerCount int) error {
	if len(request.ProviderPositionsX) != providerCount || len(request.ProviderPositionsY) != providerCount || len(request.ProviderPositionsZ) != providerCount || len(request.ProviderCapacities) != providerCount {
		return fmt.Errorf("provider arrays must match provider_ids")
	}
	return nil
}

func decodeActions(request *gepv1.BatchDecideRequest, providers []engine.AffordanceProvider) ([]engine.AdvertisedAction, error) {
	actionCount := len(request.ActionIds)
	if len(request.ActionProviderIndices) != actionCount || len(request.ActionEstimatedDurations) != actionCount || len(request.ActionDomains) != actionCount || len(request.ActionIntrinsicPriorities) != actionCount || len(request.ActionAdvertisementRadii) != actionCount {
		return nil, fmt.Errorf("action arrays must match action_ids")
	}
	if err := validateOffsets(request.ActionTagOffsets, len(request.ActionTags), actionCount, "action_tag_offsets"); err != nil {
		return nil, err
	}
	if err := validateOffsets(request.ActionDeltaOffsets, len(request.DeltaConsiderationIds), actionCount, "action_delta_offsets"); err != nil {
		return nil, err
	}
	if len(request.DeltaConsiderationIds) != len(request.DeltaValues) {
		return nil, fmt.Errorf("delta arrays must have equal lengths")
	}

	actions := make([]engine.AdvertisedAction, actionCount)
	for actionIndex := range actions {
		providerIndex := request.ActionProviderIndices[actionIndex]
		if providerIndex >= uint32(len(providers)) {
			return nil, fmt.Errorf("action %d references an unknown provider", actionIndex)
		}

		action := decodeAction(request, actionIndex)
		actions[actionIndex] = action
		providers[providerIndex].AdvertisedActions = append(providers[providerIndex].AdvertisedActions, action)
	}
	return actions, nil
}

func decodeAction(request *gepv1.BatchDecideRequest, actionIndex int) engine.AdvertisedAction {
	tagStart, tagEnd := request.ActionTagOffsets[actionIndex], request.ActionTagOffsets[actionIndex+1]
	deltaStart, deltaEnd := request.ActionDeltaOffsets[actionIndex], request.ActionDeltaOffsets[actionIndex+1]
	deltas := make(map[string]float64, deltaEnd-deltaStart)
	for deltaIndex := deltaStart; deltaIndex < deltaEnd; deltaIndex++ {
		deltas[request.DeltaConsiderationIds[deltaIndex]] = request.DeltaValues[deltaIndex]
	}
	return engine.AdvertisedAction{
		ActionID:            request.ActionIds[actionIndex],
		Deltas:              deltas,
		EstimatedDuration:   request.ActionEstimatedDurations[actionIndex],
		Tags:                request.ActionTags[tagStart:tagEnd],
		Domain:              request.ActionDomains[actionIndex],
		IntrinsicPriority:   request.ActionIntrinsicPriorities[actionIndex],
		AdvertisementRadius: request.ActionAdvertisementRadii[actionIndex],
	}
}

func validateOffsets(offsets []uint32, length, itemCount int, name string) error {
	if len(offsets) != itemCount+1 || len(offsets) == 0 || offsets[0] != 0 || offsets[len(offsets)-1] != uint32(length) {
		return fmt.Errorf("%s must delimit every action", name)
	}
	for index := 1; index < len(offsets); index++ {
		if offsets[index] < offsets[index-1] || offsets[index] > uint32(length) {
			return fmt.Errorf("%s must be monotonic and in range", name)
		}
	}
	return nil
}
