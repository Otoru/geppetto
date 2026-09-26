package service

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	engine "github.com/vitorhugo/geppetto"
	gepv1 "github.com/vitorhugo/geppetto/internal/gen/go/geppetto/v1"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// noSelection is the wire sentinel for "no action selected": the response
// uses a packed sint32 array with no optional presence, so -1 marks agents
// that exhausted every candidate.
const noSelection = -1

// Offset validation messages quote proto field names. They are derived from
// the generated struct tags so the messages cannot drift from the proto
// definition.
var (
	actionTagOffsetsFieldName   = protoFieldName(&gepv1.BatchDecideRequest{}, "ActionTagOffsets")
	actionDeltaOffsetsFieldName = protoFieldName(&gepv1.BatchDecideRequest{}, "ActionDeltaOffsets")
)

// protoFieldName resolves a generated field's wire name from its protobuf
// struct tag. It panics at package init if the field disappears, which turns
// a proto rename into a loud startup failure instead of a stale message.
func protoFieldName(message proto.Message, goFieldName string) string {
	field, ok := reflect.TypeOf(message).Elem().FieldByName(goFieldName)
	if !ok {
		panic(fmt.Sprintf("proto message %T has no field %s", message, goFieldName))
	}
	for _, part := range strings.Split(field.Tag.Get("protobuf"), ",") {
		if name, ok := strings.CutPrefix(part, "name="); ok {
			return name
		}
	}
	panic(fmt.Sprintf("proto field %s of %T has no wire name", goFieldName, message))
}

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
		response.SelectedActionIndices[agentIndex] = noSelection
	}
	return response
}

func (s *DecisionServer) decideBatch(ctx context.Context, request *gepv1.BatchDecideRequest, profile engine.Profile, providers []engine.AffordanceProvider, actions []engine.AdvertisedAction, response *gepv1.BatchDecideResponse) {
	agentCount := len(request.AgentIds)
	preferences := make([][]engine.Candidate, agentCount)
	decider := engine.NewDecider(engine.WithProfile(profile))

	// Parallel phase: each agent scores and ranks its candidates. Workers
	// write only to their own preferences slot; providers stay read-only.
	jobs := make(chan int)
	var workers sync.WaitGroup
	workerCount := min(s.workers, agentCount)
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for agentIndex := range jobs {
				if ctx.Err() != nil {
					continue
				}
				s.rankOneWithDecider(request, profile, providers, decider, agentIndex, preferences)
			}
		}()
	}

	for agentIndex := range agentCount {
		jobs <- agentIndex
	}
	close(jobs)
	workers.Wait()

	// Serial reconciliation: contested provider slots go to the nearest
	// agents; losers walk their own fallback lists. Agents that exhaust
	// their candidates keep the noSelection prefilled by newBatchResponse.
	agents := make([]engine.Agent, agentCount)
	for agentIndex := range agents {
		agents[agentIndex] = engine.Agent{
			ID:       request.AgentIds[agentIndex],
			Position: engine.Position{X: request.PositionsX[agentIndex], Y: request.PositionsY[agentIndex], Z: request.PositionsZ[agentIndex]},
		}
	}
	assignments := decider.Reconcile(agents, preferences)
	for agentIndex, selected := range assignments {
		if selected != nil {
			writeDecision(response, agentIndex, selected, actions, providers, request.ActionProviderIndices)
		}
	}
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

func (s *DecisionServer) rankOne(request *gepv1.BatchDecideRequest, profile engine.Profile, providers []engine.AffordanceProvider, agentIndex int, preferences [][]engine.Candidate) {
	s.rankOneWithDecider(request, profile, providers, engine.NewDecider(engine.WithProfile(profile)), agentIndex, preferences)
}

func (s *DecisionServer) rankOneWithDecider(request *gepv1.BatchDecideRequest, profile engine.Profile, providers []engine.AffordanceProvider, decider engine.Decider, agentIndex int, preferences [][]engine.Candidate) {
	agent := s.agents.Get().(*engine.Agent)
	populateAgent(agent, request, profile, agentIndex)

	preferences[agentIndex] = decider.Rank(*agent, providers, request.Seed+uint64(agentIndex))

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
	return noSelection
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
	if err := validateOffsets(request.ActionTagOffsets, len(request.ActionTags), actionCount, actionTagOffsetsFieldName); err != nil {
		return nil, err
	}
	if err := validateOffsets(request.ActionDeltaOffsets, len(request.DeltaConsiderationIds), actionCount, actionDeltaOffsetsFieldName); err != nil {
		return nil, err
	}
	if len(request.DeltaConsiderationIds) != len(request.DeltaValues) {
		return nil, fmt.Errorf("delta arrays must have equal lengths")
	}
	// action_capacities/action_occupancies are optional: an empty array reads
	// as all zeros (unlimited capacity, no occupancy), which is what every
	// pre-existing client sends.
	if len(request.ActionCapacities) != 0 && len(request.ActionCapacities) != actionCount {
		return nil, fmt.Errorf("action_capacities must match action_ids")
	}
	if len(request.ActionOccupancies) != 0 && len(request.ActionOccupancies) != actionCount {
		return nil, fmt.Errorf("action_occupancies must match action_ids")
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
		Capacity:            int(uint32At(request.ActionCapacities, actionIndex)),
		Occupancy:           int(uint32At(request.ActionOccupancies, actionIndex)),
	}
}

// uint32At reads index from a parallel array that may be omitted entirely.
func uint32At(array []uint32, index int) uint32 {
	if len(array) == 0 {
		return 0
	}
	return array[index]
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
