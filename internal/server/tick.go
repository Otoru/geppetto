package server

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sort"

	gepv1 "github.com/vitorhugo/geppetto/gen/go/geppetto/v1"
	"github.com/vitorhugo/geppetto/internal/engine"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// BatchTick advances every supplied agent and returns only state mutations.
func (s *DecisionServer) BatchTick(ctx context.Context, request *gepv1.BatchTickRequest) (*gepv1.BatchTickResponse, error) {
	if request == nil || request.Batch == nil {
		return nil, status.Error(codes.InvalidArgument, "batch is required")
	}
	profile, ok := s.profiles.profiles[request.Batch.ProfileId]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "unknown profile %q", request.Batch.ProfileId)
	}
	providers, actions, err := decodeProviders(request.Batch)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := validateAgents(request.Batch, len(profile.Considerations)); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := validateTickRequest(request, len(request.Batch.AgentIds), len(actions)); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	agents := make([]engine.Agent, len(request.Batch.AgentIds))
	for index := range agents {
		if err := populateTickAgent(&agents[index], request, profile, actions, providers, index); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	}
	response := &gepv1.BatchTickResponse{ConsiderationChangeOffsets: []uint32{0}, QueueChangeOffsets: []uint32{0}, CommitmentChangeOffsets: []uint32{0}, CommitmentTagOffsets: []uint32{0}}
	for index := range agents {
		if ctx.Err() != nil {
			return nil, status.FromContextError(ctx.Err()).Err()
		}
		before := agents[index]
		rng := rand.New(rand.NewPCG(request.Batch.Seed+uint64(index), request.Batch.Seed+uint64(index)+1))
		result := engine.Advance(&agents[index], profile, providers, tickRequestAt(request, index), rng)
		appendTickDelta(response, request, profile, actions, before, agents[index], result, index)
	}
	return response, nil
}

func validateTickRequest(request *gepv1.BatchTickRequest, agents, actions int) error {
	if len(request.CurrentActionIndices) != agents {
		return fmt.Errorf("current_action_indices must match batch.agent_ids")
	}
	for _, values := range [][]float64{request.CurrentActionElapsed, request.CurrentActionUtilities} {
		if len(values) != 0 && len(values) != agents {
			return fmt.Errorf("current action arrays must match batch.agent_ids")
		}
	}
	if len(request.CurrentActionPlayerQueued) != 0 && len(request.CurrentActionPlayerQueued) != agents {
		return fmt.Errorf("current_action_player_queued must match batch.agent_ids")
	}
	for _, values := range [][]gepv1.SimulationLevel{request.SimulationLevels, request.TargetSimulationLevels} {
		if len(values) != 0 && len(values) != agents {
			return fmt.Errorf("simulation level arrays must match batch.agent_ids")
		}
	}
	for index, action := range request.CurrentActionIndices {
		if action < -1 || int(action) >= actions {
			return fmt.Errorf("current action %d references an unknown action", index)
		}
	}
	if err := validateTickOffsets(request.QueueActionOffsets, len(request.QueueActionIndices), agents, "queue_action_offsets"); err != nil {
		return err
	}
	if len(request.QueueActionElapsed) != 0 && len(request.QueueActionElapsed) != len(request.QueueActionIndices) {
		return fmt.Errorf("queue action arrays must match queue_action_indices")
	}
	if len(request.QueueActionUtilities) != 0 && len(request.QueueActionUtilities) != len(request.QueueActionIndices) {
		return fmt.Errorf("queue action arrays must match queue_action_indices")
	}
	for _, action := range request.QueueActionIndices {
		if int(action) >= actions {
			return fmt.Errorf("queued action references an unknown action")
		}
	}
	if err := validateTickOffsets(request.CommitmentOffsets, len(request.CommitmentIds), agents, "commitment_offsets"); err != nil {
		return err
	}
	if len(request.CommitmentExpiresAt) != len(request.CommitmentIds) {
		return fmt.Errorf("commitment_expires_at must match commitment_ids")
	}
	if len(request.CommitmentIds) != 0 && len(request.CommitmentTagOffsets) == 0 {
		return fmt.Errorf("commitment_tag_offsets is required when commitments are present")
	}
	if len(request.CommitmentTagOffsets) != 0 && validateOffsets(request.CommitmentTagOffsets, len(request.CommitmentContradictoryTags), len(request.CommitmentIds), "commitment_tag_offsets") != nil {
		return fmt.Errorf("commitment_tag_offsets must delimit every commitment")
	}
	if err := validateTickOffsets(request.AggregatedEventOffsets, len(request.AggregatedEventKinds), agents, "aggregated_event_offsets"); err != nil {
		return err
	}
	if len(request.AggregatedEventHours) != len(request.AggregatedEventKinds) {
		return fmt.Errorf("aggregated_event_hours must match aggregated_event_kinds")
	}
	if len(request.NewAggregatedEventKinds) != 0 && len(request.NewAggregatedEventKinds) != agents {
		return fmt.Errorf("new_aggregated_event_kinds must match batch.agent_ids")
	}
	for _, world := range []struct {
		offsets []uint32
		ids     []string
		values  []float64
		name    string
	}{{request.PerceptionOffsets, request.PerceptionIds, request.PerceptionValues, "perception"}, {request.RelationshipOffsets, request.RelationshipIds, request.RelationshipValues, "relationship"}, {request.ContextOffsets, request.ContextIds, request.ContextValues, "context"}} {
		if err := validateTickOffsets(world.offsets, len(world.ids), agents, world.name+"_offsets"); err != nil {
			return err
		}
		if len(world.values) != len(world.ids) {
			return fmt.Errorf("%s values must match ids", world.name)
		}
	}
	return nil
}

