package engine

import "math/rand/v2"

// CheckPreemption replaces the current action when a critical consideration
// makes the best available action valuable enough to clear the configured margin.
func CheckPreemption(agent *Agent, providers []AffordanceProvider, tuning Tuning, rng *rand.Rand) bool {
	if agent.CurrentAction == nil {
		return false
	}

	hasCriticalConsideration, hasImminentCollapse := urgencyState(agent.Considerations)
	if !hasCriticalConsideration || (agent.CurrentAction.PlayerQueued && !hasImminentCollapse) {
		return false
	}

	bestAction := SelectAction(*agent, providers, tuning, rng)
	if bestAction == nil {
		return false
	}

	preemptionMargin := tuning.PreemptionMargin
	if preemptionMargin == 0 {
		preemptionMargin = PREEMPTION_MARGIN
	}
	if bestAction.ContinuationUtility > preemptionMargin*agent.CurrentAction.ContinuationUtility {
		agent.CurrentAction = bestAction
		return true
	}
	return false
}

func urgencyState(considerations map[string]Consideration) (hasCriticalConsideration, hasImminentCollapse bool) {
	for _, consideration := range considerations {
		if consideration.Value < consideration.CriticalThreshold {
			hasCriticalConsideration = true
		}
		if consideration.Value <= collapseThreshold(consideration) {
			hasImminentCollapse = true
		}
	}
	return hasCriticalConsideration, hasImminentCollapse
}

// collapseThreshold reads the imminent-collapse floor from the consideration
// itself; a zero CriticalThreshold means "not configured" and falls back to
// the engine default.
func collapseThreshold(consideration Consideration) float64 {
	if consideration.CriticalThreshold != 0 {
		return consideration.CriticalThreshold
	}
	return IMMINENT_COLLAPSE_THRESHOLD
}

// Tick advances one full-detail simulation tick for agent.
func Tick(agent *Agent, providers []AffordanceProvider, world World, deltaHours float64, tuning Tuning, rng *rand.Rand) {
	UpdateConsiderations(agent, world, deltaHours)
	advanceCurrentAction(agent, deltaHours)

	if agent.CurrentAction == nil && len(agent.ActionQueue) == 0 {
		agent.CurrentAction = SelectAction(*agent, providers, tuning, rng)
		return
	}

	CheckPreemption(agent, providers, tuning, rng)
}

func advanceCurrentAction(agent *Agent, deltaHours float64) {
	if agent.CurrentAction == nil {
		return
	}

	ApplyGradualDeltas(agent, agent.CurrentAction, deltaHours)
	if agent.CurrentAction.Elapsed >= agent.CurrentAction.Action.EstimatedDuration {
		agent.CurrentAction = nil
	}
}

// ApplyAggregatedEvent records an event produced while an agent uses simplified LOD.
func ApplyAggregatedEvent(agent *Agent, event AggregatedEvent) {
	agent.AggregatedEvents = append(agent.AggregatedEvents, event)
}

// TransitionToFull reconstructs a plausible detailed state from accumulated
// events. The effects table, declared by the profile, maps each aggregated
// event kind to the consideration deltas it applies — the engine knows
// neither event nor consideration names. Events after hour and undeclared
// kinds are ignored; a nil table reconstructs nothing.
func TransitionToFull(agent *Agent, hour float64, effects []AggregatedEventEffect) {
	deltasByKind := make(map[string]map[string]float64, len(effects))
	for _, effect := range effects {
		deltasByKind[effect.Kind] = effect.Deltas
	}
	for _, event := range agent.AggregatedEvents {
		if event.AtHour > hour {
			continue
		}
		if deltas, declared := deltasByKind[event.Kind]; declared {
			applyAggregatedDeltas(agent, deltas)
		}
	}
	agent.AggregatedEvents = nil
	agent.SimulationLevel = Full
}

func applyAggregatedDeltas(agent *Agent, deltas map[string]float64) {
	for considerationID, delta := range deltas {
		consideration, exists := agent.Considerations[considerationID]
		if !exists {
			continue
		}
		consideration.Value = clamp(consideration.Value+delta, consideration.Min, consideration.Max)
		agent.Considerations[considerationID] = consideration
	}
}
