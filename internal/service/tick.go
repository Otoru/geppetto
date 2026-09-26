package service

import (
	"context"
	"errors"
	"fmt"
	"sort"

	engine "github.com/Otoru/geppetto"
	gepv1 "github.com/Otoru/geppetto/internal/gen/go/geppetto/v1"
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
	simulator := engine.NewSimulator(engine.WithProfile(profile))
	for index := range agents {
		if ctx.Err() != nil {
			return nil, status.FromContextError(ctx.Err()).Err()
		}
		before := agents[index]
		simulator.Advance(&agents[index], providers, tickRequestAt(request, index), request.Batch.Seed+uint64(index))
		appendTickDelta(response, request, profile, actions, before, agents[index], index)
	}
	return response, nil
}

func validateTickRequest(request *gepv1.BatchTickRequest, agents, actions int) error {
	if err := validateTickCurrentActions(request, agents, actions); err != nil {
		return err
	}
	if err := validateTickQueuedActions(request, agents, actions); err != nil {
		return err
	}
	if err := validateTickCommitments(request, agents); err != nil {
		return err
	}
	if err := validateTickAggregatedEvents(request, agents); err != nil {
		return err
	}
	return validateTickWorlds(request, agents)
}

func validateTickCurrentActions(request *gepv1.BatchTickRequest, agents, actions int) error {
	if len(request.CurrentActionIndices) != agents {
		return fmt.Errorf("current_action_indices must match batch.agent_ids")
	}
	for _, check := range []struct {
		count   int
		message string
	}{
		{len(request.CurrentActionElapsed), "current action arrays must match batch.agent_ids"},
		{len(request.CurrentActionUtilities), "current action arrays must match batch.agent_ids"},
		{len(request.CurrentActionPlayerQueued), "current_action_player_queued must match batch.agent_ids"},
		{len(request.SimulationLevels), "simulation level arrays must match batch.agent_ids"},
		{len(request.TargetSimulationLevels), "simulation level arrays must match batch.agent_ids"},
	} {
		if mismatchedOptionalCount(check.count, agents) {
			return errors.New(check.message)
		}
	}
	return validateKnownCurrentActions(request.CurrentActionIndices, actions)
}

func validateKnownCurrentActions(indices []int32, actions int) error {
	for index, action := range indices {
		if action < -1 || int(action) >= actions {
			return fmt.Errorf("current action %d references an unknown action", index)
		}
	}
	return nil
}

func validateTickQueuedActions(request *gepv1.BatchTickRequest, agents, actions int) error {
	if err := validateTickOffsets(request.QueueActionOffsets, len(request.QueueActionIndices), agents, "queue_action_offsets"); err != nil {
		return err
	}
	for _, count := range []int{len(request.QueueActionElapsed), len(request.QueueActionUtilities)} {
		if mismatchedOptionalCount(count, len(request.QueueActionIndices)) {
			return fmt.Errorf("queue action arrays must match queue_action_indices")
		}
	}
	return validateKnownQueuedActions(request.QueueActionIndices, actions)
}

func validateKnownQueuedActions(indices []uint32, actions int) error {
	for _, action := range indices {
		if int(action) >= actions {
			return fmt.Errorf("queued action references an unknown action")
		}
	}
	return nil
}

func validateTickCommitments(request *gepv1.BatchTickRequest, agents int) error {
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
	return nil
}

func validateTickAggregatedEvents(request *gepv1.BatchTickRequest, agents int) error {
	if err := validateTickOffsets(request.AggregatedEventOffsets, len(request.AggregatedEventKinds), agents, "aggregated_event_offsets"); err != nil {
		return err
	}
	if len(request.AggregatedEventHours) != len(request.AggregatedEventKinds) {
		return fmt.Errorf("aggregated_event_hours must match aggregated_event_kinds")
	}
	if mismatchedOptionalCount(len(request.NewAggregatedEventKinds), agents) {
		return fmt.Errorf("new_aggregated_event_kinds must match batch.agent_ids")
	}
	return nil
}

func validateTickWorlds(request *gepv1.BatchTickRequest, agents int) error {
	for _, world := range []struct {
		offsets []uint32
		ids     []string
		values  []float64
		name    string
	}{
		{request.PerceptionOffsets, request.PerceptionIds, request.PerceptionValues, "perception"},
		{request.RelationshipOffsets, request.RelationshipIds, request.RelationshipValues, "relationship"},
		{request.ContextOffsets, request.ContextIds, request.ContextValues, "context"},
	} {
		if err := validateTickWorld(world.offsets, world.ids, world.values, world.name, agents); err != nil {
			return err
		}
	}
	return nil
}

func validateTickWorld(offsets []uint32, ids []string, values []float64, name string, agents int) error {
	if err := validateTickOffsets(offsets, len(ids), agents, name+"_offsets"); err != nil {
		return err
	}
	if len(values) != len(ids) {
		return fmt.Errorf("%s values must match ids", name)
	}
	return nil
}