func validateTickOffsets(offsets []uint32, length, agents int, name string) error {
	if len(offsets) == 0 {
		if length != 0 {
			return fmt.Errorf("%s is required when entries are present", name)
		}
		return nil
	}
	return validateOffsets(offsets, length, agents, name)
}

func populateTickAgent(agent *engine.Agent, request *gepv1.BatchTickRequest, profile engine.Profile, actions []engine.AdvertisedAction, providers []engine.AffordanceProvider, index int) error {
	populateAgent(agent, request.Batch, profile, index)
	agent.SimulationLevel = engine.Full
	if len(request.SimulationLevels) > 0 {
		agent.SimulationLevel = fromWireLevel(request.SimulationLevels[index])
	}
	if action := request.CurrentActionIndices[index]; action >= 0 {
		instance := actionInstance(actions, providers, request.Batch.ActionProviderIndices, int(action))
		instance.Elapsed = floatAt(request.CurrentActionElapsed, index)
		instance.ContinuationUtility = floatAt(request.CurrentActionUtilities, index)
		instance.PlayerQueued = boolAt(request.CurrentActionPlayerQueued, index)
		agent.CurrentAction = &instance
	}
	if len(request.QueueActionOffsets) > 0 {
		for offset := request.QueueActionOffsets[index]; offset < request.QueueActionOffsets[index+1]; offset++ {
			action := int(request.QueueActionIndices[offset])
			instance := actionInstance(actions, providers, request.Batch.ActionProviderIndices, action)
			instance.Elapsed = floatAt(request.QueueActionElapsed, int(offset))
			instance.ContinuationUtility = floatAt(request.QueueActionUtilities, int(offset))
			engine.EnqueueAction(agent, instance)
		}
	}
	if len(request.CommitmentOffsets) > 0 {
		for offset := request.CommitmentOffsets[index]; offset < request.CommitmentOffsets[index+1]; offset++ {
			start, end := request.CommitmentTagOffsets[offset], request.CommitmentTagOffsets[offset+1]
			tags := map[string]bool{}
			for _, tag := range request.CommitmentContradictoryTags[start:end] {
				tags[tag] = true
			}
			agent.NarrativeCommitments = append(agent.NarrativeCommitments, engine.NarrativeCommitment{ID: request.CommitmentIds[offset], ExpiresAt: request.CommitmentExpiresAt[offset], ContradictoryTags: tags})
		}
	}
	if len(request.AggregatedEventOffsets) > 0 {
		for offset := request.AggregatedEventOffsets[index]; offset < request.AggregatedEventOffsets[index+1]; offset++ {
			agent.AggregatedEvents = append(agent.AggregatedEvents, engine.AggregatedEvent{Kind: request.AggregatedEventKinds[offset], AtHour: request.AggregatedEventHours[offset]})
		}
	}
	return nil
}

func tickRequestAt(request *gepv1.BatchTickRequest, index int) engine.TickRequest {
	result := engine.TickRequest{NowHours: request.NowHours, World: engine.World{Perception: worldAt(request.PerceptionOffsets, request.PerceptionIds, request.PerceptionValues, index), Relationships: worldAt(request.RelationshipOffsets, request.RelationshipIds, request.RelationshipValues, index), ContextValues: worldAt(request.ContextOffsets, request.ContextIds, request.ContextValues, index)}}
	if len(request.TargetSimulationLevels) > 0 {
		result.TargetLevel = fromWireLevel(request.TargetSimulationLevels[index])
	}
	if len(request.NewAggregatedEventKinds) > 0 && request.NewAggregatedEventKinds[index] != "" {
		result.AggregatedEvent = &engine.AggregatedEvent{Kind: request.NewAggregatedEventKinds[index]}
	}
	return result
}
func worldAt(offsets []uint32, ids []string, values []float64, index int) map[string]float64 {
	if len(offsets) == 0 {
		return nil
	}
	result := map[string]float64{}
	for offset := offsets[index]; offset < offsets[index+1]; offset++ {
		result[ids[offset]] = values[offset]
	}
	return result
}
func actionInstance(actions []engine.AdvertisedAction, providers []engine.AffordanceProvider, indices []uint32, index int) engine.ActionInstance {
	return engine.ActionInstance{Action: actions[index], ProviderID: providers[indices[index]].ID}
}
func floatAt(values []float64, index int) float64 {
	if len(values) == 0 {
		return 0
	}
	return values[index]
}
func boolAt(values []bool, index int) bool { return len(values) > 0 && values[index] }
func fromWireLevel(level gepv1.SimulationLevel) engine.SimulationLevel {
	if level == gepv1.SimulationLevel_SIMULATION_LEVEL_SIMPLIFIED {
		return engine.Simplified
	}
	if level == gepv1.SimulationLevel_SIMULATION_LEVEL_UNSPECIFIED {
		return ""
	}
	return engine.Full
}
func toWireLevel(level engine.SimulationLevel) gepv1.SimulationLevel {
	if level == engine.Simplified {
		return gepv1.SimulationLevel_SIMULATION_LEVEL_SIMPLIFIED
	}
	return gepv1.SimulationLevel_SIMULATION_LEVEL_FULL
}