func mismatchedOptionalCount(count, expected int) bool {
	return count != 0 && count != expected
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

func appendTickDelta(response *gepv1.BatchTickResponse, request *gepv1.BatchTickRequest, profile engine.Profile, actions []engine.AdvertisedAction, before, after engine.Agent, index int) {
	appendConsiderationChanges(response, request, profile, after, index)
	appendActionChange(response, request, actions, after, index)
	appendQueueChange(response, request, actions, before, after, index)
	appendCommitmentChange(response, before, after, index)
	appendLevelChange(response, before, after, index)
}

func appendConsiderationChanges(response *gepv1.BatchTickResponse, request *gepv1.BatchTickRequest, profile engine.Profile, after engine.Agent, index int) {
	changes := 0
	for cIndex, consideration := range profile.Considerations {
		value := after.Considerations[consideration.ID].Value
		if value == request.Batch.ConsiderationValues[index*len(profile.Considerations)+cIndex] {
			continue
		}
		if changes == 0 {
			response.ConsiderationChangeAgentIndices = append(response.ConsiderationChangeAgentIndices, uint32(index))
		}
		response.ChangedConsiderationIndices = append(response.ChangedConsiderationIndices, uint32(cIndex))
		response.ChangedConsiderationValues = append(response.ChangedConsiderationValues, value)
		changes++
	}
	if changes > 0 {
		response.ConsiderationChangeOffsets = append(response.ConsiderationChangeOffsets, uint32(len(response.ChangedConsiderationIndices)))
	}
}

func appendActionChange(response *gepv1.BatchTickResponse, request *gepv1.BatchTickRequest, actions []engine.AdvertisedAction, after engine.Agent, index int) {
	beforeIndex := request.CurrentActionIndices[index]
	afterIndex := int32(actionIndex(after.CurrentAction, actions, request.Batch.ActionProviderIndices, request.Batch.ProviderIds))
	if beforeIndex == afterIndex && !actionProgressChanged(after.CurrentAction, request, index) {
		return
	}
	response.ActionChangeAgentIndices = append(response.ActionChangeAgentIndices, uint32(index))
	response.CurrentActionIndices = append(response.CurrentActionIndices, afterIndex)
	elapsed, utility, playerQueued := 0.0, 0.0, false
	if after.CurrentAction != nil {
		elapsed = after.CurrentAction.Elapsed
		utility = after.CurrentAction.ContinuationUtility
		playerQueued = after.CurrentAction.PlayerQueued
	}
	response.CurrentActionElapsed = append(response.CurrentActionElapsed, elapsed)
	response.CurrentActionUtilities = append(response.CurrentActionUtilities, utility)
	response.CurrentActionPlayerQueued = append(response.CurrentActionPlayerQueued, playerQueued)
}

func actionProgressChanged(action *engine.ActionInstance, request *gepv1.BatchTickRequest, index int) bool {
	if action == nil {
		return false
	}
	return action.Elapsed != floatAt(request.CurrentActionElapsed, index) ||
		action.ContinuationUtility != floatAt(request.CurrentActionUtilities, index) ||
		action.PlayerQueued != boolAt(request.CurrentActionPlayerQueued, index)
}

func appendQueueChange(response *gepv1.BatchTickResponse, request *gepv1.BatchTickRequest, actions []engine.AdvertisedAction, before, after engine.Agent, index int) {
	if len(before.ActionQueue) == len(after.ActionQueue) {
		return
	}
	response.QueueChangeAgentIndices = append(response.QueueChangeAgentIndices, uint32(index))
	for _, queued := range after.ActionQueue {
		response.QueueActionIndices = append(response.QueueActionIndices, int32(actionIndex(&queued, actions, request.Batch.ActionProviderIndices, request.Batch.ProviderIds)))
		response.QueueActionElapsed = append(response.QueueActionElapsed, queued.Elapsed)
		response.QueueActionUtilities = append(response.QueueActionUtilities, queued.ContinuationUtility)
	}
	response.QueueChangeOffsets = append(response.QueueChangeOffsets, uint32(len(response.QueueActionIndices)))
}

func appendCommitmentChange(response *gepv1.BatchTickResponse, before, after engine.Agent, index int) {
	if len(before.NarrativeCommitments) == len(after.NarrativeCommitments) {
		return
	}
	response.CommitmentChangeAgentIndices = append(response.CommitmentChangeAgentIndices, uint32(index))
	for _, commitment := range after.NarrativeCommitments {
		response.CommitmentIds = append(response.CommitmentIds, commitment.ID)
		response.CommitmentExpiresAt = append(response.CommitmentExpiresAt, commitment.ExpiresAt)
		tags := sortedTags(commitment.ContradictoryTags)
		response.CommitmentContradictoryTags = append(response.CommitmentContradictoryTags, tags...)
		response.CommitmentTagOffsets = append(response.CommitmentTagOffsets, uint32(len(response.CommitmentContradictoryTags)))
	}
	response.CommitmentChangeOffsets = append(response.CommitmentChangeOffsets, uint32(len(response.CommitmentIds)))
}

func sortedTags(tags map[string]bool) []string {
	result := make([]string, 0, len(tags))
	for tag := range tags {
		result = append(result, tag)
	}
	sort.Strings(result)
	return result
}

func appendLevelChange(response *gepv1.BatchTickResponse, before, after engine.Agent, index int) {
	if before.SimulationLevel != after.SimulationLevel {
		response.SimulationLevelChangeAgentIndices = append(response.SimulationLevelChangeAgentIndices, uint32(index))
		response.SimulationLevels = append(response.SimulationLevels, toWireLevel(after.SimulationLevel))
	}
	if before.SimulationLevel == engine.Simplified && after.SimulationLevel == engine.Full {
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