func appendTickDelta(response *gepv1.BatchTickResponse, request *gepv1.BatchTickRequest, profile engine.Profile, actions []engine.AdvertisedAction, before, after engine.Agent, result engine.TickResult, index int) {
	changes := 0
	for cIndex, c := range profile.Considerations {
		if after.Considerations[c.ID].Value != request.Batch.ConsiderationValues[index*len(profile.Considerations)+cIndex] {
			if changes == 0 {
				response.ConsiderationChangeAgentIndices = append(response.ConsiderationChangeAgentIndices, uint32(index))
			}
			response.ChangedConsiderationIndices = append(response.ChangedConsiderationIndices, uint32(cIndex))
			response.ChangedConsiderationValues = append(response.ChangedConsiderationValues, after.Considerations[c.ID].Value)
			changes++
		}
	}
	if changes > 0 {
		response.ConsiderationChangeOffsets = append(response.ConsiderationChangeOffsets, uint32(len(response.ChangedConsiderationIndices)))
	}
	beforeIndex := request.CurrentActionIndices[index]
	afterIndex := actionIndex(after.CurrentAction, actions, request.Batch.ActionProviderIndices, request.Batch.ProviderIds)
	if beforeIndex != int32(afterIndex) || (after.CurrentAction != nil && (after.CurrentAction.Elapsed != floatAt(request.CurrentActionElapsed, index) || after.CurrentAction.ContinuationUtility != floatAt(request.CurrentActionUtilities, index) || after.CurrentAction.PlayerQueued != boolAt(request.CurrentActionPlayerQueued, index))) {
		response.ActionChangeAgentIndices = append(response.ActionChangeAgentIndices, uint32(index))
		response.CurrentActionIndices = append(response.CurrentActionIndices, int32(afterIndex))
		if after.CurrentAction != nil {
			response.CurrentActionElapsed = append(response.CurrentActionElapsed, after.CurrentAction.Elapsed)
			response.CurrentActionUtilities = append(response.CurrentActionUtilities, after.CurrentAction.ContinuationUtility)
			response.CurrentActionPlayerQueued = append(response.CurrentActionPlayerQueued, after.CurrentAction.PlayerQueued)
		} else {
			response.CurrentActionElapsed = append(response.CurrentActionElapsed, 0)
			response.CurrentActionUtilities = append(response.CurrentActionUtilities, 0)
			response.CurrentActionPlayerQueued = append(response.CurrentActionPlayerQueued, false)
		}
	}
	if len(before.ActionQueue) != len(after.ActionQueue) {
		response.QueueChangeAgentIndices = append(response.QueueChangeAgentIndices, uint32(index))
		for _, queued := range after.ActionQueue {
			response.QueueActionIndices = append(response.QueueActionIndices, int32(actionIndex(&queued, actions, request.Batch.ActionProviderIndices, request.Batch.ProviderIds)))
			response.QueueActionElapsed = append(response.QueueActionElapsed, queued.Elapsed)
			response.QueueActionUtilities = append(response.QueueActionUtilities, queued.ContinuationUtility)
		}
		response.QueueChangeOffsets = append(response.QueueChangeOffsets, uint32(len(response.QueueActionIndices)))
	}
	if len(before.NarrativeCommitments) != len(after.NarrativeCommitments) {
		response.CommitmentChangeAgentIndices = append(response.CommitmentChangeAgentIndices, uint32(index))
		for _, commitment := range after.NarrativeCommitments {
			response.CommitmentIds = append(response.CommitmentIds, commitment.ID)
			response.CommitmentExpiresAt = append(response.CommitmentExpiresAt, commitment.ExpiresAt)
			tags := make([]string, 0, len(commitment.ContradictoryTags))
			for tag := range commitment.ContradictoryTags {
				tags = append(tags, tag)
			}
			sort.Strings(tags)
			response.CommitmentContradictoryTags = append(response.CommitmentContradictoryTags, tags...)
			response.CommitmentTagOffsets = append(response.CommitmentTagOffsets, uint32(len(response.CommitmentContradictoryTags)))
		}
		response.CommitmentChangeOffsets = append(response.CommitmentChangeOffsets, uint32(len(response.CommitmentIds)))
	}
	if before.SimulationLevel != result.SimulationLevel {
		response.SimulationLevelChangeAgentIndices = append(response.SimulationLevelChangeAgentIndices, uint32(index))
		response.SimulationLevels = append(response.SimulationLevels, toWireLevel(result.SimulationLevel))
	}
	if before.SimulationLevel == engine.Simplified && result.SimulationLevel == engine.Full {
		response.AggregatedEventClearAgentIndices = append(response.AggregatedEventClearAgentIndices, uint32(index))
	}
}
func actionIndex(instance *engine.ActionInstance, actions []engine.AdvertisedAction, actionProviders []uint32, providerIDs []string) int {
	if instance == nil {
		return -1
	}
	for index, action := range actions {
		if action.ActionID == instance.Action.ActionID && providerIDs[actionProviders[index]] == instance.ProviderID {
			return index
		}
	}
	return -1
}
